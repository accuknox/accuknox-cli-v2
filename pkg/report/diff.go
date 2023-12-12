package report

import (
	"strconv"

	"github.com/accuknox/accuknox-cli-v2/pkg/summary"
	dev2summary "github.com/accuknox/dev2/api/grpc/v2/summary"
)

// Difference calculatest the difference between two summaries, latest and baseline.
// It returns a summary.Workload struct that contains the differences between the two summaries.
// The actual changes are tracked by the Graph tracker, which is used to find granular differences, and generate reports.
// We only go to do deep comparison if the hashes of JSON subtrees doesn't matches, to avoid unnecessary comparisons.
func Difference(latestSummary, baselineSummary *summary.Workload, tracker *Graph) *summary.Workload {
	diffResult := &summary.Workload{}

	if latestSummary.GetHash() == baselineSummary.GetHash() {
		diffResult = latestSummary
		return diffResult
	}

	rootHash := latestSummary.GetHash()
	currentPath := "workload"

	tracker.AddNode(&Node{
		Type:   "workload",
		Hash:   rootHash,
		Path:   currentPath,
		Level:  0,
		Change: ChangeType{},
	}, "")

	diffResult.Clusters = compareClusters(latestSummary.Clusters, baselineSummary.Clusters, tracker, rootHash, currentPath)
	return diffResult
}

// compareClusters compares clusters
func compareClusters(latest, baseline map[string]*summary.Cluster, tracker *Graph, parentHash string, parentPath string) map[string]*summary.Cluster {
	diffClusters := make(map[string]*summary.Cluster)

	for key, latestCluster := range latest {
		if baseline, ok := baseline[key]; ok {
			if latestCluster.GetHash() != baseline.GetHash() {
				clusterHash := latestCluster.GetHash()
				clusterInfo := make(map[K]V)
				clusterInfoKey := "Cluster-" + key
				clusterInfo[K(clusterInfoKey)] = V(latestCluster.ClusterName)
				currentPath := parentPath + "/cluster/" + latestCluster.ClusterName

				tracker.AddNode(&Node{
					Type:   "cluster",
					Hash:   clusterHash,
					Path:   currentPath,
					Level:  1,
					Change: ChangeType{},
				}, parentHash)

				diffClusters[key] = compareNamespaces(latestCluster.Namespaces, baseline.Namespaces, tracker, clusterHash, currentPath)
			}
		} else {
			diffClusters[key] = latestCluster
		}
	}

	for key, baselineCluster := range baseline {
		if _, ok := latest[key]; !ok {
			diffClusters[key] = baselineCluster
		}
	}

	return diffClusters
}

// compareNamespaces compares namespaces
func compareNamespaces(latest, baseline map[string]*summary.Namespace, tracker *Graph, parentHash string, parentPath string) *summary.Cluster {
	diffNamespaces := make(map[string]*summary.Namespace)

	for key, latestNamespace := range latest {
		if baseline, ok := baseline[key]; ok {
			if latestNamespace.GetHash() != baseline.GetHash() {
				nsHash := latestNamespace.GetHash()
				nsInfo := make(map[K]V)
				nsInfoKey := "Namespace-" + key
				nsInfo[K(nsInfoKey)] = V(latestNamespace.NamespaceName)
				currentPath := parentPath + "/namespace/" + latestNamespace.NamespaceName

				tracker.AddNode(&Node{
					Type:   "namespace",
					Hash:   nsHash,
					Path:   currentPath,
					Level:  2,
					Change: ChangeType{},
				}, parentHash)

				diffNamespaces[key] = compareWorkloadEvents(latestNamespace, baseline, tracker, nsHash, currentPath)
			}
		} else {
			diffNamespaces[key] = latestNamespace
		}
	}

	for key, baselineNamespace := range baseline {
		if _, ok := latest[key]; !ok {
			diffNamespaces[key] = baselineNamespace
		}
	}

	return &summary.Cluster{
		Namespaces: diffNamespaces,
	}
}

