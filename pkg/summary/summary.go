package summary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/dev2/api/grpc/v2/summary"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/schollz/progressbar/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	log "github.com/sirupsen/logrus"
)

var globalConn *grpc.ClientConn

func disconnect() {
	if globalConn != nil {
		err := globalConn.Close()
		if err != nil {
			log.WithError(err).Error("failed to close connection")
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
		log.WithError(err).Errorf("failed to connect to grpc: %v", err)
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

	bar := progressbar.NewOptions(
		-1,
		progressbar.OptionSetDescription("Processing workload data..."),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetPredictTime(false),
		progressbar.OptionSetWidth(10),
		progressbar.OptionClearOnFinish(),
	)

	sumResp, err := o.getSummaryPerWorkload(client, data, bar)
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
		log.WithError(err).Error("Failed to get summary")
		return err
	}

	if workload != nil && o.Format == "glance" {
		glance(workload)
		return nil
	}

	if !o.noFilters() {
		workload = filterOpts(workload, o)
	}

	if workload != nil {
		if o.Format == "table" {
			displayWorkloadInTable(workload)
			writeTableToFile(workload)
		} else if o.Format == "json" {
			jsonData, err := json.MarshalIndent(workload, "", "    ")
			if err != nil {
				log.WithError(err).Error("Failed to format workload as JSON")
				return err
			}

			fmt.Println(string(jsonData))
			return nil
		} else { // default
			StartTUI(workload)
		}
	} else {
		fmt.Println("No workloads found.")
	}

	return nil
}

func (o *Options) getSummaryPerWorkload(client summary.SummaryClient, sumReq *summary.SummaryRequest, bar *progressbar.ProgressBar) (*Workload, error) {
	workloadReq := &summary.WorkloadRequest{}

	workloads, err := client.GetWorkloads(context.Background(), workloadReq)
	if err != nil {
		log.WithError(err).Errorf("failed to get workloads: %v", err)
		return nil, err
	}

	rootWorkload := &Workload{
		Clusters: make(map[string]*Cluster),
	}

	sumRespChan := make(chan *summary.SummaryResponse)
	done := make(chan struct{})
	var wg sync.WaitGroup

	for _, w := range workloads.Workloads {
		wg.Add(1)
		go func(w *summary.Workload) {
			defer wg.Done()

			sumReq.Namespaces = []string{w.Namespace}
			sumReq.Clusters = []string{w.Cluster}
			sumResp, err := client.GetSummaryEvent(context.Background(), sumReq) // <sumresp.cluster.namespace.workloadevents.labels>
			if err != nil {
				log.WithError(err).Errorf("error while fetching summary for workload: %s in namespace: %s, error: %s", w.Name, w.Namespace, err.Error())
				return
			}
			sumRespChan <- sumResp
			bar.Add(1)
		}(w)
	}

	go func() {
		wg.Wait()
		close(sumRespChan)
		close(done)

		bar.Finish()
	}()

	go func() {
		for sumResp := range sumRespChan {
			processSummaryResponse(rootWorkload, sumResp, workloads.Workloads) // error is here, we need to pass the value here
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
