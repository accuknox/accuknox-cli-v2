package summary

import (
	"fmt"
	"os"

	"github.com/accuknox/dev2/api/grpc/v2/summary"
	"github.com/olekukonko/tablewriter"
)

func displayWorkloadInTable(workload *Workload) {
	for clusterName, cluster := range workload.Clusters {
		fmt.Printf("Cluster: %s\n", clusterName)
		displayClusterInTable(cluster)
	}
}

func displayClusterInTable(cluster *Cluster) {
	for nsName, namespace := range cluster.Namespaces {
		for deploymentName, workloadEvents := range namespace.Deployments {
			displayWorkloadEvents("File Events", tablewriter.NewWriter(os.Stdout), nsName, deploymentName, workloadEvents, "File")
			displayWorkloadEvents("Process Events", tablewriter.NewWriter(os.Stdout), nsName, deploymentName, workloadEvents, "Process")
			displayWorkloadEvents("Ingress Events", tablewriter.NewWriter(os.Stdout), nsName, deploymentName, workloadEvents, "Ingress")
			displayWorkloadEvents("Egress Events", tablewriter.NewWriter(os.Stdout), nsName, deploymentName, workloadEvents, "Egress")
			displayWorkloadEvents("Bind Events", tablewriter.NewWriter(os.Stdout), nsName, deploymentName, workloadEvents, "Bind")
		}

		for replicaSetName, workloadEvents := range namespace.ReplicaSets {
			displayWorkloadEvents("File Events", tablewriter.NewWriter(os.Stdout), nsName, replicaSetName, workloadEvents, "File")
			displayWorkloadEvents("Process Events", tablewriter.NewWriter(os.Stdout), nsName, replicaSetName, workloadEvents, "Process")
			displayWorkloadEvents("Ingress Events", tablewriter.NewWriter(os.Stdout), nsName, replicaSetName, workloadEvents, "Ingress")
			displayWorkloadEvents("Egress Events", tablewriter.NewWriter(os.Stdout), nsName, replicaSetName, workloadEvents, "Egress")
			displayWorkloadEvents("Bind Events", tablewriter.NewWriter(os.Stdout), nsName, replicaSetName, workloadEvents, "Bind")
		}

		for statefulSetName, workloadEvents := range namespace.StatefulSets {
			displayWorkloadEvents("File Events", tablewriter.NewWriter(os.Stdout), nsName, statefulSetName, workloadEvents, "File")
			displayWorkloadEvents("Process Events", tablewriter.NewWriter(os.Stdout), nsName, statefulSetName, workloadEvents, "Process")
			displayWorkloadEvents("Ingress Events", tablewriter.NewWriter(os.Stdout), nsName, statefulSetName, workloadEvents, "Ingress")
			displayWorkloadEvents("Egress Events", tablewriter.NewWriter(os.Stdout), nsName, statefulSetName, workloadEvents, "Egress")
			displayWorkloadEvents("Bind Events", tablewriter.NewWriter(os.Stdout), nsName, statefulSetName, workloadEvents, "Bind")
		}

		for daemonSetName, workloadEvents := range namespace.DaemonSets {
			displayWorkloadEvents("File Events", tablewriter.NewWriter(os.Stdout), nsName, daemonSetName, workloadEvents, "File")
			displayWorkloadEvents("Process Events", tablewriter.NewWriter(os.Stdout), nsName, daemonSetName, workloadEvents, "Process")
			displayWorkloadEvents("Ingress Events", tablewriter.NewWriter(os.Stdout), nsName, daemonSetName, workloadEvents, "Ingress")
			displayWorkloadEvents("Egress Events", tablewriter.NewWriter(os.Stdout), nsName, daemonSetName, workloadEvents, "Egress")
			displayWorkloadEvents("Bind Events", tablewriter.NewWriter(os.Stdout), nsName, daemonSetName, workloadEvents, "Bind")
		}

		for jobName, workloadEvents := range namespace.Jobs {
			displayWorkloadEvents("File Events", tablewriter.NewWriter(os.Stdout), nsName, jobName, workloadEvents, "File")
			displayWorkloadEvents("Process Events", tablewriter.NewWriter(os.Stdout), nsName, jobName, workloadEvents, "Process")
			displayWorkloadEvents("Ingress Events", tablewriter.NewWriter(os.Stdout), nsName, jobName, workloadEvents, "Ingress")
			displayWorkloadEvents("Egress Events", tablewriter.NewWriter(os.Stdout), nsName, jobName, workloadEvents, "Egress")
			displayWorkloadEvents("Bind Events", tablewriter.NewWriter(os.Stdout), nsName, jobName, workloadEvents, "Bind")
		}

		for cronJobName, workloadEvents := range namespace.CronJobs {
			displayWorkloadEvents("File Events", tablewriter.NewWriter(os.Stdout), nsName, cronJobName, workloadEvents, "File")
			displayWorkloadEvents("Process Events", tablewriter.NewWriter(os.Stdout), nsName, cronJobName, workloadEvents, "Process")
			displayWorkloadEvents("Ingress Events", tablewriter.NewWriter(os.Stdout), nsName, cronJobName, workloadEvents, "Ingress")
			displayWorkloadEvents("Egress Events", tablewriter.NewWriter(os.Stdout), nsName, cronJobName, workloadEvents, "Egress")
			displayWorkloadEvents("Bind Events", tablewriter.NewWriter(os.Stdout), nsName, cronJobName, workloadEvents, "Bind")
		}
	}
}

