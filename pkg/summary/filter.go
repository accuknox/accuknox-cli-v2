package summary

import "github.com/accuknox/dev2/api/grpc/v2/summary"

// Needs fixing
func filterOpts(workload *Workload, opts Options) *Workload {
	filteredWorkload := &Workload{
		Clusters: make(map[string]*Cluster),
	}

	namespaceFilterProvided := len(opts.NamespaceRegex) > 0 || len(opts.Namespace) > 0

	for clusterName, cluster := range workload.Clusters {
		newCluster := &Cluster{
			Cluster:    cluster.Cluster,
			Namespaces: make(map[string]*Namespace),
		}

		for nsName, namespace := range cluster.Namespaces {
			if !namespaceFilterProvided || matchesNamespace(nsName, opts) {
				newNamespace := &Namespace{
					Namespace:     namespace.Namespace,
					WorkloadTypes: make(map[string]*WorkloadType),
				}

				for wtName, workloadType := range namespace.WorkloadTypes {
					if len(opts.LabelsRegex) > 0 || len(opts.DestinationRegex) > 0 || len(opts.SourceRegex) > 0 {
						newWorkloadType := &WorkloadType{
							Events: &Events{
								File:    filterEvents(workloadType.Events.File, workloadType, opts),
								Process: filterEvents(workloadType.Events.Process, workloadType, opts),
							},
						}
						newNamespace.WorkloadTypes[wtName] = newWorkloadType
					} else {
						newNamespace.WorkloadTypes[wtName] = workloadType
					}
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

func matchesNamespaceRegex(name string, opts Options) bool {
	for _, regex := range opts.NamespaceRegex {
		if regex.MatchString(name) {
			return true
		}
	}
	return false
}

func matchesRegex(event *summary.ProcessFileEvent, workloadEvent *summary.WorkloadEvents, opts Options) bool {
	if len(opts.DestinationRegex) > 0 {
		for _, regex := range opts.DestinationRegex {
			if regex.MatchString(event.Destination) {
				return true
			}
		}
	}

	if len(opts.SourceRegex) > 0 {
		for _, regex := range opts.SourceRegex {
			if regex.MatchString(event.Source) {
				return true
			}
		}
	}

	return false
}

func matchRegexLabels(workloadEvents *summary.WorkloadEvents, opts Options) bool {
	if len(opts.LabelsRegex) > 0 && workloadEvents != nil {
		for _, regex := range opts.LabelsRegex {
			if regex.MatchString(workloadEvents.Labels) {
				return true
			}
		}
	}
	return false
}

func stringInSlice(str string, list []string) bool {
	for _, v := range list {
		if v == str {
			return true
		}
	}
	return false
}

func filterEvents(events []*summary.ProcessFileEvent, wlType *WorkloadType, opts Options) []*summary.ProcessFileEvent {
	filteredEvents := []*summary.ProcessFileEvent{}

	if !labelsMatch(wlType.WorkloadEvents, opts) {
		return filteredEvents
	}

	for _, event := range events {
		if matches(event, wlType.WorkloadEvents, opts) {
			filteredEvents = append(filteredEvents, event)
		}
	}

	return filteredEvents
}

func matches(event *summary.ProcessFileEvent, workloadEvent *summary.WorkloadEvents, opts Options) bool {
	if matchesRegex(event, workloadEvent, opts) {
		return true
	}

	if len(opts.Destination) > 0 && stringInSlice(event.Destination, opts.Destination) {
		return true
	}
	if len(opts.Source) > 0 && stringInSlice(event.Source, opts.Source) {
		return true
	}

	return false
}

func labelsMatch(workloadEvents *summary.WorkloadEvents, opts Options) bool {
	if matchRegexLabels(workloadEvents, opts) {
		return true
	}

	if len(opts.Labels) > 0 && stringInSlice(workloadEvents.Labels, opts.Labels) {
		return true
	}

	return false
}

func matchesNamespace(name string, opts Options) bool {
	if stringInSlice(name, opts.Namespace) || matchesNamespaceRegex(name, opts) {
		return true
	}
	return false
}
