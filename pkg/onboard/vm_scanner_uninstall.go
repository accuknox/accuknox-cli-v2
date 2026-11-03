package onboard

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// UninstallVMScanner removes the native and legacy Linux Omni installation.
// Missing installations return os.ErrNotExist.
// Knoxctl itself is shared with other commands and is never removed.
func UninstallVMScanner(ctx context.Context) error {
	run := func(ctx context.Context, args ...string) (string, error) {
		output, err := exec.CommandContext(ctx, "systemctl", args...).CombinedOutput()
		if err != nil {
			return string(output), fmt.Errorf("systemctl %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
		}
		return string(output), nil
	}
	return uninstallVMScanner(ctx, []string{VMScannerSystemdDirectory, "/etc/systemd/system"}, []string{
		vmScannerLogDirectory,
		"/var/lib/knoxctl/vm-scan",
		"/var/lib/accuknox-omni",
		"/usr/local/bin/omni",
		"/var/lib/systemd/timers/stamp-omni.timer",
	}, run)
}

func uninstallVMScanner(ctx context.Context, unitDirs, dataPaths []string, run vmScannerCommand) error {
	var unitPaths []string
	for _, dir := range unitDirs {
		units, err := legacyVMScannerUnits(dir, "")
		if err != nil {
			return err
		}
		for _, unit := range []string{"omni.timer", "omni.service"} {
			if _, exists := units[unit]; exists {
				unitPaths = append(unitPaths, filepath.Join(dir, unit))
			}
		}
	}
	if len(unitPaths) > 0 {
		var stopErrors []error
		for _, unit := range []string{"omni.timer", "omni.service"} {
			state, err := run(ctx, "show", "--property=LoadState", "--value", unit)
			if strings.TrimSpace(state) == "not-found" {
				continue
			}
			if err == nil {
				_, err = run(ctx, "disable", "--now", unit)
			}
			if err != nil {
				stopErrors = append(stopErrors, fmt.Errorf("stopping %s: %w", unit, err))
			}
		}
		if err := errors.Join(stopErrors...); err != nil {
			return err
		}
	}
	found := len(unitPaths) > 0
	var cleanupErrors []error
	for _, path := range unitPaths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("removing %s: %w", path, err))
		}
	}
	if len(unitPaths) > 0 {
		if _, err := run(ctx, "daemon-reload"); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
	}
	for _, path := range dataPaths {
		if _, err := os.Lstat(path); err == nil {
			found = true
		} else if !os.IsNotExist(err) {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("checking %s: %w", path, err))
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("removing %s: %w", path, err))
		}
	}
	if err := errors.Join(cleanupErrors...); err != nil {
		return err
	}
	if !found {
		return os.ErrNotExist
	}
	return nil
}
