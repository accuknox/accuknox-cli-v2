package summary

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/olekukonko/tablewriter"
)

func displayWorkloadInTable(workload *Workload) {
	for clusterName, cluster := range workload.Clusters {
		fmt.Printf("Cluster: %s\n", clusterName)
		displayClusterInTable(cluster)
	}
}

func displayClusterInTable(cluster *Cluster) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Namespace", "Workload Type", "File Events", "Process Events", "Ingress Events", "Egress Events", "Bind Events"})

	for nsName, namespace := range cluster.Namespaces {
		for wtName, workloadType := range namespace.WorkloadTypes {
			fileEvents := len(workloadType.Events.File)
			processEvents := len(workloadType.Events.Process)
			ingressEvents := len(workloadType.Events.Ingress)
			egressEvents := len(workloadType.Events.Egress)
			bindEvents := len(workloadType.Events.Bind)

			row := []string{
				nsName,
				wtName,
				fmt.Sprintf("%d", fileEvents),
				fmt.Sprintf("%d", processEvents),
				fmt.Sprintf("%d", ingressEvents),
				fmt.Sprintf("%d", egressEvents),
				fmt.Sprintf("%d", bindEvents),
			}
			table.Append(row)
		}
	}

	table.Render()
	fmt.Println()
}

func writeTableToFile(workload *Workload) error {
	outDir := "out"
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

	for clusterName, cluster := range workload.Clusters {
		fmt.Fprintf(file, "Cluster: %s\n", clusterName)
		writeClusterToTable(cluster, file)
	}

	fmt.Printf("Summary written to %s\n", filePath)
	return nil
}

func writeClusterToTable(cluster *Cluster, file *os.File) {
	table := tablewriter.NewWriter(file)
	table.SetHeader([]string{"Namespace", "Workload Type", "File Events", "Process Events", "Ingress Events", "Egress Events", "Bind Events"})

	for nsName, namespace := range cluster.Namespaces {
		for wtName, workloadType := range namespace.WorkloadTypes {
			fileEvents := len(workloadType.Events.File)
			processEvents := len(workloadType.Events.Process)
			ingressEvents := len(workloadType.Events.Ingress)
			egressEvents := len(workloadType.Events.Egress)
			bindEvents := len(workloadType.Events.Bind)

			row := []string{
				nsName,
				wtName,
				fmt.Sprintf("%d", fileEvents),
				fmt.Sprintf("%d", processEvents),
				fmt.Sprintf("%d", ingressEvents),
				fmt.Sprintf("%d", egressEvents),
				fmt.Sprintf("%d", bindEvents),
			}
			table.Append(row)
		}
	}

	table.Render()
	fmt.Fprintln(file)
}
