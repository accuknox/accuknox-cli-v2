//go:build linux

package onboard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"

	dbus "github.com/coreos/go-systemd/v22/dbus"
	godbus "github.com/godbus/dbus/v5"

	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
)

// PrintAgentRestartCounts reads counters without starting or resetting services.
func PrintAgentRestartCounts(ctx context.Context, out io.Writer, exclude map[string]bool) error {
	conn, err := dbus.NewWithContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to systemd: %w", err)
	}
	defer conn.Close()
	read := func(ctx context.Context, service string) (map[string]any, error) {
		properties, err := conn.GetAllPropertiesContext(ctx, service)
		if isSystemdUnitNotLoaded(err) {
			return nil, errRestartUnitNotLoaded
		}
		return properties, err
	}
	return printAgentRestartCounts(ctx, out, agentRestartServices(), exclude, read)
}

func isSystemdUnitNotLoaded(err error) bool {
	var dbusErr godbus.Error
	if errors.As(err, &dbusErr) {
		return dbusErr.Name == "org.freedesktop.systemd1.NoSuchUnit" || dbusErr.Name == "org.freedesktop.DBus.Error.UnknownObject"
	}
	return false
}

func StartSystemdService(serviceName string) error {
	if serviceName == "" {
		return nil
	}
	ctx := context.Background()
	// Connect to systemd dbus
	conn, err := dbus.NewWithContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to systemd: %v", err)
	}
	defer conn.Close()

	// reload systemd config, equivalent to systemctl daemon-reload
	if err := conn.ReloadContext(ctx); err != nil {
		return fmt.Errorf("failed to reload systemd configuration: %v", err)
	}

	// enable service
	_, _, err = conn.EnableUnitFilesContext(ctx, []string{serviceName}, false, true)
	if err != nil {
		return fmt.Errorf("failed to enable %s: %v", serviceName, err)
	}

	// Start the service
	ch := make(chan string, 1)
	if _, err := conn.RestartUnitContext(ctx, serviceName, "replace", ch); err != nil {
		return fmt.Errorf("failed to start %s: %v", serviceName, err)
	}
	logger.Print("Started %s", serviceName)

	return nil
}

func StopSystemdService(serviceName string, skipDeleteDisable, force bool) error {
	ctx := context.Background()
	conn, err := dbus.NewWithContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to systemd: %v", err)
	}
	defer conn.Close()

	loaded := true
	state := ""
	property, err := conn.GetUnitPropertyContext(ctx, serviceName, "ActiveState")
	if err != nil {
		if !strings.Contains(err.Error(), "not loaded") {
			return fmt.Errorf("failed to check service status: %v", err)
		}
		loaded = false
	} else {
		state, _ = property.Value.Value().(string)
	}

	if loaded && state != "inactive" && state != "failed" {
		stopChan := make(chan string, 1)
		if _, err := conn.StopUnitContext(ctx, serviceName, "replace", stopChan); err != nil {
			if !strings.Contains(err.Error(), "not loaded") {
				return fmt.Errorf("failed to stop existing %s service: %v", serviceName, err)
			}
		} else {
			logger.Info1("Stopping existing %s...", serviceName)
			select {
			case res := <-stopChan:
				if res != "done" {
					logger.Error("Stop job for %s finished with result %q", serviceName, res)
				} else {
					logger.Info1("%s stopped successfully.", serviceName)
				}
			case <-time.After(2 * time.Minute):
				logger.Error("Timed out waiting for %s to stop", serviceName)
			}
		}
	}

	if loaded {
		killLeftovers(ctx, conn, serviceName)
	}

	if !skipDeleteDisable {
		if _, err := conn.DisableUnitFilesContext(ctx, []string{serviceName}, false); err != nil {
			if !strings.Contains(err.Error(), "does not exist") &&
				!strings.Contains(err.Error(), "No such file or directory") {
				logger.Error("Failed to disable %s : %v", serviceName, err)
				return err
			}
		} else {
			logger.Info1("Disabled %s", serviceName)
		}

		svcFilePath := cm.SystemdDir + serviceName
		if err := os.Remove(svcFilePath); err != nil {
			if !os.IsNotExist(err) {
				logger.Error("Failed to delete %s file: %v", serviceName, err)
				return err
			}
		}

		// reload systemd config, equivalent to systemctl daemon-reload
		if err := conn.ReloadContext(ctx); err != nil {
			return fmt.Errorf("failed to reload systemd configuration: %v", err)
		}
	}

	return nil
}
func killLeftovers(ctx context.Context, conn *dbus.Conn, serviceName string) {
	if err := conn.KillUnitWithTarget(ctx, serviceName, dbus.All, int32(syscall.SIGTERM)); err != nil {
		if !strings.Contains(err.Error(), "not loaded") {
			logger.Info1("No leftover processes for %s (%v)", serviceName, err)
		}
		return
	}

	time.Sleep(5 * time.Second)

	if err := conn.KillUnitWithTarget(ctx, serviceName, dbus.All, int32(syscall.SIGKILL)); err != nil {
		logger.Info1("SIGKILL of leftovers for %s: %v", serviceName, err)
	}
}

func GetSystemdServiceStatus(name string) (string, error) {
	ctx := context.Background()
	status := ""
	// Connect to systemd dbus
	conn, err := dbus.NewWithContext(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to connect to systemd: %v", err)
	}
	defer conn.Close()

	props, err := conn.GetUnitPropertiesContext(ctx, name)
	if err != nil {
		return "", fmt.Errorf("failed to get properties for service %s: %v", name, err)
	}
	status, ok := props["ActiveState"].(string)
	if !ok {
		return "", fmt.Errorf("could not interpret ActiveState for %s", name)
	}

	return status, nil
}

func ResetRestartCounter(service string) error {
	if service == "" {
		return nil
	}
	ctx := context.Background()

	dConn, err := dbus.NewWithContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to systemd: %v", err)
	}
	defer dConn.Close()

	return resetRestartCounter(ctx, service, dConn)
}

type restartCounterConnection interface {
	GetUnitPropertyContext(context.Context, string, string) (*dbus.Property, error)
	ResetFailedUnitContext(context.Context, string) error
	StartUnitContext(context.Context, string, string, chan<- string) (int, error)
}

func resetRestartCounter(ctx context.Context, service string, conn restartCounterConnection) error {
	loadState, err := conn.GetUnitPropertyContext(ctx, service, "LoadState")
	if isSystemdUnitNotLoaded(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("checking %s: %w", service, err)
	}
	if loadState.Value.Value() == "not-found" {
		return nil
	}
	if err := conn.ResetFailedUnitContext(ctx, service); err != nil {
		if isSystemdUnitNotLoaded(err) {
			return nil
		}
		return fmt.Errorf("failed to reset restart counter for %s: %w", service, err)
	}

	ch := make(chan string, 1)
	_, err = conn.StartUnitContext(ctx, service, "replace", ch)
	if err != nil {
		if isSystemdUnitNotLoaded(err) {
			return nil
		}
		return fmt.Errorf("failed to start %s: %w", service, err)
	}

	select {
	case msg := <-ch:
		if msg != "done" {
			return fmt.Errorf("failed to start %s: job result %q", service, msg)
		}
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(20 * time.Second):
		return fmt.Errorf("timeout waiting for %s to start", service)
	}

	return nil
}