func displayWorkloadEvents(eventType string, writer *tablewriter.Table, nsName, workloadName string, workloadEvents *WorkloadEvents, eventTypeKey string) {
	var events interface{}
	switch eventTypeKey {
	case "File":
		events = workloadEvents.Events.File
	case "Process":
		events = workloadEvents.Events.Process
	case "Ingress":
		events = workloadEvents.Events.Ingress
	case "Egress":
		events = workloadEvents.Events.Egress
	case "Bind":
		events = workloadEvents.Events.Bind
	}
	displayEvents(eventType, writer, nsName, workloadName, events)
}

func displayEvents(title string, table *tablewriter.Table, nsName string, wtName string, events interface{}) {
	eventCount := getEventCount(events)
	if eventCount == 0 {
		return // Skip rendering if there are no events
	}

	fmt.Printf("%s for Namespace '%s', Workload Type '%s':\n", title, nsName, wtName)

	switch e := events.(type) {
	case []*summary.ProcessFileEvent:
		table.SetHeader([]string{"Pod", "Container", "Image Name", "Source", "Destination", "Count", "Updated Time"})
		for _, event := range e {
			row := []string{
				event.Pod,
				event.Container.Name,
				event.Container.Image,
				event.Source,
				event.Destination,
				fmt.Sprintf("%d", event.Count),
				fmt.Sprintf("%d", event.UpdatedTime),
			}
			table.Append(row)
		}
	case []*summary.NetworkEvent:
		table.SetHeader([]string{"Pod", "Container", "Image Name", "IP", "Port", "Protocol", "Peer Domain Name", "Count", "Updated Time"})
		for _, event := range e {
			row := []string{
				event.Pod,
				event.Container.Name,
				event.Container.Image,
				event.Ip,
				fmt.Sprintf("%d", event.Port),
				event.Protocol,
				event.PeerDomainName,
				fmt.Sprintf("%d", event.Count),
				fmt.Sprintf("%d", event.UpdatedTime),
			}
			table.Append(row)
		}
	}

	table.Render()
	fmt.Println()
}

