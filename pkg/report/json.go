package report

import (
	"fmt"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/summary"
	"github.com/clarketm/json"
)

func (g Graph) generateJSON(rootHash string) *summary.Workload {
	dfsResult := g.DepthFirstSearch(rootHash)
	level4Nodes := g.groupLevel4Nodes(dfsResult)

	workload := &summary.Workload{
		Clusters: make(map[string]*summary.Cluster),
	}

	for _, nodes := range level4Nodes {
		for _, node := range nodes {
			if len(node.Change.Insert) == 0 || node.Change.Canceled {
				continue
			}

			pathSegments := strings.Split(node.Path, "/")
			if len(pathSegments) < 9 {
				continue
			}

			clusterName := pathSegments[2]   // name of the cluster
			namespaceName := pathSegments[4] // name of the namespace
			resourceType := pathSegments[6]  // type of resource we are dealing with
			resourceName := pathSegments[8]  // name of the resource
			eventType := pathSegments[10]    // type of current event

			if _, ok := workload.Clusters[clusterName]; !ok {
				workload.Clusters[clusterName] = &summary.Cluster{
					Namespaces: make(map[string]*summary.Namespace),
				}
			}

			cluster := workload.Clusters[clusterName]
			if _, ok := cluster.Namespaces[namespaceName]; !ok {
				cluster.Namespaces[namespaceName] = &summary.Namespace{
					Deployments:  make(map[string]*summary.WorkloadEvents),
					ReplicaSets:  make(map[string]*summary.WorkloadEvents),
					StatefulSets: make(map[string]*summary.WorkloadEvents),
					DaemonSets:   make(map[string]*summary.WorkloadEvents),
					Jobs:         make(map[string]*summary.WorkloadEvents),
					CronJobs:     make(map[string]*summary.WorkloadEvents),
				}
			}

			// Ensure workload event exists
			namespace := cluster.Namespaces[namespaceName]
			var workloadEvents *summary.WorkloadEvents

			switch resourceType {
			case "deployment":
				if _, ok := namespace.Deployments[resourceName]; !ok {
					namespace.Deployments[resourceName] = &summary.WorkloadEvents{
						Events: &summary.Events{},
					}
				}
				workloadEvents = namespace.Deployments[resourceName]
			case "replicaSet":
				if _, ok := namespace.ReplicaSets[resourceName]; !ok {
					namespace.ReplicaSets[resourceName] = &summary.WorkloadEvents{
						Events: &summary.Events{},
					}
				}
				workloadEvents = namespace.ReplicaSets[resourceName]
			case "statefulSet":
				if _, ok := namespace.StatefulSets[resourceName]; !ok {
					namespace.StatefulSets[resourceName] = &summary.WorkloadEvents{
						Events: &summary.Events{},
					}
				}
				workloadEvents = namespace.StatefulSets[resourceName]
			case "daemonSet":
				if _, ok := namespace.DaemonSets[resourceName]; !ok {
					namespace.DaemonSets[resourceName] = &summary.WorkloadEvents{
						Events: &summary.Events{},
					}
				}
				workloadEvents = namespace.DaemonSets[resourceName]
			case "job":
				if _, ok := namespace.Jobs[resourceName]; !ok {
					namespace.Jobs[resourceName] = &summary.WorkloadEvents{
						Events: &summary.Events{},
					}
				}
				workloadEvents = namespace.Jobs[resourceName]
			case "cronJob":
				if _, ok := namespace.CronJobs[resourceName]; !ok {
					namespace.CronJobs[resourceName] = &summary.WorkloadEvents{
						Events: &summary.Events{},
					}
				}
				workloadEvents = namespace.CronJobs[resourceName]
			}

			if workloadEvents != nil {
				switch eventType {
				case "ingress":
					if node.NetworkData != nil {
						workloadEvents.Events.Ingress = append(workloadEvents.Events.Ingress, node.NetworkData)
					}
				case "egress":
					if node.NetworkData != nil {
						workloadEvents.Events.Egress = append(workloadEvents.Events.Egress, node.NetworkData)
					}
				case "bind":
					if node.NetworkData != nil {
						workloadEvents.Events.Bind = append(workloadEvents.Events.Bind, node.NetworkData)
					}
				case "file":
					if node.FileProcessData != nil {
						workloadEvents.Events.File = append(workloadEvents.Events.File, node.FileProcessData)
					}
				case "process":
					if node.FileProcessData != nil {
						workloadEvents.Events.Process = append(workloadEvents.Events.Process, node.FileProcessData)
					}
				}
			}

		}
	}

	return workload
}

func (g Graph) writeDiffJSON(fileName, rootHash string) error {
	workload := g.generateJSON(rootHash)

	jsonData, err := json.MarshalIndent(workload, "", "    ")
	if err != nil {
		return err
	}

	err = common.CleanAndWrite(fileName, jsonData)
	if err != nil {
		return err
	}

	fmt.Printf("Diff json file written to: %s", fileName)
	return nil
}
