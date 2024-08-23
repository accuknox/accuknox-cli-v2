package scan

import (
	"encoding/json"
	"fmt"
	"strings"

	kaproto "github.com/kubearmor/KubeArmor/protobuf"
)

type Alert struct {
	// Name of the policy
	PolicyName string `json:"policyName"`

	// Operation
	Operation string `json:"operation"`

	// PID
	PID int32 `json:"pid"`

	// Process name
	ProcessName string `json:"processName"`

	// Command
	Command string `json:"command"`

	// Message
	Message string `json:"message"`

	// Tags
	Tags []string `json:"tags"`
}

type AlertProcessor struct {
	alerts []Alert
}

func NewAlertProcessor() *AlertProcessor {
	return &AlertProcessor{
		alerts: []Alert{},
	}
}

func (ap *AlertProcessor) ProcessAlerts(segregatedData *SegregatedData) {
	ap.processAlertGroup(segregatedData.Alerts.Network)
	ap.processAlertGroup(segregatedData.Alerts.File)
	ap.processAlertGroup(segregatedData.Alerts.Process)
}

func (ap *AlertProcessor) processAlertGroup(alerts []kaproto.Alert) {
	for _, kaAlert := range alerts {
		alert := Alert{
			PolicyName:  kaAlert.PolicyName,
			Operation:   kaAlert.Operation,
			PID:         kaAlert.PID,
			ProcessName: kaAlert.ProcessName,
			Command:     getActualProcessName(kaAlert.Source),
			Message:     kaAlert.Message,
			Tags:        ap.processTags(kaAlert),
		}
		ap.alerts = append(ap.alerts, alert)
	}
}

func (ap *AlertProcessor) processTags(kaAlert kaproto.Alert) []string {
	if len(kaAlert.ATags) > 0 {
		return kaAlert.ATags
	}
	return strings.Split(kaAlert.Tags, ",")
}

// GenerateJSON generates a JSON representation of the alerts
func (ap *AlertProcessor) GenerateJSON() ([]byte, error) {
	return json.MarshalIndent(ap.alerts, "", "  ")
}

// GenerateMarkdownTable generates a fancy markdown table of alerts
func (ap *AlertProcessor) GenerateMarkdownTable() string {
	var sb strings.Builder

	sb.WriteString("| 📜 Policy Name | 🔧 Operation | 🔢 PID | ⚡ Command | 💻 Process Name | 📣 Message | 🏷️ Tags |\n")
	sb.WriteString("|----------------|--------------|--------|------------|----------------|-----------|--------|\n")

	for _, alert := range ap.alerts {
		sb.WriteString(fmt.Sprintf("| %s | %s | %d | %s | %s | %s | %s |\n",
			alert.PolicyName,
			alert.Operation,
			alert.PID,
			alert.Command,
			alert.ProcessName,
			alert.Message,
			strings.Join(alert.Tags, ", ")))
	}

	return sb.String()
}
