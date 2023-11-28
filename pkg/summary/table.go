package summary

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/accuknox/dev2/api/grpc/v2/summary"
	"github.com/olekukonko/tablewriter"
	"github.com/schollz/progressbar/v3"
)

func displayWorkloadInTable(workload *Workload) {
	for clusterName, cluster := range workload.Clusters {
		fmt.Printf("Cluster: %s\n", clusterName)
		displayClusterInTable(cluster)
	}
}

func displayClusterInTable(cluster *Cluster) {
	for nsName, namespace := range cluster.Namespaces {
		for wtName, workloadType := range namespace.WorkloadTypes {
			displayEvents("File Events", tablewriter.NewWriter(os.Stdout), nsName, wtName, workloadType.Events.File)
			displayEvents("Process Events", tablewriter.NewWriter(os.Stdout), nsName, wtName, workloadType.Events.Process)
			displayEvents("Ingress Events", tablewriter.NewWriter(os.Stdout), nsName, wtName, workloadType.Events.Ingress)
			displayEvents("Egress Events", tablewriter.NewWriter(os.Stdout), nsName, wtName, workloadType.Events.Egress)
			displayEvents("Bind Events", tablewriter.NewWriter(os.Stdout), nsName, wtName, workloadType.Events.Bind)
		}
	}
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

func writeTableToFile(workload *Workload) error {
	fmt.Println()
	fmt.Println("Writing summary to file...")

	outDir := "knoxctl_out/summary/table"
	if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fileName := "summary.txt"
	filePath := filepath.Join(outDir, fileName)

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

	fmt.Printf("Summary written to %s\n", filePath)
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

	bar.Add(1)
	table.Render()
	fmt.Fprintln(file)
}

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

func calculateTotalProgressSteps(workload *Workload) int {
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
