package summary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/dev2/api/grpc/v2/summary"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/schollz/progressbar/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var globalConn *grpc.ClientConn

func disconnect() {
	if globalConn != nil {
		err := globalConn.Close()
		if err != nil {
			fmt.Println("Failed to close connection: ", err)
		}
	}
}

func getGRPCConnection(address string) (*grpc.ClientConn, error) {
	if globalConn == nil {
		var err error
		opts := []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(32 * 10e6)),
		}

		globalConn, err = grpc.Dial(address, opts...)
		if err != nil {
			return nil, err
		}
	}

	return globalConn, nil
}

// GetSummary on pods
func GetSummary(c *k8s.Client, o Options) (*Workload, error) {
	gRPC, err := common.ConnectGrpc(c, o.GRPC)
	if err != nil {
		return nil, err
	}

	data := &summary.SummaryRequest{
		Labels:      o.Labels,
		Namespaces:  o.Namespace,
		Operation:   o.Operation,
		Source:      o.Source,
		Destination: o.Destination,
	}

	conn, err := getGRPCConnection(gRPC)
	if err != nil {
		return nil, errors.New("could not connect to the server. Possible troubleshooting:\n- Check if discovery engine is running\n- kubectl get po -n accuknox-agents")
	}

	client := summary.NewSummaryClient(conn)

	sumResp, err := o.getSummaryPerWorkload(client, data)
	if err != nil {
		return nil, err
	}

	return sumResp, nil
}

// Summary summarizes the data recieved from discovery engine
func Summary(c *k8s.Client, o Options) error {
	defer disconnect()
	fmt.Println("Summarizing data...")
	workload, err := GetSummary(c, o)
	if err != nil {
		return err
	}

	if workload != nil && o.Glance {
		glance(workload)
	}

	if !o.noFilters() {
		workload = filterOpts(workload, o)
	}

	if workload != nil {
		// PrintDebugLog()
		switch {
		case o.View == "table":
			displayWorkloadInTable(workload)

		case o.View == "json":
			jsonData, err := json.MarshalIndent(workload, "", "    ")
			if err != nil {
				return err
			}
			fmt.Println(string(jsonData))

		case o.Dump:
			// err := writeTableToFile(workload)
			// if err != nil {
			// 	fmt.Println("Failed to write table to file: ", err)
			// 	return err
			// }

			jsonData, err := json.MarshalIndent(workload, "", "    ")
			if err != nil {
				return err
			}

			dirPath := "knoxctl_out/summary/json"
			if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
				return err
			}

			filePath := filepath.Join(dirPath, "summary.json")
			if err := os.WriteFile(filePath, jsonData, 0600); err != nil {
				return err
			}
			fmt.Println("JSON summary written to ", filePath)

		default:
			if len(workload.Clusters) != 0 {
				StartTUI(workload)
			} else {
				fmt.Println("Summary data not found.")
			}
		}
	} else {
		fmt.Println("No workloads found.")
	}

	// PrintDebugLog()
	return nil
}

func (o *Options) getSummaryPerWorkload(client summary.SummaryClient, sumReq *summary.SummaryRequest) (*Workload, error) {
	workloadReq := &summary.WorkloadRequest{}

	workloads, err := client.GetWorkloads(context.Background(), workloadReq)
	if err != nil {
		return nil, err
	}

	rootWorkload := &Workload{
		Clusters: make(map[string]*Cluster),
	}

	errChan := make(chan error)
	workloadChan := make(chan *summary.Workload)
	sumRespChan := make(chan *summary.SummaryResponse)
	done := make(chan struct{})
	var wg sync.WaitGroup

	bar := initializeProgressBar("Processing Workloads...", len(workloads.Workloads))

	const numWorkers = 30
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for w := range workloadChan {
				reqCopy := &summary.SummaryRequest{
					Clusters:   []string{w.Cluster},
					Namespaces: []string{w.Namespace},
				}

				ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
				defer cancel()

				sumResp, err := client.GetSummaryEvent(ctx, reqCopy)
				if err != nil {
					errChan <- err
					continue
				}

				sumRespChan <- sumResp
			}
		}()
	}

	// Send workloads to the channel
	go func() {
		for _, w := range workloads.Workloads {
			workloadChan <- w
		}
		close(workloadChan)
	}()

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(sumRespChan)
		close(errChan)
		_ = bar.Finish()
	}()

	// Process the results
	go func() {
		for {
			select {
			case sumResp, ok := <-sumRespChan:
				if !ok {
					sumRespChan = nil
				} else {
					processSummaryResponse(rootWorkload, sumResp, bar)
				}

			case _, ok := <-errChan:
				if !ok {
					errChan = nil
				}
			}

			if sumRespChan == nil && errChan == nil {
				close(done)
				break
			}
		}
	}()

	<-done

	return rootWorkload, nil
}