// compareWorkloadEvents is a wrapper that calls compareWorkloadEventsMap for each workload type
func compareWorkloadEvents(latest, baseline *summary.Namespace, tracker *Graph, parentHash string, parentPath string) *summary.Namespace {
	diffWorkloadEvents := &summary.Namespace{
		Deployments:  compareWorkloadEventsMap(latest.Deployments, baseline.Deployments, tracker, parentHash, parentPath, "deployment"),
		ReplicaSets:  compareWorkloadEventsMap(latest.ReplicaSets, baseline.ReplicaSets, tracker, parentHash, parentPath, "replicaset"),
		StatefulSets: compareWorkloadEventsMap(latest.StatefulSets, baseline.StatefulSets, tracker, parentHash, parentPath, "statefulset"),
		DaemonSets:   compareWorkloadEventsMap(latest.DaemonSets, baseline.DaemonSets, tracker, parentHash, parentPath, "daemonset"),
		Jobs:         compareWorkloadEventsMap(latest.Jobs, baseline.Jobs, tracker, parentHash, parentPath, "job"),
		CronJobs:     compareWorkloadEventsMap(latest.CronJobs, baseline.CronJobs, tracker, parentHash, parentPath, "cronjob"),
	}

	return diffWorkloadEvents
}

// compareWorkloadEventsMap compares workload events at more granular level
func compareWorkloadEventsMap(latest, baseline map[string]*summary.WorkloadEvents, tracker *Graph, parentHash, parentPath, wlType string) map[string]*summary.WorkloadEvents {
	diffWorkloadEventsMap := make(map[string]*summary.WorkloadEvents)
	weInfo := make(map[K]V)

	for key, latestWorkloadEvents := range latest {
		if baseline, ok := baseline[key]; ok {
			if latestWorkloadEvents.GetHash() != baseline.GetHash() {
				weHash := latestWorkloadEvents.GetHash()
				weInfoKey := "WorkloadEvents-" + key
				weInfo[K(weInfoKey)] = V(latestWorkloadEvents.WorkloadName)
				currentPath := parentPath + "/resource-type/" + wlType + "/resource-name/" + latestWorkloadEvents.WorkloadName

				tracker.AddNode(&Node{
					Type:   "workload-events",
					Hash:   weHash,
					Path:   currentPath,
					Level:  3,
					Change: ChangeType{},
				}, parentHash)

				diffWorkloadEventsMap[key] = compareEvents(latestWorkloadEvents, baseline, tracker, weHash, currentPath)
			}
		} else {
			diffWorkloadEventsMap[key] = latestWorkloadEvents
		}
	}

	for key, baselineWorkloadEvents := range baseline {
		if _, ok := latest[key]; !ok {
			diffWorkloadEventsMap[key] = baselineWorkloadEvents
		}
	}

	return diffWorkloadEventsMap
}