/* func writeTableToFile(workload *Workload) error {
	fmt.Println()

	outDir := "knoxctl_out/summary/table"
	if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fileName := "summary.txt"
	filePath := filepath.Join(outDir, fileName)

	filePath = filepath.Clean(filePath)
	if filepath.IsAbs(filePath) {
		return fmt.Errorf("invalid file path: path must not be absolute")
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	totalProgressSteps := calculateTotalProgressSteps(workload)
	bar := initializeProgressBar("Writing Summary to File...", totalProgressSteps)
	for clusterName, cluster := range workload.Clusters {
		fmt.Fprintf(file, "Cluster: %s\n", clusterName)
		writeClusterToTable(cluster, file, bar)
	}

	fmt.Printf("Table based text summary written to %s\n", filePath)
	return nil
}

func writeClusterToTable(cluster *Cluster, file *os.File, bar *progressbar.ProgressBar) {
	for nsName, namespace := range cluster.Namespaces {
		for wtName, workloadType := range namespace.WorkloadTypes {
			writeEventsToFile("File Events", tablewriter.NewWriter(file), file, nsName, wtName, workloadType.Events.File, bar)
			writeEventsToFile("Process Events", tablewriter.NewWriter(file), file, nsName, wtName, workloadType.Events.Process, bar)
			writeEventsToFile("Ingress Events", tablewriter.NewWriter(file), file, nsName, wtName, workloadType.Events.Ingress, bar)
			writeEventsToFile("Egress Events", tablewriter.NewWriter(file), file, nsName, wtName, workloadType.Events.Egress, bar)
			writeEventsToFile("Bind Events", tablewriter.NewWriter(file), file, nsName, wtName, workloadType.Events.Bind, bar)
		}
	}
}

func writeEventsToFile(title string, table *tablewriter.Table, file *os.File, nsName string, wtName string, events interface{}, bar *progressbar.ProgressBar) {
	eventCount := getEventCount(events)
	if eventCount == 0 {
		return // Skip rendering if there are no events
	}

	fmt.Fprintf(file, "%s for Namespace '%s', Workload Type '%s':\n", title, nsName, wtName)

	switch e := events.(type) {
	case []*summary.ProcessFileEvent:
		table.SetHeader([]string{"Pod", "Container", "Image Name", "Source", "Destination", "Count", "Updated Time"})
		for _, event := range e {
			row := []string{
				event.Pod,
				event.Container.Name,
				event.Container.Image,
				event.Source,
				event.Destination,
				fmt.Sprintf("%d", event.Count),
				fmt.Sprintf("%d", event.UpdatedTime),
			}
			table.Append(row)
		}
	case []*summary.NetworkEvent:
		table.SetHeader([]string{"Pod", "Container", "Image Name", "IP", "Port", "Protocol", "Peer Domain Name", "Count", "Updated Time"})
		for _, event := range e {
			row := []string{
				event.Pod,
				event.Container.Name,
				event.Container.Image,
				event.Ip,
				fmt.Sprintf("%d", event.Port),
				event.Protocol,
				event.PeerDomainName,
				fmt.Sprintf("%d", event.Count),
				fmt.Sprintf("%d", event.UpdatedTime),
			}
			table.Append(row)
		}
	}

	err := bar.Add(1)
	if err != nil {
		fmt.Printf("Error incrementing progress bar: %s\n", err)
	}
	table.Render()
	fmt.Fprintln(file)
} */

func getEventCount(events interface{}) int {
	switch e := events.(type) {
	case []*summary.ProcessFileEvent:
		return len(e)
	case []*summary.NetworkEvent:
		return len(e)
	default:
		return 0
	}
}

/* func calculateTotalProgressSteps(workload *Workload) int {
	total := 0
	for _, cluster := range workload.Clusters {
		for _, namespace := range cluster.Namespaces {
			for _, workloadType := range namespace.WorkloadTypes {
				total += getEventGroupCount(workloadType)
			}
		}
	}
	return total
}

func getEventGroupCount(workloadType *WorkloadType) int {
	count := 0
	eventGroups := []interface{}{
		workloadType.Events.File,
		workloadType.Events.Process,
		workloadType.Events.Ingress,
		workloadType.Events.Egress,
		workloadType.Events.Bind,
	}

	for _, events := range eventGroups {
		if getEventCount(events) > 0 {
			count++
		}
	}

	return count
}
*/
