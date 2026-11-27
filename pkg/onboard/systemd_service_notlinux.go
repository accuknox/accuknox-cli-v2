//go:build !linux

package onboard

import (
	"context"
	"fmt"
	"io"
)

func PrintAgentRestartCounts(ctx context.Context, out io.Writer, exclude map[string]bool) error {
	return fmt.Errorf("systemd is not available on this platform")
}

func StartSystemdService(serviceName string) error {
	return fmt.Errorf("systemd is not available on this platform")
}

func StopSystemdService(serviceName string, skipDeleteDisable, force bool) error {
	return fmt.Errorf("systemd is not available on this platform")
}

func GetSystemdServiceStatus(name string) (string, error) {
	return "", fmt.Errorf("systemd is not available on this platform")
}

func ResetRestartCounter(service string) error {
	return fmt.Errorf("systemd is not available on this platform")
}