// compareEvents compares events
func compareEvents(latest, baseline *summary.WorkloadEvents, tracker *Graph, parentHash string, parentPath string) *summary.WorkloadEvents {
	if latest == nil || baseline == nil {
		return &summary.WorkloadEvents{}
	}

	diffEvents := &summary.WorkloadEvents{}

	if latest.Events != nil && baseline.Events != nil {

		if len(latest.Events.File) > 0 && len(baseline.Events.File) > 0 {
			currentPath := parentPath + "/events/file"
			actualEvent := "file"
			results := compareFileEvents(latest.Events.File, baseline.Events.File, tracker, parentHash, currentPath, actualEvent)

			if diffEvents.Events == nil {
				diffEvents.Events = &summary.Events{}
			}

			for _, result := range results {
				if result != nil {
					diffEvents.Events.File = append(diffEvents.Events.File, result)
				}
			}
		}

		if len(latest.Events.Process) > 0 && len(baseline.Events.Process) > 0 {
			currentPath := parentPath + "/events/process"
			actualEvent := "process"
			results := compareFileEvents(latest.Events.Process, baseline.Events.Process, tracker, parentHash, currentPath, actualEvent)

			if diffEvents.Events == nil {
				diffEvents.Events = &summary.Events{}
			}

			for _, result := range results {
				if result != nil {
					diffEvents.Events.Process = append(diffEvents.Events.Process, result)
				}
			}
		}

		if len(latest.Events.Ingress) > 0 && len(baseline.Events.Ingress) > 0 {
			currentPath := parentPath + "/events/ingress"
			actualEvent := "ingress"
			results := compareNetworkEvents(latest.Events.Ingress, baseline.Events.Ingress, tracker, parentHash, currentPath, actualEvent)

			if diffEvents.Events == nil {
				diffEvents.Events = &summary.Events{}
			}

			for _, result := range results {
				if result != nil {
					diffEvents.Events.Ingress = append(diffEvents.Events.Ingress, result)
				}
			}
		}

		if len(latest.Events.Egress) > 0 && len(baseline.Events.Egress) > 0 {
			currentPath := parentPath + "/events/egress"
			actualEvent := "egress"
			results := compareNetworkEvents(latest.Events.Egress, baseline.Events.Egress, tracker, parentHash, currentPath, actualEvent)

			if diffEvents.Events == nil {
				diffEvents.Events = &summary.Events{}
			}

			for _, result := range results {
				if result != nil {
					diffEvents.Events.Egress = append(diffEvents.Events.Egress, result)
				}
			}
		}

		if len(latest.Events.Bind) > 0 && len(baseline.Events.Bind) > 0 {
			currentPath := parentPath + "/events/bind"
			actualEvent := "bind"
			results := compareNetworkEvents(latest.Events.Bind, baseline.Events.Bind, tracker, parentHash, currentPath, actualEvent)

			if diffEvents.Events == nil {
				diffEvents.Events = &summary.Events{}
			}

			for _, result := range results {
				if result != nil {
					diffEvents.Events.Bind = append(diffEvents.Events.Bind, result)
				}
			}
		}
	}

	return diffEvents
}

// compareFileEvents compares all the file/process events
func compareFileEvents(latest, baseline []*dev2summary.ProcessFileEvent, tracker *Graph, parentHash, parentPath, actualEvent string) []*dev2summary.ProcessFileEvent {
	var diffEvents []*dev2summary.ProcessFileEvent

	minLength := min(len(latest), len(baseline))
	for i := 0; i < minLength; i++ {
		fileDelta(latest[i], baseline[i], tracker, parentHash, parentPath, actualEvent)
	}

	// In case there are extra events to be handled we need to add/remove them to the graph
	for i := minLength; i < len(latest); i++ {
		if latest[i] != nil {
			addNewNodeFP(generateHash(parentPath, "Insert", latest[i].Source), "Insert", latest[i], tracker, parentPath, actualEvent, parentHash)
			diffEvents = append(diffEvents, latest[i])
		}
	}

	for i := minLength; i < len(baseline); i++ {
		if baseline[i] != nil {
			addNewNodeFP(generateHash(parentPath, "Remove", baseline[i].Source), "Remove", baseline[i], tracker, parentPath, actualEvent, parentHash)
			diffEvents = append(diffEvents, baseline[i])
		}
	}

	return diffEvents
}

// compareNetworkEvents compares all the network events
func compareNetworkEvents(latest, baseline []*dev2summary.NetworkEvent, tracker *Graph, parentHash, parentPath, actualEvent string) []*dev2summary.NetworkEvent {
	var diffEvents []*dev2summary.NetworkEvent

	minLength := min(len(latest), len(baseline))
	for i := 0; i < minLength; i++ {
		networkDelta(latest[i], baseline[i], tracker, parentHash, parentPath, actualEvent)
	}

	// In case there are extra events to be handled we need to add/remove them to the graph
	for i := minLength; i < len(latest); i++ {
		if latest[i] != nil {
			addNewNodeNetwork(generateHash(parentPath, "Insert", latest[i].Ip), "Insert", latest[i], tracker, parentPath, actualEvent, parentHash)
			diffEvents = append(diffEvents, latest[i])
		}
	}

	for i := minLength; i < len(baseline); i++ {
		if baseline[i] != nil {
			addNewNodeNetwork(generateHash(parentPath, "Remove", baseline[i].Ip), "Remove", baseline[i], tracker, parentPath, actualEvent, parentHash)
		}
	}

	return diffEvents
}

