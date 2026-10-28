package vmscan

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
)

// Install installs Omni's native scheduled agent
func Install(ctx context.Context, c Config, stdout, stderr io.Writer) error {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return fmt.Errorf("the Omni Bash installer requires Linux or macOS")
	}
	source := c.ScriptURL
	if source == "" {
		source = InstallerURL
	}
	return install(ctx, c, source, &http.Client{Timeout: 2 * time.Minute}, stdout, stderr)
}

// ValidateInstall checks local assets before any installer execution or download.
func (c Config) ValidateInstall() error {
	if c.ClusterID != nil || c.ClusterName != "" {
		return fmt.Errorf("cluster metadata requires native scanning; omit --use-script")
	}
	if err := c.Validate(); err != nil {
		return err
	}
	if c.OutputDir != "" {
		return fmt.Errorf("--output-dir is supported only for direct package scans")
	}
	if c.ScriptPath != "" {
		info, err := os.Stat(c.ScriptPath)
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			return fmt.Errorf("local Omni installer must be a non-empty readable file: %s", c.ScriptPath)
		}
	}
	for _, source := range []string{c.ScriptURL, c.BinaryURL} {
		if source == "" {
			continue
		}
		u, err := url.ParseRequestURI(source)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Fragment != "" {
			return fmt.Errorf("installer and binary URLs must be HTTP(S) URLs without credentials or fragments")
		}
	}
	if c.Offline {
		if c.ScriptPath == "" {
			return fmt.Errorf("offline installation requires --script-path pointing to a local installer")
		}
		if c.BinaryURL != "" {
			return fmt.Errorf("a binary download URL cannot be used for offline installation")
		}
		installDir := c.InstallDir
		if installDir == "" {
			installDir = "/usr/local/bin"
		}
		binary := filepath.Join(installDir, "omni")
		info, err := os.Stat(binary)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
			return fmt.Errorf("offline installation requires an executable Omni binary at %s", binary)
		}
		return validateLocalDatabases(c.DBDirectory)
	}
	return nil
}

func install(ctx context.Context, c Config, scriptURL string, client *http.Client, stdout, stderr io.Writer) error {
	if err := c.ValidateInstall(); err != nil {
		return err
	}
	var body io.ReadCloser
	if c.ScriptPath != "" {
		file, err := os.Open(c.ScriptPath)
		if err != nil {
			return fmt.Errorf("opening local Omni installer: %w", err)
		}
		body = file
	} else {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, scriptURL, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("downloading Omni installer: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("downloading Omni installer: HTTP %d", resp.StatusCode)
		}
		body = resp.Body
	}
	defer body.Close()
	// Finish downloading before executing so a failed download cannot run a partial script.
	const maxScriptSize = 8 << 20
	script, err := io.ReadAll(io.LimitReader(body, maxScriptSize+1))
	if err != nil {
		return fmt.Errorf("reading Omni installer: %w", err)
	}
	if len(script) == 0 || len(script) > maxScriptSize {
		return fmt.Errorf("invalid Omni installer size")
	}
	cmd := exec.CommandContext(ctx, "bash", "-s", "-",
		"--artifact-endpoint="+c.Endpoint,
		"--tenant-id="+strconv.FormatUint(uint64(c.TenantID), 10),
		"--artifact-label="+c.Label)
	cmd.Env = append(os.Environ(), "OMNI_ARTIFACT_API_TOKEN="+c.Token)
	if runtime.GOOS == "linux" {
		cmd.Env = append(cmd.Env, "OMNI_INSTALL_SYSTEMD_DIR="+onboard.VMScannerSystemdDirectory)
	}
	if c.BinaryURL != "" {
		cmd.Env = append(cmd.Env, "OMNI_URL="+c.BinaryURL)
	}
	if c.InstallDir != "" {
		cmd.Env = append(cmd.Env, "OMNI_INSTALL_PATH="+c.InstallDir)
	}
	if c.DBDirectory != "" {
		cmd.Env = append(cmd.Env, "OMNI_HOME_DIR="+c.DBDirectory)
	}
	if c.SkipDBUpdate || c.Offline {
		cmd.Env = append(cmd.Env, "OMNI_SKIP_DB_DOWNLOAD=true")
	}
	if c.Offline {
		directory, installDir := c.DBDirectory, c.InstallDir
		if directory == "" {
			directory = "/var/lib/accuknox-omni"
		}
		if installDir == "" {
			installDir = "/usr/local/bin"
		}
		cmd.Env = append(cmd.Env, "OMNI_SKIP_DOWNLOAD=true", "OMNI_SKIP_MALWARE_SCAN=true", "OMNI_HOME_DIR="+directory, "OMNI_INSTALL_PATH="+installDir)
	}
	cmd.Stdin = bytes.NewReader(script)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running Omni installer: %w", err)
	}
	return nil
}

// Omni's Linux scanner opens both databases even when it does not update them.
func validateLocalDatabases(directory string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("offline VM scanning is currently supported on Linux only")
	}
	if directory == "" {
		directory = "/var/lib/accuknox-omni"
	}
	for _, path := range []string{"db/trivy.db", "db/metadata.json", "java-db/trivy-java.db", "java-db/metadata.json"} {
		fullPath := filepath.Join(directory, filepath.FromSlash(path))
		info, err := os.Stat(fullPath)
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			return fmt.Errorf("offline scanning requires a pre-provisioned database file: %s", fullPath)
		}
	}
	return nil
}
