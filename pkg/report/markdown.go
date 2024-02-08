package report

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
)

// MarkdownPR creates a markdown report for pull requests. MarkdownPR is a
// reciever function for the Graph struct, the reason for this is to keep
// Graph struct clean/consistent, since essentially we are going to print the graph.
func (g Graph) markdownPR(filename, rootHash string) error {
	var isSomethingThere bool

	file, err := common.CleanAndCreate(filename)
	fmt.Printf("Writing report markdown file to: %s\n", filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	currentTime := time.Now().UTC().Format("2006-01-02 15:04:05 UTC")
	_, err = writer.WriteString(fmt.Sprintf("# Report [%s]\n\n", currentTime))
	if err != nil {
		return err
	}

	dfsResult := g.DepthFirstSearch(rootHash)
	level4NodesByParent := g.groupLevel4Nodes(dfsResult)

	processFileHeader := "<table><tr><th>Source Path</th><th>Destination Path</th><th>Status</th></tr>"
	networkHeader := "<table><tr><th>Protocol</th><th>Command</th><th>POD/SVC/IP</th><th>Port</th><th>Namespace</th><th>Type</th></tr>"
	bindHeader := "<table><tr><th>Protocol</th><th>Command</th><th>Bind Port</th><th>Bind Address</th></tr>"

	addedEvents := make(map[string]struct{})

	for _, nodes := range level4NodesByParent {
		shouldPrintInfoCard := false
		localIsSomethingThere := false

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
			commonInfo := createCommonInfoCard(nodes[0])
			_, err := writer.WriteString(commonInfo + "\n")
			if err != nil {
				return err
			}
		}

		processFileTable := new(strings.Builder)
		ingressTable := new(strings.Builder)
		egressTable := new(strings.Builder)
		bindTable := new(strings.Builder)

		processFileTable.WriteString(processFileHeader)
		ingressTable.WriteString(networkHeader)
		egressTable.WriteString(networkHeader)
		bindTable.WriteString(bindHeader)

		for _, node := range nodes {
			if node.Change.Canceled {
				continue
			}

			switch node.Type {
			case "file-process-event":
				populateProcessFileTable(processFileTable, node, addedEvents)
			case "network-event":
				switch node.Change.GranularEvent {
				case "ingress":
					populateNetworkTable(ingressTable, node, "ingress", addedEvents)
				case "egress":
					populateNetworkTable(egressTable, node, "egress", addedEvents)
				case "bind":
					populateBindTable(bindTable, node, "bind", addedEvents)
				}
			}
		}

		if processFileTable.Len() > len(processFileHeader) {
			isSomethingThere = true
			localIsSomethingThere = true

			if err := writeAndCheck(writer, "<details>\n<summary>Process/File Summary</summary>\n\n"); err != nil {
				return err
			}
			if err := writeAndCheck(writer, processFileTable.String()+"</table>\n\n"); err != nil {
				return err
			}
			if err := writeAndCheck(writer, "</details>\n"); err != nil {
				return err
			}
		}

		if ingressTable.Len() > len(networkHeader) {
			isSomethingThere = true
			localIsSomethingThere = true

			if err := writeAndCheck(writer, "<details>\n<summary>Ingress Connections</summary>\n\n"); err != nil {
				return err
			}
			if err := writeAndCheck(writer, ingressTable.String()+"</table>\n\n"); err != nil {
				return err
			}
			if err := writeAndCheck(writer, "</details>\n"); err != nil {
				return err
			}
		}

		if egressTable.Len() > len(networkHeader) {
			isSomethingThere = true
			localIsSomethingThere = true

			if err := writeAndCheck(writer, "<details>\n<summary>Egress Connections</summary>\n\n"); err != nil {
				return err
			}
			if err := writeAndCheck(writer, egressTable.String()+"</table>\n\n"); err != nil {
				return err
			}
			if err := writeAndCheck(writer, "</details>\n"); err != nil {
				return err
			}
		}

		if bindTable.Len() > len(bindHeader) {
			isSomethingThere = true
			localIsSomethingThere = true

			if err := writeAndCheck(writer, "<details>\n<summary>Bind Connections</summary>\n\n"); err != nil {
				return err
			}
			if err := writeAndCheck(writer, bindTable.String()+"</table>\n\n"); err != nil {
				return err
			}
			if err := writeAndCheck(writer, "</details>\n"); err != nil {
				return err
			}
		}

		if localIsSomethingThere {
			if err := writeAndCheck(writer, "<hr>\n"); err != nil {
				return err
			}
		}
	}

	if !isSomethingThere {
		if err := writeAndCheck(writer, "No changes detected.\n"); err != nil {
			return err
		}
	}

	return writer.Flush()
}

func populateProcessFileTable(table *strings.Builder, node *Node, addedEvents map[string]struct{}) {
	if node.FileProcessData == nil {
		return
	}

	source := node.FileProcessData.Source
	if source == "" {
		source = "N/A"
	}

	destination := node.FileProcessData.Destination
	if destination == "" {
		destination = "N/A"
	}

	sourceDelta := matchAndDelta(node.FileProcessData.Source, node.Change.Insert, node.Change.Remove)
	destinationDelta := matchAndDelta(node.FileProcessData.Destination, node.Change.Insert, node.Change.Remove)

	changeKey := fmt.Sprintf("process|%s|%s", source, destination)

	if _, exists := addedEvents[changeKey]; !exists {
		addedEvents[changeKey] = struct{}{}
		table.WriteString(fmt.Sprintf("<tr><td>\n%s\n</td><td>\n%s\n</td><td>Allow</td></tr>", sourceDelta, destinationDelta))
	}
}

