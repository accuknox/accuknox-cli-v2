package summary

import (
	"regexp"

	"github.com/accuknox/dev2/api/grpc/v2/summary"
)

// Needs fixing
func filterOpts(workload *Workload, opts Options) *Workload {
	filteredWorkload := &Workload{
		Clusters: make(map[string]*Cluster),
	}

	namespaceFilterProvided := len(opts.NamespaceRegex) > 0 || len(opts.Namespace) > 0

	for clusterName, cluster := range workload.Clusters {
		newCluster := &Cluster{
			Namespaces: make(map[string]*Namespace),
		}

		for nsName, namespace := range cluster.Namespaces {
			if !namespaceFilterProvided || matchesNamespace(nsName, opts) {
				newNamespace := &Namespace{
					Deployments:  filterWorkloadEvents(namespace.Deployments, opts),
					ReplicaSets:  filterWorkloadEvents(namespace.ReplicaSets, opts),
					StatefulSets: filterWorkloadEvents(namespace.StatefulSets, opts),
					DaemonSets:   filterWorkloadEvents(namespace.DaemonSets, opts),
					Jobs:         filterWorkloadEvents(namespace.Jobs, opts),
					CronJobs:     filterWorkloadEvents(namespace.CronJobs, opts),
				}

				newCluster.Namespaces[nsName] = newNamespace
			}
		}

		if len(newCluster.Namespaces) > 0 {
			filteredWorkload.Clusters[clusterName] = newCluster
		}
	}

	return filteredWorkload
}

func filterWorkloadEvents(workloadEventsMap map[string]*WorkloadEvents, opts Options) map[string]*WorkloadEvents {
	filteredEventsMap := make(map[string]*WorkloadEvents)

	for wtName, workloadEvents := range workloadEventsMap {
		if len(opts.LabelsRegex) > 0 || len(opts.DestinationRegex) > 0 || len(opts.SourceRegex) > 0 {
			filteredEvents := &WorkloadEvents{
				Events: &Events{
					File:    filterEvents(workloadEvents.Events.File, opts),
					Process: filterEvents(workloadEvents.Events.Process, opts),
				},
			}
			filteredEventsMap[wtName] = filteredEvents
		} else {
			filteredEventsMap[wtName] = workloadEvents
		}
	}

	return filteredEventsMap
}

func filterEvents(events []*summary.ProcessFileEvent, opts Options) []*summary.ProcessFileEvent {
	filteredEvents := []*summary.ProcessFileEvent{}

	for _, event := range events {
		if matches(event, opts) {
			filteredEvents = append(filteredEvents, event)
		}
	}

	return filteredEvents
}

// Update matchesRegex, matchRegexLabels, labelsMatch, and matchesNamespace functions accordingly.
func matchesRegex(patterns []*regexp.Regexp, text string) bool {
	for _, regex := range patterns {
		if regex.MatchString(text) {
			return true
		}
	}
	return false
}

func matches(event *summary.ProcessFileEvent, opts Options) bool {
	if len(opts.DestinationRegex) > 0 && matchesRegex(opts.DestinationRegex, event.Destination) {
		return true
	}
	if len(opts.SourceRegex) > 0 && matchesRegex(opts.SourceRegex, event.Source) {
		return true
	}
	return false
}

func labelsMatch(workloadEvents *summary.WorkloadEvents, opts Options) bool {
	if len(opts.LabelsRegex) > 0 && matchesRegex(opts.LabelsRegex, workloadEvents.Labels) {
		return true
	}
	return false
}

func matchesNamespace(name string, opts Options) bool {
	if stringInSlice(name, opts.Namespace) || matchesRegex(opts.NamespaceRegex, name) {
		return true
	}
	return false
}

func stringInSlice(str string, slice []string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}
