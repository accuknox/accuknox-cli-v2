package report

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
)

func (g Graph) printTable(rootHash string) error {
	currentTime := time.Now().UTC().Format("2006-01-02 15:04:05 UTC")
	fmt.Printf("Report [%s]\n\n", currentTime)

	dfsResult := g.DepthFirstSearch(rootHash)
	level4NodesByParent := g.groupLevel4Nodes(dfsResult)

	processFileData := [][]string{}
	networkData := [][]string{}
	bindData := [][]string{}

	// addedEvents := make(map[string]struct{})

	for _, nodes := range level4NodesByParent {
		shouldPrintInfoCard := false

		for _, node := range nodes {
			if node.Change.Canceled {
				continue
			}
			if len(node.Change.Insert) > 0 || len(node.Change.Remove) > 0 {
				shouldPrintInfoCard = true
				break
			}
		}

		if shouldPrintInfoCard {
			commonInfo := createCommonInfoCardTable(nodes[0])
			fmt.Println(commonInfo)
		}

		for _, node := range nodes {
			if node.Change.Canceled {
				continue
			}

			switch node.Type {
			case "file-process-event":
				processFileData = append(processFileData, processFileEvent(node))
			case "network-event":
				networkData = append(networkData, networkEvent(node))
			case "bind-event":
				bindData = append(bindData, bindEvent(node))
			}
		}

		if len(processFileData) > 0 {
			fmt.Println("Process/File Summary:")
			printTable([]string{"Source Path", "Destination Path", "Status"}, processFileData)
		}

		if len(networkData) > 0 {
			fmt.Println("Network Summary:")
			printTable([]string{"Protocol", "Command", "POD/SVC/IP", "Port", "Namespace", "Type"}, networkData)
		}

		if len(bindData) > 0 {
			fmt.Println("Bind Summary:")
			printTable([]string{"Protocol", "Command", "Bind Port", "Bind Address"}, bindData)
		}
	}

	return nil
}

func printTable(header []string, data [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetBorder(true)
	table.AppendBulk(data)
	table.Render()
}

func processFileEvent(node *Node) []string {
	if node.FileProcessData == nil {
		return nil
	}

	source := node.FileProcessData.Source
	if source == "" {
		source = "N/A"
	}

	destination := node.FileProcessData.Destination
	if destination == "" {
		destination = "N/A"
	}

	sourceDelta := matchAndDeltaTable(source, node.Change.Insert, node.Change.Remove)
	destinationDelta := matchAndDeltaTable(destination, node.Change.Insert, node.Change.Remove)

	return []string{sourceDelta, destinationDelta, "Allow"}
}

func networkEvent(node *Node) []string {
	if node.NetworkData == nil {
		return nil
	}

	command := node.NetworkData.Command
	if command == "" {
		command = "N/A"
	}

	podSvcIP := fmt.Sprint(node.NetworkData.Ip)
	if podSvcIP == "" {
		podSvcIP = "N/A"
	}

	port := fmt.Sprint(node.NetworkData.Port)
	if node.NetworkData.Port == 0 {
		port = "N/A"
	}

	protocol := node.NetworkData.Protocol
	if protocol == "" {
		protocol = "N/A"
	}

	var namespace string
	var resourceType string

	// This can be improved I am not sure if this is the best way to do this.
	if node.NetworkData.ControlInfo != nil && node.NetworkData.ControlInfo.Resource.Namespace != "" && node.NetworkData.ControlInfo.Resource.Type != "" {
		namespace = node.NetworkData.ControlInfo.Resource.Namespace
		resourceType = node.NetworkData.ControlInfo.Resource.Type
	} else {
		namespace = "N/A"
		resourceType = "N/A"
	}

	commandDelta := matchAndDeltaTable(command, node.Change.Insert, node.Change.Remove)
	podSvcIPDelta := matchAndDeltaTable(podSvcIP, node.Change.Insert, node.Change.Remove)
	portDelta := matchAndDeltaTable(port, node.Change.Insert, node.Change.Remove)
	protocolDelta := matchAndDeltaTable(protocol, node.Change.Insert, node.Change.Remove)

	return []string{protocolDelta, commandDelta, podSvcIPDelta, portDelta, namespace, resourceType}
}

func bindEvent(node *Node) []string {
	if node.NetworkData == nil {
		return nil
	}

	command := node.NetworkData.Command
	if command == "" {
		command = "N/A"
	}

	bindPort := fmt.Sprint(node.NetworkData.Port)
	if node.NetworkData.Port == 0 {
		bindPort = "N/A"
	}

	protocol := node.NetworkData.Protocol
	if protocol == "" {
		protocol = "N/A"
	}

	commandDelta := matchAndDeltaTable(command, node.Change.Insert, node.Change.Remove)
	bindPortDelta := matchAndDeltaTable(bindPort, node.Change.Insert, node.Change.Remove)
	protocolDelta := matchAndDeltaTable(protocol, node.Change.Insert, node.Change.Remove)

	return []string{protocolDelta, commandDelta, bindPortDelta}
}

func createCommonInfoCardTable(node *Node) string {
	parsedPathInfo := parsePathInfo(node.Path)

	var infoCard strings.Builder
	infoCard.WriteString("\nWorkload Information \n")
	commonKeys := []string{"cluster", "namespace"}

	for _, key := range commonKeys {
		if value, ok := parsedPathInfo[key]; ok {
			infoCard.WriteString(fmt.Sprintf("%s: %s  \n", cap(key), value))
		} else {
			infoCard.WriteString(fmt.Sprintf("%s: %s  \n", cap(key), "N/A"))
		}
	}

	resourceType := parsedPathInfo["resource-type"]
	resourceName := parsedPathInfo["resource-name"]
	if resourceType == "" {
		resourceType = "N/A"
	}
	if resourceName == "" {
		resourceName = "N/A"
	}
	infoCard.WriteString(fmt.Sprintf("Type/Name: %s/%s  \n", resourceType, resourceName))

	return infoCard.String()
}

func matchAndDeltaTable(field string, insertions, removals []string) string {
	hasInsertion := false
	for _, insertion := range insertions {
		if strings.Contains(field, insertion) {
			hasInsertion = true
			break
		}
	}

	hasDeletion := false
	for _, deletion := range removals {
		if strings.Contains(field, deletion) {
			hasDeletion = true
			break
		}
	}

	if hasInsertion || hasDeletion {
		return createDeltaStringTable(insertions, removals)
	}

	return field
}

func createDeltaStringTable(insertions, removals []string) string {
	deltaItems := make([]string, 0, len(insertions)+len(removals))

	red := "\033[31m"   // Red for removals
	green := "\033[32m" // Green for insertions
	reset := "\033[0m"  // Reset to default color

	for _, removal := range removals {
		formattedRemoval := fmt.Sprintf("%s-%s%s", red, removal, reset)
		deltaItems = append(deltaItems, formattedRemoval)
	}
	for _, insertion := range insertions {
		formattedInsertion := fmt.Sprintf("%s+%s%s", green, insertion, reset)
		deltaItems = append(deltaItems, formattedInsertion)
	}

	return strings.Join(deltaItems, " ")
}
