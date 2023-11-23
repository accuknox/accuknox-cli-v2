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
			for wtName, workloadType := range namespace.WorkloadTypes {
				workloadTypeCount++
				fileEvents := len(workloadType.Events.File)
				processEvents := len(workloadType.Events.Process)
				ingressEvents := len(workloadType.Events.Ingress)
				egressEvents := len(workloadType.Events.Egress)
				bindEvents := len(workloadType.Events.Bind)
				wtTotalEvents := fileEvents + processEvents + ingressEvents + egressEvents + bindEvents

				updateHighestEvents(&highest, nsName, wtName,
					fileEvents, processEvents, ingressEvents, egressEvents, bindEvents, wtTotalEvents)

				nsTotalEvents += wtTotalEvents
			}

			if nsTotalEvents > highest.Namespace.Count {
				highest.Namespace = Highest{Name: nsName, Count: nsTotalEvents}
			}
		}
	}

	return namespaceCount, workloadTypeCount, totalEvents, highest
}

func updateHighestEvents(highest *HighestEvents, nsName, wtName string,
	fileEvents, processEvents, ingressEvents, egressEvents, bindEvents, wtTotalEvents int) {

	if wtTotalEvents > highest.WorkloadType.Count {
		highest.WorkloadType = Highest{
			Name:  fmt.Sprintf("%s (%s)", wtName, nsName),
			Count: wtTotalEvents,
		}
	}

	updateHighestCountWithName(&highest.File, fileEvents, wtName, nsName)
	updateHighestCountWithName(&highest.Process, processEvents, wtName, nsName)
	updateHighestCountWithName(&highest.Ingress, ingressEvents, wtName, nsName)
	updateHighestCountWithName(&highest.Egress, egressEvents, wtName, nsName)
	updateHighestCountWithName(&highest.Bind, bindEvents, wtName, nsName)
}

func updateHighestCountWithName(h *HighestCountWithName, newCount int, wtName, nsName string) {
	if newCount > h.Count {
		h.Count = newCount
		h.Name = wtName
		h.NSName = nsName
	}
}
