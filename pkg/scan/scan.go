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
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/scan/policy"
	kaproto "github.com/kubearmor/KubeArmor/protobuf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	// GitHub owner of policy templates repo
	OwnerKubeArmor = "kubearmor"

	// Policy templates
	PolicyTemplateRepo = "policy-templates"
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

	// Policy applier
	policyApplier *policy.Apply

	// Alerts processor
	alertProcessor *AlertProcessor
}

// Enforce Client interface on Scan structure
var _ Client = (*Scan)(nil)

// New instantiates the Scan subcommand to scan CI/CD pipeline events
func New(opts *ScanOptions) *Scan {
	s := &Scan{
		options:        opts,
		errChan:        make(chan error),
		alertsChan:     make(chan []byte),
		logsChan:       make(chan []byte),
		done:           make(chan struct{}),
		processForest:  NewProcessForest(),
		networkCache:   NewNetworkCache(),
		segregate:      NewSegregator(),
		alertProcessor: NewAlertProcessor(),
	}

	if opts.RepoBranch == "" {
		opts.RepoBranch = "main"
	}
	zipURL := fmt.Sprintf("https://github.com/%s/%s/archive/refs/heads/%s.zip", OwnerKubeArmor, PolicyTemplateRepo, opts.RepoBranch)

	hostname, _ := getHostname()
	s.policyApplier = policy.NewApplier(
		opts.GRPC,
		zipURL,
		hostname,
		opts.PolicyAction,
		opts.PolicyEvent,
		opts.PolicyDryRun,
	)

	return s
}

// Start implements Client interface
func (s *Scan) Start() error {
	fmt.Println("Starting to scan...")
	// isRunning := isKubeArmorActive()
	// if !isRunning {
	// 	fmt.Println("KubeArmor service is not running")
	// 	return nil
	// }

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
		_ = s.conn.Close()
		fmt.Println("Released gRPC service")
	}

	// post processing data
	s.postProcessing()
	return nil
}

func (s *Scan) HandlePolicies() error {
	fmt.Println("Handling policies...")

	if s.options.PolicyDryRun {
		fmt.Println(`Running in dry run mode, policies won't be 
        applied on the system, but will get generated and saved`)
	}

	err := s.policyApplier.Apply()
	if err != nil {
		return fmt.Errorf("failed to apply hardening policies: %s", err.Error())
	}

	if s.options.PolicyDryRun {
		s.postProcessing()
	}

	fmt.Println("Policies handled successfully")
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

func (s *Scan) postProcessing() {
	currentTime := time.Now().Format("2006-02-02_15-04-05")

	outputDir := "."
	if s.options.Output != "" {
		outputDir = s.options.Output
	}

	createFilePath := func(baseName, ext string) string {
		fileName := fmt.Sprintf("knoxctl_scan_%s_%s.%s", baseName, currentTime, ext)
		return filepath.Join(outputDir, fileName)
	}

	segregatedDataPath := createFilePath("segragated_data", "json")
	err := s.segregate.SaveSegregatedDataToFile(segregatedDataPath)
	if err != nil {
		fmt.Printf("error while saving segregated data: %s\n", err.Error())
	} else {
		fmt.Printf("Segrgated data saved successfully to %s\n", segregatedDataPath)
	}

	s.processForest.BuildFromSegregatedData(s.segregate.data.Logs.Process)
	processTreePath := createFilePath("process_tree", "json")
	err = s.processForest.SaveProcessForestJSON(processTreePath)
	if err != nil {
		fmt.Printf("failed to write process tree json file: %s\n", err.Error())
	} else {
		fmt.Printf("Process tree json written to %s\n", processTreePath)
	}

	s.networkCache.StartCachingEvents(s.segregate.data.Logs.Network)
	processTreeMDPath := createFilePath("process_tree", "md")
	err = s.processForest.SaveProcessForestMarkdown(processTreeMDPath)
	if err != nil {
		fmt.Printf("failed to write process tree markdown file: %s\n", err.Error())
	} else {
		fmt.Printf("Process tree markdown written to %s\n", processTreeMDPath)
	}

	networkFilePath := createFilePath("network_events", "json")
	err = s.networkCache.SaveNetworkCacheJSON(networkFilePath)
	if err != nil {
		fmt.Printf("failed to write network json file: %s\n", err.Error())
	} else {
		fmt.Printf("Network events json file written to %s\n", networkFilePath)
	}

	networkMDPath := createFilePath("network_events_md", "md")
	err = s.networkCache.SaveNetworkCacheMarkdown(networkMDPath)
	if err != nil {
		fmt.Printf("failed to write to network json file: %s\n", err.Error())
	} else {
		fmt.Printf("Network events markdown file written to %s\n", networkMDPath)
	}

	if s.options.PolicyDryRun {
		policiesPath := createFilePath("generated_policies", "yaml")
		err := s.policyApplier.SavePolicies(policiesPath)
		if err != nil {
			fmt.Printf("failed to save generated policies: %s\n", err.Error())
		} else {
			fmt.Printf("Generated policies saved to %s\n", policiesPath)
		}
	}

	// Start handling alerts
	s.alertProcessor.ProcessAlerts(s.segregate.data)

	// Generate and save JSON
	alertsJSON, err := s.alertProcessor.GenerateJSON()
	if err != nil {
		fmt.Printf("Error generating JSON for alerts: %v\n", err)
	} else {
		alertsJSONPath := createFilePath("processed_alerts", "json")
		err = common.CleanAndWrite(alertsJSONPath, alertsJSON)
		if err != nil {
			fmt.Printf("Error writing alerts JSON to file: %v\n", err)
		} else {
			fmt.Printf("Processed alerts JSON written to %s\n", alertsJSONPath)
		}
	}

	// Generate and save Markdown
	alertsMarkdown := s.alertProcessor.GenerateMarkdownTable()
	alertsMarkdownPath := createFilePath("processed_alerts", "md")
	err = common.CleanAndWrite(alertsMarkdownPath, []byte(alertsMarkdown))
	if err != nil {
		fmt.Printf("Error writing alerts Markdown to file: %v\n", err)
	} else {
		fmt.Printf("Processed alerts Markdown written to %s\n", alertsMarkdownPath)
	}
}