// fileDelta compares file/process events at field level
func fileDelta(latest, baseline *dev2summary.ProcessFileEvent, tracker *Graph, parentHash string, parentPath string, actualEvent string) *dev2summary.ProcessFileEvent {
	fields := []string{"source", "destination"}

	for _, field := range fields {
		latestValue := getFileFieldValue(latest, field)
		baselineValue := getFileFieldValue(baseline, field)

		compareFields(latestValue, baselineValue, "file-process-event", tracker, parentPath, field, parentHash, actualEvent, latest, baseline)
	}

	return latest
}

// networkDelta compares network events at field level
func networkDelta(latest, baseline *dev2summary.NetworkEvent, tracker *Graph, parentHash string, parentPath string, actualEvent string) *dev2summary.NetworkEvent {
	fields := []string{"ip", "port", "protocol", "peerDomainName", "command"}

	for _, field := range fields {
		latestValue := getNetworkFieldValue(latest, field)
		baselineValue := getNetworkFieldValue(baseline, field)

		compareFields(latestValue, baselineValue, "network-event", tracker, parentPath, field, parentHash, actualEvent, latest, baseline)
	}

	return latest
}

// compareFields compares the values of the fields, it calls Myres diff algo, then stores all the changes in the track graph
func compareFields(latestValue, baselineValue, eventType string, tracker *Graph, parentPath string, field string, parentHash string, granularEvent string, latestEvent, baselineEvent interface{}) {
	if latestValue == baselineValue {
		return
	}

	var editActions []EditAction

	if latestValue != "" && baselineValue != "" {
		editActions = myersDiff([]string{baselineValue}, []string{latestValue})
	} else if latestValue == "" {
		editActions = []EditAction{Remove{baselineValue}}
	} else if baselineValue == "" {
		editActions = []EditAction{Insert{latestValue}}
	}

	eventInfo := make(map[K]V)
	eventInfo[K(eventType)] = V(field)

	for _, action := range editActions {
		actionType := getActionTypeIdentifier(action)
		hash := generateHash(parentHash, actionType, latestValue)

		switch a := action.(type) {
		case Insert:
			newNode := &Node{
				Type:   eventType,
				Hash:   hash,
				Path:   parentPath + "/" + field,
				Level:  4,
				Change: ChangeType{Insert: []string{a.line}, Event: field, GranularEvent: granularEvent, Canceled: false},
			}
			if eventType == "file-process-event" {
				newNode.FileProcessData = latestEvent.(*dev2summary.ProcessFileEvent)
			} else if eventType == "network-event" {
				newNode.NetworkData = latestEvent.(*dev2summary.NetworkEvent)
			}
			tracker.AddNode(newNode, parentHash)
		case Remove:
			newNode := &Node{
				Type:   eventType,
				Hash:   hash,
				Path:   parentPath + "/" + field,
				Level:  4,
				Change: ChangeType{Remove: []string{a.line}, Event: field, GranularEvent: granularEvent, Canceled: false},
			}
			if eventType == "file-process-event" {
				newNode.FileProcessData = baselineEvent.(*dev2summary.ProcessFileEvent)
			} else if eventType == "network-event" {
				newNode.NetworkData = baselineEvent.(*dev2summary.NetworkEvent)
			}
			tracker.AddNode(newNode, parentHash)
		}
	}
}

// Helper to get a string identifier for the action type
func getActionTypeIdentifier(action EditAction) string {
	switch action.(type) {
	case Insert:
		return "Insert"
	case Remove:
		return "Remove"
	default:
		return "Unknown"
	}
}

