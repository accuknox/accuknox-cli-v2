package report

import (
	"fmt"
	"regexp"

	"github.com/accuknox/accuknox-cli-v2/pkg/summary"
)

// FilterGraph filters the graph with given option and cancels the node
// if its not to be included
func (g *Graph) FilterGraph(rootHash string, opts *summary.Options) {
	dfsResult := g.DepthFirstSearch(rootHash)
	level4NodesByParent := g.groupLevel4Nodes(dfsResult)

	for _, nodes := range level4NodesByParent {
		for _, node := range nodes {
			if len(opts.WorkloadsRegex) > 0 || len(opts.Workloads) > 0 {
				matchesWorkload(node, opts.Workloads, opts.WorkloadsRegex)
			}

			if len(opts.SourceRegex) > 0 || len(opts.Source) > 0 {
				matchesSource(node, opts.Source, opts.SourceRegex)
			}

			if len(opts.DestinationRegex) > 0 || len(opts.Destination) > 0 {
				matchesDestination(node, opts.Destination, opts.DestinationRegex)
			}

			if len(opts.CommandRegex) > 0 || len(opts.Command) > 0 {
				isCommandIgnored(node, opts.Command, opts.CommandRegex)
			}

			if len(opts.IgnorePathsRegex) > 0 || len(opts.IgnorePath) > 0 {
				isPathIgnored(node, opts.IgnorePath, opts.IgnorePathsRegex)
			}
		}
	}
}

// matchesWorkload checks if the node's workload matches any of the provided workload regex patterns or strings.
func matchesWorkload(node *Node, workloads []string, regexes []*regexp.Regexp) {
	workload := getWorkloadFromPath(node.Path)

	if !matchesStringOrRegex(workload, workloads, regexes) {
		node.Change.Canceled = true
	}
}

// matchesStringOrRegex checks if the input string matches any of the provided strings or regex patterns.
func matchesStringOrRegex(input string, strings []string, regexes []*regexp.Regexp) bool {
	for _, str := range strings {
		if input == str {
			return true
		}
	}

	for _, r := range regexes {
		if r.MatchString(input) {
			return true
		}
	}

	return false
}

// isPathIgnored checks if the node's source or destination path should be ignored based on the provided regex patterns.
func isPathIgnored(node *Node, ignorePaths []string, regexes []*regexp.Regexp) {
	if node.FileProcessData == nil {
		return
	}

	source := node.FileProcessData.Source
	if source != "" && matchesStringOrRegex(source, ignorePaths, regexes) {
		node.Change.Canceled = true
	}

	destination := node.FileProcessData.Destination
	if destination != "" && matchesStringOrRegex(destination, ignorePaths, regexes) {
		node.Change.Canceled = true
	}
}

// matchesSource checks if the node's source matches any of the provided source strings or regex patterns.
func matchesSource(node *Node, sources []string, regexes []*regexp.Regexp) {
	if node.FileProcessData == nil {
		return
	}
	source := node.FileProcessData.Source
	if source != "" && !matchesStringOrRegex(source, sources, regexes) {
		node.Change.Canceled = true
	}
}

// matchesDestination checks if the node's destination matches any of the provided destination strings or regex patterns.
func matchesDestination(node *Node, destinations []string, regexes []*regexp.Regexp) {
	if node.FileProcessData == nil {
		return
	}
	destination := node.FileProcessData.Destination
	if destination != "" && !matchesStringOrRegex(destination, destinations, regexes) {
		node.Change.Canceled = true
	}
}

// isCommandIgnored checks if the node's command matches any of the provided command strings or regex patterns.
func isCommandIgnored(node *Node, commands []string, regexes []*regexp.Regexp) {
	if node.NetworkData == nil {
		return
	}
	command := node.NetworkData.Command
	if command != "" && matchesStringOrRegex(command, commands, regexes) {
		node.Change.Canceled = true
	}
}

// getWorkloadFromPath extracts the workload from the node's path.
func getWorkloadFromPath(path string) string {
	parsedPath := parsePathInfo(path)

	resourceType := parsedPath["resource-type"]
	resourceName := parsedPath["resource-name"]
	if resourceType == "" || resourceName == "" {
		return ""
	}

	return fmt.Sprintf("%s/%s", resourceType, resourceName)
}
