package scan

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	kaproto "github.com/kubearmor/KubeArmor/protobuf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Scan structure dependencies
type Scan struct {
	// Options for scan subcommand
	options *ScanOptions

	// Connection for gRPC service
	conn *grpc.ClientConn

	// Stream filter (all, system, policy)
	streamFilter string

	// Service client
	serviceClient kaproto.LogServiceClient

	// Alerts stream from KA
	alertsStream kaproto.LogService_WatchAlertsClient

	// Alerts chan
	alertsChan chan []byte

	// Logs stream from KA
	logStream kaproto.LogService_WatchLogsClient

	// Logs chan
	logsChan chan []byte

	// Errors chan
	errChan chan error

	// Process Tree data structure
	processForest *ProcessForest

	// Network event cache
	networkCache *NetworkCache

	// Segregator
	segregate *Segregate

	// WaitGroup to wait for all goroutines to finish
	wg sync.WaitGroup

	// Done chan
	done chan struct{}
}

// Enforce Client interface on Scan structure
var _ Client = (*Scan)(nil)

// New instantiates the Scan subcommand to scan CI/CD pipeline events
func New(opts *ScanOptions) *Scan {
	return &Scan{
		options:       opts,
		errChan:       make(chan error),
		alertsChan:    make(chan []byte),
		logsChan:      make(chan []byte),
		done:          make(chan struct{}),
		processForest: NewProcessForest(),
		networkCache:  NewNetworkCache(),
		segregate:     NewSegregator(),
	}
}

// Start implements Client interface
func (s *Scan) Start() error {
	fmt.Println("Starting to scan...")
	isRunning := isKubeArmorActive()
	if !isRunning {
		fmt.Println("KubeArmor service is not running")
		return nil
	}

	err := s.ConnectToGRPC()
	if err != nil {
		return fmt.Errorf("failed to connect to kubearmor's gRPC service: %s", err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGABRT, syscall.SIGKILL)

	go func() {
		<-signalChan
		fmt.Println("Received OS interrupt, shutting down gracefully...")
		cancel()
	}()

	if !s.healthCheck() {
		fmt.Println("Service health check failed")
		return nil
	}

	// Start collecting data
	err = s.CollectData(ctx)
	if err != nil {
		return fmt.Errorf("failed to collect data: %s", err.Error())
	}

	// Wait
	<-ctx.Done()

	close(s.done)

	// Close the gRPC connection
	if s.conn != nil {
		s.conn.Close()
		fmt.Println("Released gRPC service")
	}

	err = s.segregate.SaveSegregatedDataToFile("segregated_data.json")
	if err != nil {
		fmt.Printf("Error saving segregated data to file: %v\n", err)
	} else {
		fmt.Println("Segregated data saved successfully.")
	}

	s.processForest.BuildFromSegregatedData(s.segregate.data.Logs.Process)
	s.processForest.SaveProcessForestJSON("process_tree.json")

	s.networkCache.StartCachingEvents(s.segregate.data.Logs.Network)
	s.networkCache.SaveNetworkCacheJSON("network_events.json")
	return nil
}

// ConnectToGRPC implements Client interface
func (s *Scan) ConnectToGRPC() error {
	if s.options.GRPC == "" {
		s.options.GRPC = common.KubeArmorGRPCAddress
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(32 * 10e6)),
	}

	connection, err := grpc.Dial(s.options.GRPC, opts...)
	if err != nil {
		return err
	}

	fmt.Printf("Connected to gRPC server at: %s\n", s.options.GRPC)
	s.conn = connection
	s.serviceClient = kaproto.NewLogServiceClient(s.conn)
	return nil
}

// healthCheck performs health check on KA's gRPC service
func (s *Scan) healthCheck() bool {
	ctx, cancel := context.WithTimeout(context.Background(), common.OneMinute)
	defer cancel()

	bigNum, _ := rand.Int(rand.Reader, big.NewInt(100))
	check := int32(bigNum.Int64())

	nonce := kaproto.NonceMessage{Nonce: check}

	result, err := s.serviceClient.HealthCheck(ctx, &nonce)
	if err != nil {
		return false
	}

	if check != result.Retval {
		return false
	}

	fmt.Println("KubeArmor gRPC service is healthy")
	return true
}

