package scan

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// isKubeArmorActive checks if KubeArmor is running as systemd service
func isKubeArmorActive() bool {
	cmd := exec.Command("systemctl", "is-active", "kubearmor")
	output, _ := cmd.Output()

	return strings.TrimSpace(string(output)) == "active"
}

// getActualProcessName name gets the name of the process given the absolute path
// of the process
func getActualProcessName(absolutePath string) string {
	return filepath.Base(absolutePath)
}

// simplifyCommand removes the path from the command and returns only
// essential parts
func simplifyCommand(command string) string {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ""
	}

	parts[0] = filepath.Base(parts[0])

	return strings.Join(parts, " ")
}

// extractNetworkType extracts network type
func extractNetworkFlow(data, resource string) string {
	if strings.Contains(data, "tcp_connect") || strings.Contains(data, "SYS_CONNECT") {
		return "egress"
	} else if strings.Contains(data, "tcp_accept") {
		return "ingress"
	} else if strings.Contains(data, "SYS_SOCKET") && strings.Contains(resource, "SOCK_DGRAM") {
		return "egress"
	}

	return ""
}
