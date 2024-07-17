package scan

import (
	"encoding/json"
	"testing"

	kaproto "github.com/kubearmor/KubeArmor/protobuf"
)

func TestNewProcessForest(t *testing.T) {
	pf := NewProcessForest()
	if pf.Roots == nil || pf.Nodes == nil {
		t.Error("NewProcessForest did not initialize Roots or Nodes")
	}
}

func TestAddProcess(t *testing.T) {
	pf := NewProcessForest()
	log := &kaproto.Log{
		HostPID:     1,
		HostPPID:    0,
		ProcessName: "test",
		Resource:    "test command",
	}

	pf.AddProcess(log)

	if len(pf.Nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(pf.Nodes))
	}

	node, exists := pf.Nodes[1]
	if !exists {
		t.Error("Node with PID 1 not found")
	}

	if node.ProcessName != "test" || node.Command != "test command" {
		t.Error("Node data not set correctly")
	}
}

func TestBuildFromSegregatedData(t *testing.T) {
	pf := NewProcessForest()
	data := []kaproto.Log{
		{HostPID: 1, HostPPID: 0, ProcessName: "root"},
		{HostPID: 2, HostPPID: 1, ProcessName: "child1"},
		{HostPID: 3, HostPPID: 1, ProcessName: "child2"},
		{HostPID: 4, HostPPID: 2, ProcessName: "grandchild"},
	}

	pf.BuildFromSegregatedData(data)

	if len(pf.Roots) != 1 {
		t.Errorf("Expected 1 root, got %d", len(pf.Roots))
	}

	root := pf.Roots[0]
	if root.PID != 1 || len(root.Children) != 2 {
		t.Errorf("Root node not correct. PID: %d, Children: %d", root.PID, len(root.Children))
	}

	if len(pf.Nodes[2].Children) != 1 {
		t.Errorf("Expected 1 grandchild, got %d", len(pf.Nodes[2].Children))
	}
}

func TestSaveProcessForestJSON(t *testing.T) {
	pf := NewProcessForest()
	data := []kaproto.Log{
		{HostPID: 1, HostPPID: 0, ProcessName: "root"},
		{HostPID: 2, HostPPID: 1, ProcessName: "child"},
	}

	pf.BuildFromSegregatedData(data)

	jsonData, err := json.Marshal(pf)
	if err != nil {
		t.Fatalf("Failed to marshal ProcessForest: %v", err)
	}

	var unmarshaledPF ProcessForest
	err = json.Unmarshal(jsonData, &unmarshaledPF)
	if err != nil {
		t.Fatalf("Failed to unmarshal ProcessForest: %v", err)
	}

	if len(unmarshaledPF.Roots) != 1 || len(unmarshaledPF.Roots[0].Children) != 1 {
		t.Error("Unmarshaled data does not match expected structure")
	}
}

func TestOrphanedProcesses(t *testing.T) {
	pf := NewProcessForest()
	data := []kaproto.Log{
		{HostPID: 1, HostPPID: 0, ProcessName: "root"},
		{HostPID: 2, HostPPID: 999, ProcessName: "orphan"}, // Parent doesn't exist
	}

	pf.BuildFromSegregatedData(data)

	if len(pf.Roots) != 2 {
		t.Errorf("Expected 2 roots (including orphan), got %d", len(pf.Roots))
	}
}

func TestConcurrentAccess(t *testing.T) {
	pf := NewProcessForest()
	done := make(chan bool)

	go func() {
		for i := 0; i < 100; i++ {
			pf.AddProcess(&kaproto.Log{HostPID: int32(i), ProcessName: "test"})
		}
		done <- true
	}()

	go func() {
		for i := 100; i < 200; i++ {
			pf.AddProcess(&kaproto.Log{HostPID: int32(i), ProcessName: "test"})
		}
		done <- true
	}()

	<-done
	<-done

	if len(pf.Nodes) != 200 {
		t.Errorf("Expected 200 nodes, got %d", len(pf.Nodes))
	}
}

