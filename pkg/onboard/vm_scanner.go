package onboard

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"
)

//go:embed templates/systemdTemplates/vm-scan.service
var vmScannerServiceTemplate string

//go:embed templates/systemdTemplates/vm-scan.timer
var vmScannerTimerTemplate string

const (
	vmScannerLogDirectory     = "/opt/omni"
	VMScannerSystemdDirectory = "/lib/systemd/system/"
)

type VMScannerServiceConfig struct {
	Executable     string
	Arguments      []string
	Token          string
	AgentDirectory string
}

type vmScannerCommand func(context.Context, ...string) (string, error)

// InstallVMScanner installs and enables the native service and daily timer,
// then starts an initial scan. It does not invoke an installation script.
func InstallVMScanner(ctx context.Context, c VMScannerServiceConfig, stdout, stderr io.Writer) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("native scheduled VM scanning requires Linux/systemd; use --use-script for Omni's macOS installer")
	}
	if c.AgentDirectory == "" {
		c.AgentDirectory = vmScannerLogDirectory
	}
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	run := func(ctx context.Context, args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, "systemctl", args...)
		var output bytes.Buffer
		if len(args) > 0 && args[0] == "show" {
			cmd.Stdout = &output
		} else {
			cmd.Stdout = io.MultiWriter(stdout, &output)
		}
		cmd.Stderr = stderr
		err := cmd.Run()
		return output.String(), err
	}
	return installVMScannerWithLegacy(ctx, c, VMScannerSystemdDirectory, "/var/lib/knoxctl/vm-scan", "/etc/systemd/system", run)
}

func systemdArgument(value string) string {
	value = strings.ReplaceAll(value, "%", "%%")
	value = strings.ReplaceAll(value, "$", "$$")
	return strconv.Quote(value)
}

func installVMScanner(ctx context.Context, c VMScannerServiceConfig, unitDir, stateDir string, run vmScannerCommand) error {
	return installVMScannerWithLegacy(ctx, c, unitDir, stateDir, "", run)
}

