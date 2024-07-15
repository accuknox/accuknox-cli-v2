package scan

import (
	"os/exec"
	"strings"
)

// isKubeArmorActive checks if KubeArmor is running as systemd service
func isKubeArmorActive() bool {
	cmd := exec.Command("systemctl", "is-active", "kubearmor")
	output, _ := cmd.Output()

	return strings.TrimSpace(string(output)) == "active"
}