// processSummaryResponse populates the Workload structure with the summary response data.
func processSummaryResponse(rootWorkload *Workload, sumResp *summary.SummaryResponse, bar *progressbar.ProgressBar) {
	_ = bar.Add(1)
	for clusterName, cluster := range sumResp.GetClusters() {
		rootCluster := rootWorkload.AddCluster(clusterName, &Cluster{
			Namespaces:  make(map[string]*Namespace),
			ClusterName: clusterName,
		})
		_ = rootCluster.SetHash()

		for nsName, ns := range cluster.GetNamespaces() {
			rootNamespace := rootCluster.AddNamespace(nsName, &Namespace{
				NamespaceName: nsName,
				Deployments:   make(map[string]*WorkloadEvents),
				ReplicaSets:   make(map[string]*WorkloadEvents),
				StatefulSets:  make(map[string]*WorkloadEvents),
				DaemonSets:    make(map[string]*WorkloadEvents),
				Jobs:          make(map[string]*WorkloadEvents),
				CronJobs:      make(map[string]*WorkloadEvents),
			})

			populateNamespace(rootNamespace, ns)
			_ = rootNamespace.SetHash()
		}
	}

	_ = rootWorkload.SetHash()
}

// populateNamespace takes a namespace from the gRPC response and populates the namespace structure.
func populateNamespace(existingNamespace *Namespace, namespace *summary.Namespace) {
	for depName, depEvents := range namespace.Deployments {
		if depEvents != nil {
			we := createWorkloadEventsFromSummary(depName, depEvents)
			_ = we.SetHash()
			existingNamespace.AddDeploymentEvents(depName, we)
		}
	}

	for rsName, rsEvents := range namespace.ReplicaSets {
		if rsEvents != nil {
			we := createWorkloadEventsFromSummary(rsName, rsEvents)
			_ = we.SetHash()
			existingNamespace.AddReplicaSetEvents(rsName, we)
		}
	}

	for ssName, ssEvents := range namespace.StatefulSets {
		if ssEvents != nil {
			we := createWorkloadEventsFromSummary(ssName, ssEvents)
			_ = we.SetHash()
			existingNamespace.AddStatefulSetEvents(ssName, we)
		}
	}

	for dsName, dsEvents := range namespace.DaemonSets {
		if dsEvents != nil {
			we := createWorkloadEventsFromSummary(dsName, dsEvents)
			_ = we.SetHash()
			existingNamespace.AddDaemonSetEvents(dsName, we)
		}
	}

	for jobName, jobEvents := range namespace.Jobs {
		if jobEvents != nil {
			we := createWorkloadEventsFromSummary(jobName, jobEvents)
			_ = we.SetHash()
			existingNamespace.AddJobEvents(jobName, we)
		}
	}
}

func createWorkloadEventsFromSummary(weName string, summaryEvents *summary.WorkloadEvents) *WorkloadEvents {
	we := &WorkloadEvents{
		WorkloadName: weName,
		Events: &Events{
			File:    summaryEvents.Events.File,
			Process: summaryEvents.Events.Process,
			Ingress: summaryEvents.Events.Ingress,
			Egress:  summaryEvents.Events.Egress,
			Bind:    summaryEvents.Events.Bind,
		},
	}

	_ = we.SetHash()
	return we
}

func initializeProgressBar(description string, max int) *progressbar.ProgressBar {
	return progressbar.NewOptions(
		max,
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionSpinnerType(9),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionSetElapsedTime(true),
		progressbar.OptionShowCount(),
		progressbar.OptionShowBytes(false),
		progressbar.OptionShowIts(),
	)
}
