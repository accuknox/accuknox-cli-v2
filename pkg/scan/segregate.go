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

// [PID]: Log
type OperationLogs struct {
	Network map[int32]kaproto.Log
	File    map[int32]kaproto.Log
	Process map[int32]kaproto.Log
}

// [PID]: Alert
type OperationAlerts struct {
	Network map[int32]kaproto.Alert
	File    map[int32]kaproto.Alert
	Process map[int32]kaproto.Alert
}

type Segregate struct {
	data *SegregatedData
}

func NewSegregator() *Segregate {
	return &Segregate{
		data: &SegregatedData{
			Logs: OperationLogs{
				Network: make(map[int32]kaproto.Log),
				File:    make(map[int32]kaproto.Log),
				Process: make(map[int32]kaproto.Log),
			},
			Alerts: OperationAlerts{
				Network: make(map[int32]kaproto.Alert),
				File:    make(map[int32]kaproto.Alert),
				Process: make(map[int32]kaproto.Alert),
			},
		},
	}
}

func (sg *Segregate) SegregateAlert(alert *kaproto.Alert) {
	sg.data.mu.Lock()
	defer sg.data.mu.Unlock()

	switch alert.Operation {
	case common.OperationFile:
		if _, exists := sg.data.Alerts.File[alert.PID]; !exists {
			sg.data.Alerts.File[alert.PID] = *alert
		}
	case common.OperationNetwork:
		if _, exists := sg.data.Alerts.Network[alert.PID]; !exists {
			sg.data.Alerts.Network[alert.PID] = *alert
		}
	case common.OperationProcess:
		if _, exists := sg.data.Alerts.Process[alert.PID]; !exists {
			sg.data.Alerts.Process[alert.PID] = *alert
		}
	}
}

func (sg *Segregate) SegregateLogs(log *kaproto.Log) {
	sg.data.mu.Lock()
	defer sg.data.mu.Unlock()

	switch log.Operation {
	case common.OperationFile:
		if _, exists := sg.data.Logs.File[log.PID]; !exists {
			sg.data.Logs.File[log.PID] = *log
		}
	case common.OperationNetwork:
		if _, exists := sg.data.Logs.Network[log.PID]; !exists {
			sg.data.Logs.Network[log.PID] = *log
		}
	case common.OperationProcess:
		if _, exists := sg.data.Logs.Process[log.PID]; !exists {
			sg.data.Logs.Process[log.PID] = *log
		}
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