// CollectData implements Client interface
func (s *Scan) CollectData(ctx context.Context) error {
	var err error

	if s.options.FilterEventType.All || (s.options.FilterEventType.All && s.options.FilterEventType.System) {
		s.streamFilter = "all"
	}

	if s.options.FilterEventType.System {
		s.streamFilter = "system"
	}

	// Default filtering in case no options are given for filtering
	if !s.options.FilterEventType.System && !s.options.FilterEventType.All {
		s.streamFilter = "policy"
	}

	s.alertsStream, err = s.serviceClient.WatchAlerts(ctx, &kaproto.RequestMessage{Filter: s.streamFilter})
	if err != nil {
		return err
	}

	s.logStream, err = s.serviceClient.WatchLogs(ctx, &kaproto.RequestMessage{Filter: s.streamFilter})
	if err != nil {
		return err
	}

	s.wg.Add(3)
	go s.collectLogs(ctx)
	go s.collectAlerts(ctx)
	go s.readErrors(ctx)

	go s.processData(ctx)

	return nil
}

// readErrors listens for errors and handles context cancellation
func (s *Scan) readErrors(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Context cancelled, wrapping up!")
			return
		case err := <-s.errChan:
			if err != nil {
				fmt.Printf("Error received while reading from service stream: %s", err.Error())
			}
		}
	}
}

// collectLogs collects logs from KubeArmor systemd service
func (s *Scan) collectLogs(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			res, err := s.logStream.Recv()
			if err != nil {
				if err == io.EOF {
					continue
				}

				s.errChan <- err
				return
			}

			data, err := json.Marshal(res)
			if err != nil {
				fmt.Printf("Failed to marshal log: %v\n", err)
				continue
			}

			select {
			case s.logsChan <- data:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (s *Scan) collectAlerts(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			res, err := s.alertsStream.Recv()
			if err != nil {
				if err == io.EOF {
					continue
				}
				s.errChan <- err
				return
			}

			data, err := json.Marshal(res)
			if err != nil {
				select {
				case s.errChan <- fmt.Errorf("failed to marshal alert: %v", err):
				default:
				}
				return
			}

			select {
			case s.alertsChan <- data:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (s *Scan) processData(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// Process remaining data before exiting
			for {
				select {
				case alertData := <-s.alertsChan:
					var alert kaproto.Alert
					if err := json.Unmarshal(alertData, &alert); err != nil {
						fmt.Printf("Failed to unmarshal alert: %v\n", err)
						continue
					}
					s.segregate.SegregateAlert(&alert)

				case logData := <-s.logsChan:
					var log kaproto.Log
					if err := json.Unmarshal(logData, &log); err != nil {
						fmt.Printf("Failed to unmarshal log: %v\n", err)
						continue
					}
					s.segregate.SegregateLogs(&log)

				case <-s.done:
					fmt.Println("All data processed, exiting safely")
					return
				}
			}
		case alertData := <-s.alertsChan:
			var alert kaproto.Alert
			if err := json.Unmarshal(alertData, &alert); err != nil {
				fmt.Printf("Failed to unmarshal alert: %v\n", err)
				continue
			}
			s.segregate.SegregateAlert(&alert)
		case logData := <-s.logsChan:
			var log kaproto.Log
			if err := json.Unmarshal(logData, &log); err != nil {
				fmt.Printf("Failed to unmarshal log: %v\n", err)
				continue
			}
			s.segregate.SegregateLogs(&log)
		}
	}
}

// func (s *Scan) addAlertToProcessTree(alert *kaproto.Alert) {
// 	command := fmt.Sprintf("%s %s", alert.ProcessName, alert.Resource)
// 	syscall := alert.Data // Assuming `Data` contains syscall info
// 	fmt.Printf("Adding Alert: PID=%d, PPID=%d, Command=%s, Syscall=%s, Timestamp=%d, UpdatedTime=%s\n", alert.PID, alert.PPID, command, syscall, alert.Timestamp, alert.UpdatedTime)
// 	s.processTree.AddProcess(alert.PID, alert.PPID, command, syscall, alert.Timestamp, alert.UpdatedTime)
// }
//
// func (s *Scan) addLogToProcessTree(log *kaproto.Log) {
// 	command := fmt.Sprintf("%s %s", log.ProcessName, log.Resource)
// 	syscall := log.Data // Assuming `Data` contains syscall info
// 	fmt.Printf("Adding Log: PID=%d, PPID=%d, Command=%s, Syscall=%s, Timestamp=%d, UpdatedTime=%s\n", log.PID, log.PPID, command, syscall, log.Timestamp, log.UpdatedTime)
// 	s.processTree.AddProcess(log.PID, log.PPID, command, syscall, log.Timestamp, log.UpdatedTime)
// }
