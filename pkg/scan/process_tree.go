package scan

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	kaproto "github.com/kubearmor/KubeArmor/protobuf"
)

// ProcessNode represents a node in Process Forest
type ProcessNode struct {
	// Name of the process
	ProcessName string `json:"processName"`

	// ProcessID
	PID int32 `json:"pid"`

	// Command execed by the proces
	Command string `json:"command"`

	// Map of children
	Children []*ProcessNode `json:"children,omitempty"`

	// ParentID
	PPID int32 `json:"ppid"`
}

// ProcessForest is the data structure that holds the information related to
// ProcessTrees, orphan nodes are considered are roots
type ProcessForest struct {
	// Roots
	Roots []*ProcessNode `json:"roots"`

	// Nodes of forest
	Nodes map[int32]*ProcessNode `json:"-"`

	// Lock
	mu sync.RWMutex
}

// NewProcessForest inits a new ProcessForest
func NewProcessForest() *ProcessForest {
	return &ProcessForest{
		Roots: make([]*ProcessNode, 0),
		Nodes: make(map[int32]*ProcessNode),
	}
}

// AddProcess adds a new node
func (pf *ProcessForest) AddProcess(log *kaproto.Log) {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	node, exists := pf.Nodes[log.HostPID]
	if !exists {
		node = &ProcessNode{
			PID:      log.HostPID,
			Children: make([]*ProcessNode, 0),
		}
		pf.Nodes[log.HostPID] = node
	}
	node.ProcessName = getActualProcessName(log.ProcessName)
	node.Command = simplifyCommand(log.Resource)
	node.PPID = log.HostPPID
}

// BuildFromSegregatedData will construct Forest from kubearmor logs
func (pf *ProcessForest) BuildFromSegregatedData(data []kaproto.Log) {
	for _, log := range data {
		pf.AddProcess(&log)
	}

	pf.constructTree()
}

// constructTree constructs a tree
func (pf *ProcessForest) constructTree() {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	childrenMap := make(map[int32][]*ProcessNode)
	for _, node := range pf.Nodes {
		if node.PPID != node.PID {
			childrenMap[node.PPID] = append(childrenMap[node.PPID], node)
		}
	}

	for _, node := range pf.Nodes {
		children, exists := childrenMap[node.PID]
		if exists {
			node.Children = children
		}

		if node.PPID == 0 || node.PPID == node.PID || pf.Nodes[node.PPID] == nil {
			if !pf.isRoot(node) {
				pf.Roots = append(pf.Roots, node)
			}
		}
	}
}

// isRoot checks whether the current node is a root
func (pf *ProcessForest) isRoot(node *ProcessNode) bool {
	for _, root := range pf.Roots {
		if root.PID == node.PID {
			return true
		}
	}
	return false
}

// GenerateMarkdownTree generates markdown
func (pf *ProcessForest) GenerateMarkdownTree() string {
	pf.mu.RLock()
	defer pf.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("```\n")
	for _, root := range pf.Roots {
		pf.writeNodeMarkdown(&sb, root, 0)
	}
	sb.WriteString("```\n")

	content := sb.String()
	return content
}

// writeNodeMarkdown writes markdown
func (pf *ProcessForest) writeNodeMarkdown(sb *strings.Builder, node *ProcessNode, depth int) {
	indent := strings.Repeat("    ", depth)
	prefix := ""
	if depth > 0 {
		prefix = "├── "
	}

	line := fmt.Sprintf("%s%s[%d] %s: %s\n", indent, prefix, node.PID, node.ProcessName, node.Command)
	sb.WriteString(line)

	for i, child := range node.Children {
		if i == len(node.Children)-1 && depth > 0 {
			pf.writeLastNodeMarkdown(sb, child, depth+1)
		} else {
			pf.writeNodeMarkdown(sb, child, depth+1)
		}
	}
}

// writeLastNodeMarkdown handles the last node
func (pf *ProcessForest) writeLastNodeMarkdown(sb *strings.Builder, node *ProcessNode, depth int) {
	indent := strings.Repeat("    ", depth-1) + "    "
	line := fmt.Sprintf("%s└── [%d] %s: %s\n", indent, node.PID, node.ProcessName, node.Command)
	sb.WriteString(line)

	for i, child := range node.Children {
		if i == len(node.Children)-1 {
			pf.writeLastNodeMarkdown(sb, child, depth+1)
		} else {
			pf.writeNodeMarkdown(sb, child, depth+1)
		}
	}
}

// SaveProcessForestJSON saves Process Forest in JSON, usually important for debugging
func (pf *ProcessForest) SaveProcessForestJSON(filename string) error {
	pf.mu.RLock()
	defer pf.mu.RUnlock()

	jsonData, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling process forest to JSON: %v", err)
	}

	err = common.CleanAndWrite(filename, jsonData)
	if err != nil {
		return fmt.Errorf("error writing process forest to file: %v", err)
	}

	return nil
}

func (pf *ProcessForest) SaveProcessForestMarkdown(filename string) error {
	markdownContent := pf.GenerateMarkdownTree()

	err := common.CleanAndWrite(filename, []byte(markdownContent))
	if err != nil {
		return fmt.Errorf("error writing process tree markdown to file: %v", err)
	}

	return nil
}
