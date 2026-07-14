//go:build linux

package onboard

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	dbus "github.com/coreos/go-systemd/v22/dbus"

	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
)

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

	stopChan := make(chan string, 1)

	property, err := conn.GetUnitPropertyContext(ctx, serviceName, "ActiveState")
	if err != nil {
		return fmt.Errorf("Failed to check service status: %s", err.Error())
	}

	state, ok := property.Value.Value().(string)
	if !ok {
		return fmt.Errorf("failed to get %s service state", serviceName)
	}
	if state != "active" && state != "deactivating" && !force {
		return nil
	}

	if _, err := conn.StopUnitContext(ctx, serviceName, "replace", stopChan); err != nil {
		if !strings.Contains(err.Error(), "not loaded") {
			return fmt.Errorf("Failed to stop existing %s service: %v\n", serviceName, err)
		}
	} else {
		logger.Info1("Stopping existing %s...", serviceName)
		<-stopChan
		logger.Info1("%s stopped successfully.", serviceName)
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

	err = dConn.ResetFailedUnitContext(ctx, service)
	if err != nil {
		fmt.Printf("failed to reset restart counter for %s: %v\n", service, err)
		return nil
	}

	err = dConn.ReloadContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to reload systemd configuration: %v", err)
	}

	ch := make(chan string, 1)
	_, err = dConn.StartUnitContext(ctx, service, "replace", ch)
	if err != nil {
		return fmt.Errorf("failed to start %s: %v", service, err)
	}

	select {
	case msg := <-ch:
		if msg != "done" {
			return fmt.Errorf("failed to start %s: %v", service, err)
		}
	case <-time.After(20 * time.Second):
		return fmt.Errorf("timeout waiting for %s to start", service)
	}

	return nil
}