// Helper to get the value of a field from the file/process event
func getFileFieldValue(event *dev2summary.ProcessFileEvent, field string) string {
	switch field {
	case "pod":
		return event.Pod
	case "source":
		return event.Source
	case "destination":
		return event.Destination
	case "count":
		return strconv.FormatInt(event.Count, 10)
	case "updatedTime":
		return strconv.FormatInt(event.UpdatedTime, 10)
	case "containerImage":
		return event.Container.Image
	case "containerName":
		return event.Container.Name
	}

	return ""
}

// Helper to get the value of a field from the network event
func getNetworkFieldValue(event *dev2summary.NetworkEvent, field string) string {
	switch field {
	case "pod":
		return event.Pod
	case "type":
		return event.Type
	case "command":
		return event.Command
	case "ip":
		return event.Ip
	case "port":
		return strconv.Itoa(int(event.Port))
	case "protocol":
		return event.Protocol
	case "peerDomainName":
		return event.PeerDomainName
	case "count":
		return strconv.FormatInt(event.Count, 10)
	case "updatedTime":
		return strconv.FormatInt(event.UpdatedTime, 10)
	default:
		return ""
	}
}

// Helper to generate a hash for a node
func generateHash(path, changeType, content string) string {
	data := path + ":" + changeType + ":" + content
	hash, err := summary.ComputeHash([]byte(data))
	if err != nil {
		return ""
	}

	return hash
}

// Helper to get the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Helper to add a new node to the graph to track changes that are not compared for File/Process
func addNewNodeFP(hash, changeType string, event *dev2summary.ProcessFileEvent, tracker *Graph, parentPath, actualEvent, parentHash string) {
	fields := map[string]string{
		"source":      event.Source,
		"destination": event.Destination,
	}

	for field, value := range fields {
		if value == "" {
			continue
		}

		fieldChangeInfo := ChangeType{
			Event:         field,
			GranularEvent: actualEvent,
			Canceled:      false,
		}
		if changeType == "Insert" {
			fieldChangeInfo.Insert = []string{value}
		} else if changeType == "Remove" {
			fieldChangeInfo.Remove = []string{value}
		}

		fieldHash := generateHash(parentPath, changeType, value)

		tracker.AddNode(&Node{
			Type:            "file-process-event",
			Hash:            fieldHash,
			Path:            parentPath + "/" + field,
			Level:           4,
			FileProcessData: event,
			Change:          fieldChangeInfo,
		}, parentHash)
	}
}

// Helper to add a new node to the graph to track changes that are not compared for Network
func addNewNodeNetwork(hash, changeType string, event *dev2summary.NetworkEvent, tracker *Graph, parentPath, actualEvent, parentHash string) {
	fields := []string{"ip", "port", "protocol", "peerDomainName", "command"}

	for _, field := range fields {
		var fieldValue string
		switch field {
		case "ip":
			fieldValue = event.Ip
		case "port":
			fieldValue = strconv.Itoa(int(event.Port))
		case "protocol":
			fieldValue = event.Protocol
		case "peerDomainName":
			fieldValue = event.PeerDomainName
		case "command":
			fieldValue = event.Command
		}

		if fieldValue == "" {
			continue
		}

		fieldChangeInfo := ChangeType{
			Event:         field,
			GranularEvent: actualEvent,
			Canceled:      false,
		}

		if changeType == "Insert" {
			fieldChangeInfo.Insert = []string{fieldValue}
		} else if changeType == "Remove" {
			fieldChangeInfo.Remove = []string{fieldValue}
		}

		fieldHash := generateHash(parentPath, changeType, fieldValue)

		tracker.AddNode(&Node{
			Type:        "network-event",
			Hash:        fieldHash,
			Path:        parentPath + "/" + field,
			Level:       4,
			NetworkData: event,
			Change:      fieldChangeInfo,
		}, parentHash)
	}
}
