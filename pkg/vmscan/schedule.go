package vmscan

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
)

// ValidateScheduled validates native installation without requiring a script
// or a separate Omni executable. Offline mode needs only local databases.
func (c Config) ValidateScheduled() error {
	if c.UploadTimeout < 0 {
		return fmt.Errorf("upload timeout must be positive")
	}
	if runtime.GOOS != "linux" {
		return fmt.Errorf("native scheduled VM scanning requires Linux/systemd; use --use-script on macOS")
	}
	if c.OutputDir == "" {
		if err := c.Validate(); err != nil {
			return err
		}
	}
	for _, path := range []string{c.RootFS, c.DBDirectory, c.OutputDir} {
		if path != "" && !filepath.IsAbs(path) {
			return fmt.Errorf("scheduled VM scanner filesystem paths must be absolute: %s", path)
		}
	}
	if c.Offline {
		return validateLocalDatabases(c.DBDirectory)
	}
	return nil
}

// InstallScheduled uses Knoxctl's service and timer templates to run Omni's
// packages daily, with the same initial-scan behavior as Omni's installer.
func InstallScheduled(ctx context.Context, c Config, stdout, stderr io.Writer) error {
	if err := c.ValidateScheduled(); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating knoxctl: %w", err)
	}
	return onboard.InstallVMScanner(ctx, onboard.VMScannerServiceConfig{
		Executable: executable,
		Arguments:  scheduledArguments(c),
		Token:      c.Token,
	}, stdout, stderr)
}

func scheduledArguments(c Config) []string {
	args := []string{"vm-scan", "--run-once"}
	if c.OutputDir != "" {
		args = append(args, "--output-dir="+c.OutputDir)
	} else {
		args = append(args, "--artifact-endpoint="+c.Endpoint, "--tenant-id="+strconv.FormatUint(uint64(c.TenantID), 10), "--artifact-label="+c.Label)
	}
	if c.UploadTimeout > 0 {
		args = append(args, "--upload-timeout="+c.UploadTimeout.String())
	}
	if c.ClusterID != nil {
		args = append(args, "--cluster-id="+strconv.Itoa(*c.ClusterID))
	}
	if c.ClusterName != "" {
		args = append(args, "--cluster-name="+c.ClusterName)
	}
	if c.DBDirectory != "" {
		args = append(args, "--vuln-db-directory="+c.DBDirectory)
	}
	if c.RootFS != "" {
		args = append(args, "--rootfs="+c.RootFS)
	}
	if c.Hostname != "" {
		args = append(args, "--hostname="+c.Hostname)
	}
	if c.SkipDBUpdate {
		args = append(args, "--skip-db-update")
	}
	if c.Offline {
		args = append(args, "--offline")
	}
	return args
}