func TestComplexProcessTree(t *testing.T) {
	pf := NewProcessForest()
	data := []kaproto.Log{
		{HostPID: 1, HostPPID: 0, ProcessName: "init"},
		{HostPID: 2, HostPPID: 1, ProcessName: "system_process_1"},
		{HostPID: 3, HostPPID: 1, ProcessName: "system_process_2"},
		{HostPID: 4, HostPPID: 2, ProcessName: "child_of_2"},
		{HostPID: 5, HostPPID: 2, ProcessName: "another_child_of_2"},
		{HostPID: 6, HostPPID: 3, ProcessName: "child_of_3"},
		{HostPID: 7, HostPPID: 4, ProcessName: "grandchild_1"},
		{HostPID: 8, HostPPID: 4, ProcessName: "grandchild_2"},
		{HostPID: 9, HostPPID: 6, ProcessName: "grandchild_of_3"},
		{HostPID: 10, HostPPID: 7, ProcessName: "great_grandchild"},
		{HostPID: 11, HostPPID: 999, ProcessName: "orphan_1"}, // Orphan process
		{HostPID: 12, HostPPID: 11, ProcessName: "child_of_orphan"},
		{HostPID: 13, HostPPID: 1000, ProcessName: "orphan_2"},   // Another orphan
		{HostPID: 14, HostPPID: 14, ProcessName: "self_parent"},  // Process with itself as parent
		{HostPID: 15, HostPPID: 1, ProcessName: "late_sibling"},  // Late addition to level 1
		{HostPID: 100, HostPPID: 0, ProcessName: "another_root"}, // Another root process
	}

	pf.BuildFromSegregatedData(data)

	if len(pf.Roots) != 5 {
		t.Errorf("Expected 5 roots, got %d", len(pf.Roots))
	}

	init := findNodeByName(pf.Roots, "init")
	if init == nil {
		t.Fatal("Init process not found in roots")
	}
	if len(init.Children) != 3 {
		t.Errorf("Init should have 3 children, got %d", len(init.Children))
	}

	sys1 := findNodeByName(init.Children, "system_process_1")
	if sys1 == nil {
		t.Fatal("system_process_1 not found")
	}
	if len(sys1.Children) != 2 {
		t.Errorf("system_process_1 should have 2 children, got %d", len(sys1.Children))
	}

	child_of_2 := findNodeByName(sys1.Children, "child_of_2")
	if child_of_2 == nil {
		t.Fatal("child_of_2 not found")
	}
	if len(child_of_2.Children) != 2 {
		t.Errorf("child_of_2 should have 2 children, got %d", len(child_of_2.Children))
	}

	great_grandchild := findNodeByName(pf.Nodes[7].Children, "great_grandchild")
	if great_grandchild == nil {
		t.Fatal("great_grandchild not found")
	}
	if len(great_grandchild.Children) != 0 {
		t.Errorf("great_grandchild should have no children, got %d", len(great_grandchild.Children))
	}

	orphan_1 := findNodeByName(pf.Roots, "orphan_1")
	if orphan_1 == nil {
		t.Fatal("orphan_1 not found in roots")
	}
	if len(orphan_1.Children) != 1 {
		t.Errorf("orphan_1 should have 1 child, got %d", len(orphan_1.Children))
	}

	orphan_2 := findNodeByName(pf.Roots, "orphan_2")
	if orphan_2 == nil {
		t.Fatal("orphan_2 not found in roots")
	}

	self_parent := findNodeByName(pf.Roots, "self_parent")
	if self_parent == nil {
		t.Fatal("self_parent not found in roots")
	}
	if len(self_parent.Children) != 0 {
		t.Errorf("self_parent should have no children, got %d", len(self_parent.Children))
	}

	late_sibling := findNodeByName(init.Children, "late_sibling")
	if late_sibling == nil {
		t.Fatal("late_sibling not found in init's children")
	}

	another_root := findNodeByName(pf.Roots, "another_root")
	if another_root == nil {
		t.Fatal("another_root not found in roots")
	}
}

func findNodeByName(nodes []*ProcessNode, name string) *ProcessNode {
	for _, node := range nodes {
		if node.ProcessName == name {
			return node
		}
	}
	return nil
}
