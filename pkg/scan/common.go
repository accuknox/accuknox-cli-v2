package scan

import (
	"context"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/publicsuffix"
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

// performDNSLookup atempts to resolve given an IP address to a domain name
func performDNSLookup(ip string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	names, err := net.DefaultResolver.LookupAddr(ctx, ip)
	if err != nil || len(names) == 0 {
		return ""
	}

	return strings.TrimSuffix(names[0], ".")
}

// extractMainDomain tries to get TLD+1 name of a given domain
func extractMainDomain(hostname string) string {
	hostname = strings.TrimSuffix(hostname, ".")

	parts := strings.Split(hostname, ".")

	if len(parts) <= 2 {
		return hostname
	}

	domain, err := publicsuffix.EffectiveTLDPlusOne(hostname)
	if err != nil {
		return strings.Join(parts[len(parts)-2:], ".")
	}

	return domain
}

// isValidIPv4 checks if the strings represents a valid IP address
func isValidIPv4(ip string) bool {
	return net.ParseIP(ip) != nil
}