func populateNetworkTable(table *strings.Builder, node *Node, tableType string, addedEvents map[string]struct{}) {
	if node.NetworkData == nil {
		return
	}

	protocol := node.NetworkData.Protocol
	if protocol == "" {
		protocol = "N/A"
	}

	command := node.NetworkData.Command
	if command == "" {
		command = "N/A"
	}

	ip := node.NetworkData.Ip
	if ip == "" {
		ip = "N/A"
	}

	port := fmt.Sprint(node.NetworkData.Port)
	if node.NetworkData.Port == 0 {
		port = "N/A"
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

	protocol = matchAndDelta(protocol, node.Change.Insert, node.Change.Remove)
	command = matchAndDelta(command, node.Change.Insert, node.Change.Remove)
	ip = matchAndDelta(ip, node.Change.Insert, node.Change.Remove)
	port = matchAndDelta(port, node.Change.Insert, node.Change.Remove)

	delta := createDeltaString(node.Change.Insert, node.Change.Remove)

	changeKey := fmt.Sprintf("network|%s|%s|%s|%s|%s", protocol, command, ip, port, delta)

	if _, exists := addedEvents[changeKey]; !exists {
		addedEvents[changeKey] = struct{}{}
		table.WriteString(fmt.Sprintf("<tr><td>\n%s\n</td><td>\n%s\n</td><td>\n%s\n</td><td>\n%s\n</td><td>\n<code>%s</code>\n</td><td>\n<code>%s</code>\n</td></tr>\n", protocol, command, ip, port, namespace, resourceType))
	}
}

func populateBindTable(table *strings.Builder, node *Node, tableType string, addedEvents map[string]struct{}) {
	if node.NetworkData == nil {
		return
	}

	protocol := node.NetworkData.Protocol
	if protocol == "" {
		protocol = "N/A"
	}

	command := node.NetworkData.Command
	if command == "" {
		command = "N/A"
	}

	bindPort := fmt.Sprint(node.NetworkData.Port)
	if node.NetworkData.Port == 0 {
		bindPort = "N/A"
	}

	bindAddress := node.NetworkData.Ip
	if bindAddress == "" {
		bindAddress = "N/A"
	}

	protocol = matchAndDelta(protocol, node.Change.Insert, node.Change.Remove)
	command = matchAndDelta(command, node.Change.Insert, node.Change.Remove)
	bindPort = matchAndDelta(bindPort, node.Change.Insert, node.Change.Remove)
	bindAddress = matchAndDelta(bindAddress, node.Change.Insert, node.Change.Remove)

	delta := createDeltaString(node.Change.Insert, node.Change.Remove)

	changeKey := fmt.Sprintf("network|%s|%s|%s|%s|%s", protocol, command, bindPort, bindAddress, delta)

	if _, exists := addedEvents[changeKey]; !exists {
		addedEvents[changeKey] = struct{}{}
		table.WriteString(fmt.Sprintf("<tr><td>\n%s\n</td><td>\n%s\n</td><td>\n%s\n</td><td>\n%s\n</td></tr>", protocol, command, bindPort, bindAddress))
	}
}

func matchAndDelta(field string, insertions, removals []string) string {
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
		return createDeltaString(insertions, removals)
	}

	if !strings.Contains(field, "```diff") {
		return fmt.Sprintf("<code>%s</code>", field)
	}

	return field
}

// Helper function to format insertions and removals.
func createDeltaString(insertions, removals []string) string {
	deltaItems := make([]string, 0, len(insertions)+len(removals))
	for _, removal := range removals {
		formattedRemoval := fmt.Sprintf("- %s\t ", removal)
		deltaItems = append(deltaItems, fmt.Sprintf("\n```diff\n%s\n```", formattedRemoval))
	}
	for _, insertion := range insertions {
		formattedInsertion := fmt.Sprintf("+ %s\t ", insertion)
		deltaItems = append(deltaItems, fmt.Sprintf("\n```diff\n%s\n```", formattedInsertion))
	}
	return strings.Join(deltaItems, "\n")
}

// createCommonInfoCard generates a markdown formatted information card for common information.
func createCommonInfoCard(node *Node) string {
	parsedPathInfo := parsePathInfo(node.Path)

	var infoCard strings.Builder
	infoCard.WriteString("\n### Workload Information \n")
	commonKeys := []string{"cluster", "namespace"}

	for _, key := range commonKeys {
		if value, ok := parsedPathInfo[key]; ok {
			infoCard.WriteString(fmt.Sprintf(">**%s**: %s  \n", cap(key), value))
		} else {
			infoCard.WriteString(fmt.Sprintf(">**%s**: %s  \n", cap(key), "N/A"))
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
	infoCard.WriteString(fmt.Sprintf(">**Type/Name**: %s/%s  \n", resourceType, resourceName))

	return infoCard.String()
}

// cap capitalizes the first letter of a string.
func cap(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// parsePathInfo parses the path of a node and returns a map of key value pairs, this is
// used to extract info about the workload.
func parsePathInfo(path string) map[K]V {
	result := make(map[K]V)
	segments := strings.Split(path, "/")

	for i := 1; i < len(segments)-1; i += 2 {
		key := K(segments[i])
		if i+1 < len(segments) {
			value := V(segments[i+1])
			result[key] = value
		}
	}

	return result
}

func writeAndCheck(writer *bufio.Writer, text string) error {
	_, err := writer.WriteString(text)
	return err
}
