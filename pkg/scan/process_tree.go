package scan

import (
	"encoding/json"
	"fmt"
	kaproto "github.com/kubearmor/KubeArmor/protobuf"
	"os"
	"sync"
)

type ProcessNode struct {
	ProcessName string                 `json:"processName"`
	PID         int32                  `json:"pid"`
	Command     string                 `json:"command"`
	Children    map[int32]*ProcessNode `json:"children,omitempty"`
}

type ProcessTree struct {
	Root map[int32]*ProcessNode `json:"root"`
	mu   sync.RWMutex
}

func NewProcessTree() *ProcessTree {
	return &ProcessTree{
		Root: make(map[int32]*ProcessNode),
	}
}

func (pt *ProcessTree) AddProcess(log *kaproto.Log) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	currentNode := &ProcessNode{
		ProcessName: log.ProcessName,
		PID:         log.HostPID,
		Command:     log.Resource,
		Children:    make(map[int32]*ProcessNode),
	}

	parentNode := pt.findOrCreateNode(log.HostPPID)

	if parentNode == nil {
		// This is a root process
		pt.Root[log.HostPID] = currentNode
	} else {
		// This is a child process
		parentNode.Children[log.HostPID] = currentNode
	}
}

func (pt *ProcessTree) findOrCreateNode(pid int32) *ProcessNode {
	if node, exists := pt.Root[pid]; exists {
		return node
	}

	for _, rootNode := range pt.Root {
		if node := pt.dfs(rootNode, pid); node != nil {
			return node
		}
	}

	// If the node doesn't exist, create it as a root node
	newNode := &ProcessNode{
		PID:      pid,
		Children: make(map[int32]*ProcessNode),
	}
	pt.Root[pid] = newNode
	return newNode
}

func (pt *ProcessTree) dfs(node *ProcessNode, targetPID int32) *ProcessNode {
	if node.PID == targetPID {
		return node
	}

	for _, child := range node.Children {
		if foundNode := pt.dfs(child, targetPID); foundNode != nil {
			return foundNode
		}
	}

	return nil
}

func (pt *ProcessTree) BuildFromSegregatedData(data map[int32]kaproto.Log) {
	for _, log := range data {
		pt.AddProcess(&log)
	}
}

func (pt *ProcessTree) SaveProcessTreeJSON(filename string) error {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	jsonData, err := json.MarshalIndent(pt.Root, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling process tree to JSON: %v", err)
	}

	err = os.WriteFile(filename, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("error writing process tree to file: %v", err)
	}

	fmt.Printf("Process tree saved to %s\n", filename)
	return nil
}
