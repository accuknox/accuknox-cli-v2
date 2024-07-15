package scan

import (
	"encoding/json"
	"fmt"
	kaproto "github.com/kubearmor/KubeArmor/protobuf"
	"os"
	"sync"
)

type ProcessNode struct {
	ProcessName string         `json:"processName"`
	PID         int32          `json:"pid"`
	Command     string         `json:"command"`
	Children    []*ProcessNode `json:"children,omitempty"`
	PPID        int32          `json:"ppid"` // Parent PID, not for JSON output
}

type ProcessForest struct {
	Roots []*ProcessNode         `json:"roots"`
	Nodes map[int32]*ProcessNode `json:"-"`
	mu    sync.RWMutex
}

func NewProcessForest() *ProcessForest {
	return &ProcessForest{
		Roots: make([]*ProcessNode, 0),
		Nodes: make(map[int32]*ProcessNode),
	}
}

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
	node.ProcessName = log.ProcessName
	node.Command = log.Resource
	node.PPID = log.HostPPID
}

func (pf *ProcessForest) BuildFromSegregatedData(data map[int32]kaproto.Log) {
	fmt.Printf("Debug: Starting to build from segregated data. Data size: %d\n", len(data))
	for _, log := range data {
		pf.AddProcess(&log)
	}
	pf.constructTree()
}

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

func (pf *ProcessForest) isRoot(node *ProcessNode) bool {
	for _, root := range pf.Roots {
		if root.PID == node.PID {
			return true
		}
	}
	return false
}

func (pf *ProcessForest) SaveProcessForestJSON(filename string) error {
	pf.mu.RLock()
	defer pf.mu.RUnlock()

	jsonData, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling process forest to JSON: %v", err)
	}

	err = os.WriteFile(filename, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("error writing process forest to file: %v", err)
	}
	return nil
}
