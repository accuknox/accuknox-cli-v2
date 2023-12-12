package report

import (
	"fmt"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	dev2summary "github.com/accuknox/dev2/api/grpc/v2/summary"
	"github.com/clarketm/json"
)

type K any // Key type K any
type V any // Value type V any

// Node is a node in the JSON tree
type Node struct {
	Type            string                        `json:"type"`            // Type of object (workload, cluster, ns, etc)
	Hash            string                        `json:"-"`               // Hash of the object
	Path            string                        `json:"path"`            // Path to reach current node
	Level           int                           `json:"level"`           // Level of current node in terms of depth (4 is leaf where the data is)
	ParentHash      string                        `json:"parent"`          // Parent hash is basically an edge that connects parent to child
	NetworkData     *dev2summary.NetworkEvent     `json:"networkdata"`     // dev2 api network event
	FileProcessData *dev2summary.ProcessFileEvent `json:"fileprocessdata"` // dev2 api file process event
	Change          ChangeType                    `json:"changetype"`      // Change type
	Children        []*Node                       `json:"children"`        // Children of the node
}

// ChangeType is the type of change that occurred
type ChangeType struct {
	Keep          []string `json:"keep"`           // Keep is the list of values that were not changed
	Insert        []string `json:"insert"`         // Insert is the list of values that were inserted
	Remove        []string `json:"remove"`         // Remove is the list of values that were removed
	GranularEvent string   `json:"granular_event"` // GranularEvent can be one of the following: file, process, ingress, egress, bind
	Event         string   `json:"event"`          // Event is the event that occurred, network or file-process
	Canceled      bool     `json:"canceled"`       // Algebraic cancellation of changes
}

// Graph is a JSON tree tracker, it tracks the JSON and keeps all the
// change related information by spawning/crawling over JSON structure.
type Graph struct {
	Nodes map[string]*Node `json:"nodes"`
}

// NewGraph returns a new instance of new graph
func NewGraph() *Graph {
	return &Graph{
		Nodes: make(map[string]*Node),
	}
}

// AddNode adds a node to the graph
func (g *Graph) AddNode(node *Node, parentHash string) {
	if node == nil {
		return
	}
	node.ParentHash = parentHash
	g.Nodes[node.Hash] = node

	if parentHash != "" {
		parentNode, exists := g.Nodes[parentHash]
		if !exists {
			return
		}
		parentNode.Children = append(parentNode.Children, node)
	}
}

// GetNode returns a node from the graph
func (g *Graph) GetNode(hash string) *Node {
	if node, exists := g.Nodes[hash]; exists {
		return node
	}

	return nil
}

// CancelOutChanges cancels out changes that are both insertions and removals
func (g *Graph) cancelOutChanges(rootHash string) {
	dfsResult := g.DepthFirstSearch(rootHash)
	insertions := make(map[string]*Node)
	removals := make(map[string]*Node)

	for _, node := range dfsResult {
		if node.Level == 4 {
			if len(node.Change.Insert) > 0 {
				for _, val := range node.Change.Insert {
					insertions[val] = node
				}
			}

			if len(node.Change.Remove) > 0 {
				for _, val := range node.Change.Remove {
					removals[val] = node
				}
			}
		}
	}

	for val, insNode := range insertions {
		if remNode, exists := removals[val]; exists {
			insNode.Change.Canceled = true
			remNode.Change.Canceled = true
		}
	}
}

// DepthFirstSearch performs a DFS on the graph and returns the nodes in the order of traversal
func (g Graph) DepthFirstSearch(startHash string) []*Node {
	rootNode, exists := g.Nodes[startHash]
	if !exists {
		return []*Node{}
	}

	var result []*Node
	visited := make(map[string]bool)
	var dfs func(node *Node)

	dfs = func(node *Node) {
		if node == nil || visited[node.Hash] {
			return
		}

		visited[node.Hash] = true
		result = append(result, node)
		for _, childHash := range node.Children {
			childNode, exists := g.Nodes[childHash.Hash]
			if exists {
				dfs(childNode)
			}
		}
	}

	dfs(rootNode)
	return result
}

// groupLevel4Nodes groups all level 4 nodes under the same parent
func (g Graph) groupLevel4Nodes(nodes []*Node) map[string][]*Node {
	groupedNodes := make(map[string][]*Node)
	for _, node := range nodes {
		if node.Level == 4 {
			groupedNodes[node.ParentHash] = append(groupedNodes[node.ParentHash], node)
		}
	}
	return groupedNodes
}

// PrintDFSTraversal prints the DFS traversal of the graph, this is for debugging purposes
func (g Graph) printDFSTraversal(startHash string) {
	rootNode, exists := g.Nodes[startHash]
	if !exists {
		fmt.Println("Root node not found")
		return
	}

	var dfs func(node *Node, indent int)
	dfs = func(node *Node, indent int) {
		if node == nil {
			return
		}

		indentStr := strings.Repeat(" ", indent)
		fmt.Printf("%sNode Type: %s\n", indentStr, node.Type)
		fmt.Printf("%sHash: %s\n", indentStr, node.Hash)
		fmt.Printf("%sPath: %s\n", indentStr, node.Path)
		fmt.Printf("%sLevel: %d\n", indentStr, node.Level)
		fmt.Printf("%sParentHash: %s\n", indentStr, node.ParentHash)
		fmt.Printf("%sChange: %+v\n", indentStr, node.Change)
		fmt.Println()

		for _, child := range node.Children {
			dfs(child, indent+2)
		}
	}

	dfs(rootNode, 0)
}

// WriteLevel4NodesToJSONFile writes all level 4 nodes to a JSON file, this is for debugging purposes
func (g Graph) writeLevel4NodesToJSONFile(outputFilename, rootHash string) error {
	file, err := common.CleanAndCreate(outputFilename)
	if err != nil {
		return fmt.Errorf("error creating file: %v", err)
	}
	defer file.Close()

	dfsResult := g.DepthFirstSearch(rootHash)
	level4NodesByParent := g.groupLevel4Nodes(dfsResult)

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	for _, nodes := range level4NodesByParent {
		for _, node := range nodes {
			if node.Level != 4 {
				continue
			}

			if err := encoder.Encode(node); err != nil {
				fmt.Printf("Error marshaling node to JSON: %s\n", err)
			}
		}
	}

	return nil
}

func (g Graph) WriteFilteredNodesTOJSONFile(nodes []*Node) error {
	file, err := common.CleanAndCreate("knoxctl_out/reports/filtered_nodes.json")
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	for _, nodes := range nodes {
		if err := encoder.Encode(nodes); err != nil {
			fmt.Printf("Error marshaling node to JSON: %s\n", err)
		}
	}

	return nil
}
