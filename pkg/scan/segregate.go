package scan

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	kaproto "github.com/kubearmor/KubeArmor/protobuf"
)

type SegregatedData struct {
	Logs   OperationLogs
	Alerts OperationAlerts
	mu     sync.RWMutex
}

type OperationLogs struct {
	Network []kaproto.Log
	File    []kaproto.Log
	Process []kaproto.Log
}

type OperationAlerts struct {
	Network []kaproto.Alert
	File    []kaproto.Alert
	Process []kaproto.Alert
}

type Segregate struct {
	data *SegregatedData
}

func NewSegregator() *Segregate {
	return &Segregate{
		data: &SegregatedData{},
	}
}

func (sg *Segregate) SegregateAlert(alert *kaproto.Alert) {
	sg.data.mu.Lock()
	defer sg.data.mu.Unlock()

	switch alert.Operation {
	case common.OperationFile:
		sg.data.Alerts.File = append(sg.data.Alerts.File, *alert)

	case common.OperationNetwork:
		sg.data.Alerts.Network = append(sg.data.Alerts.Network, *alert)

	case common.OperationProcess:
		sg.data.Alerts.Process = append(sg.data.Alerts.Process, *alert)
	}
}

func (sg *Segregate) SegregateLogs(logs *kaproto.Log) {
	sg.data.mu.Lock()
	defer sg.data.mu.Unlock()

	switch logs.Operation {
	case common.OperationFile:
		sg.data.Logs.File = append(sg.data.Logs.File, *logs)

	case common.OperationNetwork:
		sg.data.Logs.Network = append(sg.data.Logs.Network, *logs)

	case common.OperationProcess:
		sg.data.Logs.Process = append(sg.data.Logs.Process, *logs)
	}
}

func (sg *Segregate) PrintSegregatedDataJSON() (string, error) {
	sg.data.mu.RLock()
	defer sg.data.mu.RUnlock()

	jsonData, err := json.MarshalIndent(sg.data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error marshaling segregated data to JSON: %v", err)
	}

	return string(jsonData), nil
}

func (sg *Segregate) SaveSegregatedDataToFile(filename string) error {
	sg.data.mu.RLock()
	defer sg.data.mu.RUnlock()

	jsonData, err := json.MarshalIndent(sg.data, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling segregated data to JSON: %v", err)
	}

	err = os.WriteFile(filename, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("error writing segregated data to file: %v", err)
	}

	fmt.Printf("Segregated data saved to %s\n", filename)
	return nil
}
