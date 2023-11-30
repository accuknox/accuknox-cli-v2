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
			err := writeTableToFile(workload)
			if err != nil {
				fmt.Println("Failed to write table to file: ", err)
				return err
			}

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
	sumRespChan := make(chan *summary.SummaryResponse)
	done := make(chan struct{})
	var wg sync.WaitGroup

	bar := initializeProgressBar("Processing Workloads...", len(workloads.Workloads))

	for _, w := range workloads.Workloads {
		wg.Add(1)
		go func(w *summary.Workload) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*30) // 30-second timeout
			defer cancel()

			sumReq.Namespaces = []string{w.Namespace}
			sumReq.Clusters = []string{w.Cluster}
			sumResp, err := client.GetSummaryEvent(ctx, sumReq)
			if err != nil {
				errChan <- err
				return
			}

			sumRespChan <- sumResp

			_ = bar.Add(1)
		}(w)
	}

	go func() {
		wg.Wait()
		close(sumRespChan)
		close(errChan)
		_ = bar.Finish()
	}()

	go func() {
		for {
			select {
			case sumResp, ok := <-sumRespChan:
				if !ok {
					sumRespChan = nil
				} else {
					processSummaryResponse(rootWorkload, sumResp, workloads.Workloads)
				}

			case _, ok := <-errChan:
				if !ok {
					errChan = nil
				}
				_ = bar.Finish()
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
func processSummaryResponse(rootWorkload *Workload, sumResp *summary.SummaryResponse, workloads []*summary.Workload) {
	for clusterName, cluster := range sumResp.GetClusters() {
		rootCluster := rootWorkload.AddCluster(clusterName, &Cluster{
			Cluster:    cluster,
			Namespaces: make(map[string]*Namespace),
		})

		for nsName, ns := range cluster.GetNamespaces() {
			rootNamespace := rootCluster.AddNamespace(nsName, &Namespace{
				Namespace:     ns,
				WorkloadTypes: make(map[string]*WorkloadType),
			})

			populateNamespace(rootNamespace, ns)
		}
	}
}

// populateNamespace takes a namespace from the gRPC response and populates the namespace structure.
func populateNamespace(existingNamespace *Namespace, namespace *summary.Namespace) {
	for workloadType, eventsMap := range map[string]map[string]*summary.WorkloadEvents{
		"Deployments":  namespace.Deployments,
		"StatefulSets": namespace.StatefulSets,
		"DaemonSets":   namespace.DaemonSets,
		"ReplicaSets":  namespace.ReplicaSets,
		"Jobs":         namespace.Jobs,
		"CronJobs":     namespace.CronJobs,
	} {
		workloadTypeStruct := existingNamespace.GetWorkloadType(workloadType)
		if workloadTypeStruct == nil {
			workloadTypeStruct = &WorkloadType{Events: &Events{}, Labels: &Labels{}}
			existingNamespace.AddWorkloadType(workloadType, workloadTypeStruct)
		}

		convertWorkloadEvents(workloadTypeStruct, eventsMap)
	}
}

// convertWorkloadEvents converts summary WorkloadEvents into the application's Events structure.
func convertWorkloadEvents(wt *WorkloadType, eventsMap map[string]*summary.WorkloadEvents) {
	for _, workloadEvents := range eventsMap {
		if workloadEvents == nil {
			continue
		}

		wt.Labels.mu.Lock()
		wt.Labels.Labels = workloadEvents.Labels
		wt.Labels.mu.Unlock()

		for _, pfe := range workloadEvents.Events.Process {
			wt.Events.AddProcessEvent(pfe)
		}

		for _, pe := range workloadEvents.Events.File {
			wt.Events.AddFileEvent(pe)
		}

		for _, ing := range workloadEvents.Events.Ingress {
			wt.Events.AddIngressEvent(ing)
		}

		for _, eg := range workloadEvents.Events.Egress {
			wt.Events.AddEgressEvent(eg)
		}

		for _, bi := range workloadEvents.Events.Bind {
			wt.Events.AddBindEvent(bi)
		}
	}
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
		progressbar.OptionShowBytes(true),
		progressbar.OptionShowIts(),
	)
}
