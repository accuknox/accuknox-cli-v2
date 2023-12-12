package summary

import (
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
)

func glance(workload *Workload) {
	clusterCount := len(workload.Clusters)
	namespaceCount, _, _, highestEvents := getSummaryStats(workload)

	table := tablewriter.NewWriter(os.Stdout)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetBorder(true)
	table.SetAutoWrapText(false)

	title := []string{"Quick Glance at Workload Summary without any filters", ""}
	table.SetHeader(title)
	table.AppendBulk([][]string{
		{"Total Clusters", fmt.Sprintf("%d", clusterCount)},
		{"Total Namespaces", fmt.Sprintf("%d", namespaceCount)},
	})

	table.Append([]string{"", ""})

	table.AppendBulk([][]string{
		{"Namespace with Highest Events", fmt.Sprintf("%s (%d events)", highestEvents.Namespace.Name, highestEvents.Namespace.Count)},
		{"Workload Type with Highest Events", fmt.Sprintf("%s (%d events)", highestEvents.WorkloadType.Name, highestEvents.WorkloadType.Count)},
		{"Highest File Events", fmt.Sprintf("%d in %s (Namespace: %s)", highestEvents.File.Count, highestEvents.File.Name, highestEvents.File.NSName)},
		{"Highest Process Events", fmt.Sprintf("%d in %s (Namespace: %s)", highestEvents.Process.Count, highestEvents.Process.Name, highestEvents.Process.NSName)},
		{"Highest Ingress Events", fmt.Sprintf("%d in %s (Namespace: %s)", highestEvents.Ingress.Count, highestEvents.Ingress.Name, highestEvents.Ingress.NSName)},
		{"Highest Egress Events", fmt.Sprintf("%d in %s (Namespace: %s)", highestEvents.Egress.Count, highestEvents.Egress.Name, highestEvents.Egress.NSName)},
		{"Highest Bind Events", fmt.Sprintf("%d in %s (Namespace: %s)", highestEvents.Bind.Count, highestEvents.Bind.Name, highestEvents.Bind.NSName)},
	})

	table.Render()
}

type HighestEvents struct {
	Namespace    Highest
	WorkloadType Highest
	File         HighestCountWithName
	Process      HighestCountWithName
	Ingress      HighestCountWithName
	Egress       HighestCountWithName
	Bind         HighestCountWithName
}

type Highest struct {
	Name  string
	Count int
}

type HighestCountWithName struct {
	Count  int
	Name   string
	NSName string
}

func getSummaryStats(workload *Workload) (int, int, int, HighestEvents) {
	var namespaceCount, workloadTypeCount, totalEvents int
	highest := HighestEvents{}

	for _, cluster := range workload.Clusters {
		namespaceCount += len(cluster.Namespaces)
		for nsName, namespace := range cluster.Namespaces {
			nsTotalEvents := 0

			for _, workloadEvents := range namespace.Deployments {
				updateNamespaceStats(&highest, nsName, "Deployments", workloadEvents, &nsTotalEvents)
				workloadTypeCount++
			}

			for _, workloadEvents := range namespace.ReplicaSets {
				updateNamespaceStats(&highest, nsName, "ReplicaSets", workloadEvents, &nsTotalEvents)
				workloadTypeCount++
			}

			for _, workloadEvents := range namespace.StatefulSets {
				updateNamespaceStats(&highest, nsName, "StatefulSets", workloadEvents, &nsTotalEvents)
				workloadTypeCount++
			}

			for _, workloadEvents := range namespace.DaemonSets {
				updateNamespaceStats(&highest, nsName, "DaemonSets", workloadEvents, &nsTotalEvents)
				workloadTypeCount++
			}

			for _, workloadEvents := range namespace.Jobs {
				updateNamespaceStats(&highest, nsName, "Jobs", workloadEvents, &nsTotalEvents)
				workloadTypeCount++
			}

			for _, workloadEvents := range namespace.CronJobs {
				updateNamespaceStats(&highest, nsName, "CronJobs", workloadEvents, &nsTotalEvents)
				workloadTypeCount++
			}

			if nsTotalEvents > highest.Namespace.Count {
				highest.Namespace = Highest{Name: nsName, Count: nsTotalEvents}
			}
		}
	}

	return namespaceCount, workloadTypeCount, totalEvents, highest
}

func updateNamespaceStats(highest *HighestEvents, nsName, wtName string, workloadEvents *WorkloadEvents, nsTotalEvents *int) {
	fileEvents := len(workloadEvents.Events.File)
	processEvents := len(workloadEvents.Events.Process)
	ingressEvents := len(workloadEvents.Events.Ingress)
	egressEvents := len(workloadEvents.Events.Egress)
	bindEvents := len(workloadEvents.Events.Bind)
	wtTotalEvents := fileEvents + processEvents + ingressEvents + egressEvents + bindEvents

	*nsTotalEvents += wtTotalEvents
}