func installVMScannerWithLegacy(ctx context.Context, c VMScannerServiceConfig, unitDir, stateDir, legacyDir string, run vmScannerCommand) error {
	if !filepath.IsAbs(c.Executable) {
		return fmt.Errorf("knoxctl executable path must be absolute")
	}
	if !filepath.IsAbs(c.AgentDirectory) {
		return fmt.Errorf("VM scanner agent directory must be absolute")
	}
	if strings.ContainsAny(c.Token, "\x00\r\n") {
		return fmt.Errorf("artifact token must not contain NUL or newline characters")
	}
	var args []string
	for _, arg := range append([]string{c.Executable}, c.Arguments...) {
		if strings.ContainsRune(arg, '\x00') {
			return fmt.Errorf("VM scanner arguments must not contain NUL characters")
		}
		args = append(args, systemdArgument(arg))
	}
	envPath := filepath.Join(c.AgentDirectory, "scanner.env")
	values := map[string]any{
		"EnvironmentFile": strings.ReplaceAll(envPath, "%", "%%"),
		"Command":         strings.Join(args, " "),
		"LogDirectory":    strings.ReplaceAll(c.AgentDirectory, "%", "%%"),
		"SystemdVersion":  getSystemdVersion(),
	}
	units := make(map[string][]byte)
	for name, source := range map[string]string{"omni.service": vmScannerServiceTemplate, "omni.timer": vmScannerTimerTemplate} {
		tmpl, err := template.New(name).Option("missingkey=error").Parse(source)
		if err != nil {
			return err
		}
		var output bytes.Buffer
		if err := tmpl.Execute(&output, values); err != nil {
			return err
		}
		units[name] = output.Bytes()
	}
	legacy, err := legacyVMScannerUnits(legacyDir, unitDir)
	if err != nil {
		return err
	}
	// Check systemd before writing files, and stop any earlier native or script
	// deployment before replacing its credentials or units.
	for _, unit := range []string{"omni.timer", "omni.service"} {
		state, err := run(ctx, "show", "--property=LoadState", "--value", unit)
		if strings.TrimSpace(state) == "not-found" {
			continue
		}
		if err != nil {
			return fmt.Errorf("checking %s: %w", unit, err)
		}
		if _, err := run(ctx, "disable", "--now", unit); err != nil {
			return fmt.Errorf("stopping %s: %w", unit, err)
		}
	}
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(stateDir, 0750); err != nil {
		return err
	}
	if err := os.MkdirAll(c.AgentDirectory, 0755); err != nil {
		return fmt.Errorf("creating VM scanner agent directory: %w", err)
	}

	environment := "OMNI_ARTIFACT_API_TOKEN=" + strconv.Quote(c.Token) + "\n"
	if err := writeVMScannerFile(envPath, []byte(environment), 0600); err != nil {
		return err
	}
	for _, unit := range []string{"omni.service", "omni.timer"} {
		if err := writeVMScannerFile(filepath.Join(unitDir, unit), units[unit], 0644); err != nil {
			return err
		}
	}

	for _, unit := range []string{"omni.service", "omni.timer"} {
		data, exists := legacy[unit]
		if !exists {
			continue
		}
		backupDir := filepath.Join(stateDir, "previous-units")
		if err := os.MkdirAll(backupDir, 0700); err != nil {
			return err
		}
		if err := writeVMScannerFile(filepath.Join(backupDir, unit), data, 0600); err != nil {
			return err
		}
		oldPath := filepath.Join(legacyDir, unit)
		current, err := os.ReadFile(oldPath)
		if err != nil {
			return err
		}
		if !bytes.Equal(current, data) {
			return fmt.Errorf("%s changed during installation; leaving it in place", oldPath)
		}
		if err := os.Remove(oldPath); err != nil {
			return err
		}
	}
	for _, args := range [][]string{{"daemon-reload"}, {"enable", "omni.service"}, {"enable", "--now", "omni.timer"}, {"restart", "omni.service"}} {
		if _, err := run(ctx, args...); err != nil {
			return fmt.Errorf("systemctl %s: %w", strings.Join(args, " "), err)
		}
	}
	oldEnv := filepath.Join(stateDir, "scanner.env")
	if oldEnv != envPath {
		info, err := os.Lstat(oldEnv)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("checking old scanner environment: %w", err)
		}
		if err == nil && info.Mode().IsRegular() {
			data, err := os.ReadFile(oldEnv)
			if err != nil {
				return err
			}
			if bytes.HasPrefix(data, []byte("OMNI_ARTIFACT_API_TOKEN=")) {
				if err := os.Remove(oldEnv); err != nil {
					return fmt.Errorf("removing migrated scanner environment: %w", err)
				}
			}
		}
	}
	return nil
}

func legacyVMScannerUnits(legacyDir, unitDir string) (map[string][]byte, error) {
	units := make(map[string][]byte)
	if legacyDir == "" || filepath.Clean(legacyDir) == filepath.Clean(unitDir) {
		return units, nil
	}
	for unit, descriptions := range map[string][]string{
		"omni.service": {"Description=AccuKnox Omni", "Description=AccuKnox Knoxctl VM vulnerability scanner"},
		"omni.timer":   {"Description=Run AccuKnox Omni periodically", "Description=Run AccuKnox VM vulnerability scanning daily"},
	} {
		path := filepath.Join(legacyDir, unit)
		if target, err := filepath.EvalSymlinks(path); err == nil {
			newTarget, targetErr := filepath.EvalSymlinks(filepath.Join(unitDir, unit))
			if targetErr == nil && filepath.Clean(target) == filepath.Clean(newTarget) {
				continue
			}
		}
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		known := false
		for _, line := range strings.Split(string(data), "\n") {
			for _, description := range descriptions {
				if strings.TrimSpace(line) == description {
					known = true
				}
			}
		}
		if !known {
			return nil, fmt.Errorf("%s overrides the VM scanner but is not a recognized Omni unit; migrate this custom unit before reinstalling", path)
		}
		units[unit] = data
	}
	return units, nil
}

func writeVMScannerFile(path string, data []byte, mode os.FileMode) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".vm-scan-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(mode); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
