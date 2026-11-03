package onboard

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type dockerSysdumpClient interface {
	ContainerList(context.Context, client.ContainerListOptions) (client.ContainerListResult, error)
	ContainerInspect(context.Context, string, client.ContainerInspectOptions) (client.ContainerInspectResult, error)
	ContainerLogs(context.Context, string, client.ContainerLogsOptions) (client.ContainerLogsResult, error)
	ServerVersion(context.Context, client.ServerVersionOptions) (client.ServerVersionResult, error)
}

// DumpDockerDiagnostics collects local agent diagnostics without executing
// commands in containers or requiring Docker Compose. Per-container failures
// are returned as warnings and saved in docker/errors.log.
func DumpDockerDiagnostics(ctx context.Context, destDir, configDir string, logTail int) ([]error, error) {
	cli, err := CreateDockerClient()
	if err != nil {
		return nil, err
	}
	defer cli.Close()
	return dumpDockerDiagnostics(ctx, cli, destDir, configDir, logTail)
}

func dumpDockerDiagnostics(ctx context.Context, cli dockerSysdumpClient, destDir, configDir string, logTail int) ([]error, error) {
	if logTail < 0 {
		return nil, fmt.Errorf("log-tail must be zero (all logs) or positive")
	}
	containers, err := cli.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("listing Docker containers: %w", err)
	}
	known := GetKnownContainerMap()
	for _, name := range []string{"discover", "rabbitmq", rraName, cm.Imagescan} {
		known[name] = container.Summary{}
	}
	selected := make([]container.Summary, 0)
	for _, item := range containers.Items {
		matched := false
		for _, name := range append(append([]string{}, item.Names...), item.Labels["com.docker.compose.service"]) {
			if _, ok := known[strings.TrimPrefix(name, "/")]; ok {
				matched = true
			}
		}
		if matched {
			selected = append(selected, item)
		}
	}
	dir := filepath.Join(destDir, "docker")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	if err := writeDockerSysdumpJSON(filepath.Join(dir, "containers.json"), selected); err != nil {
		return nil, err
	}
	var warnings []error
	record := func(err error) {
		if err != nil {
			warnings = append(warnings, err)
		}
	}
	if version, err := cli.ServerVersion(ctx, client.ServerVersionOptions{}); err != nil {
		record(fmt.Errorf("Docker version: %w", err))
	} else {
		record(writeDockerSysdumpJSON(filepath.Join(dir, "version.json"), version))
	}
	configDirs := make(map[string]bool)
	if configDir != "" {
		configDirs[filepath.Clean(configDir)] = true
	}
	for _, item := range selected {
		if err := ctx.Err(); err != nil {
			record(err)
			break
		}
		// Prefer a readable container name; use the ID if no safe name is
		// available. API calls continue to use the stable container ID.
		id := filepath.Base(item.ID)
		if id == "." || id == ".." || id == string(filepath.Separator) {
			record(fmt.Errorf("invalid container ID %q", item.ID))
			continue
		}
		folder := id
		for _, rawName := range item.Names {
			name := strings.TrimPrefix(rawName, "/")
			if filepath.IsLocal(name) && filepath.Base(name) == name && name != "." && !strings.ContainsRune(name, '\\') {
				folder = name
				break
			}
		}
		containerDir := filepath.Join(dir, "containers", folder)
		if err := os.MkdirAll(containerDir, 0700); err != nil {
			record(err)
			continue
		}
		inspectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		inspection, err := cli.ContainerInspect(inspectCtx, item.ID, client.ContainerInspectOptions{})
		cancel()
		if err != nil {
			record(fmt.Errorf("inspecting %s: %w", item.ID, err))
		} else {
			info := inspection.Container
			if configDir == "" {
				for _, mount := range info.Mounts {
					for path := filepath.Clean(mount.Source); filepath.IsAbs(path) && path != filepath.Dir(path); path = filepath.Dir(path) {
						if filepath.Base(path) == cm.DefaultConfigPathDirName {
							configDirs[path] = true
							break
						}
					}
				}
			}
			redactDockerSysdumpEnv(&info)
			record(writeDockerSysdumpJSON(filepath.Join(containerDir, "inspect.json"), info))
		}
		logsCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		tail := strconv.Itoa(logTail)
		if logTail == 0 {
			tail = "all"
		}
		stream, err := cli.ContainerLogs(logsCtx, item.ID, client.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Timestamps: true, Tail: tail})
		if err != nil {
			record(fmt.Errorf("logs for %s: %w", item.ID, err))
		} else {
			if err := saveDockerSysdumpLogs(filepath.Join(containerDir, "logs.txt"), stream); err != nil {
				record(fmt.Errorf("copying logs for %s: %w", item.ID, err))
			}
			stream.Close()
		}
		cancel()
	}
	if configDir == "" && len(configDirs) == 0 {
		if path, err := cm.GetDefaultConfigPath(); err != nil {
			record(err)
		} else if _, err := os.Stat(path); err == nil {
			configDirs[path] = true
		}
	}
	paths := make([]string, 0, len(configDirs))
	for path := range configDirs {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		record(fmt.Errorf("onboarding config directory not found; specify --config-path to include it"))
	}
	for i, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			record(fmt.Errorf("onboarding config %s: %w", path, err))
			continue
		}
		if !info.IsDir() || filepath.Clean(path) == string(filepath.Separator) {
			record(fmt.Errorf("onboarding config %s must be a directory other than the filesystem root", path))
			continue
		}
		record(readAndDumpDir(path, filepath.Join(dir, "config", strconv.Itoa(i+1))))
	}
	record(writeDockerSysdumpJSON(filepath.Join(dir, "config-sources.json"), paths))
	if len(selected) == 0 {
		record(fmt.Errorf("no AccuKnox agent containers found (including stopped containers)"))
	}
	if len(warnings) > 0 {
		var messages []string
		for _, warning := range warnings {
			messages = append(messages, warning.Error())
		}
		if err := os.WriteFile(filepath.Join(dir, "errors.log"), []byte(strings.Join(messages, "\n")+"\n"), 0600); err != nil {
			return warnings, err
		}
	}
	return warnings, nil
}

func writeDockerSysdumpJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0600)
}

func redactDockerSysdumpEnv(info *container.InspectResponse) {
	if info.Config == nil {
		return
	}
	// Copy slices before editing: callers may reuse the inspection response.
	config := *info.Config
	config.Env = append([]string(nil), config.Env...)
	for i, entry := range config.Env {
		key, _, _ := strings.Cut(entry, "=")
		for _, sensitive := range []string{"TOKEN", "PASSWORD", "SECRET", "KEY", "AUTH", "CREDENTIAL"} {
			if strings.Contains(strings.ToUpper(key), sensitive) {
				config.Env[i] = key + "=[REDACTED]"
				break
			}
		}
	}
	info.Config = &config
}

func saveDockerSysdumpLogs(path string, stream io.Reader) (err error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}()
	reader := bufio.NewReader(stream)
	header, _ := reader.Peek(8)
	if len(header) >= 8 && header[0] <= byte(stdcopy.Systemerr) && header[1] == 0 && header[2] == 0 && header[3] == 0 {
		_, err = stdcopy.StdCopy(file, file, reader)
	} else {
		_, err = io.Copy(file, reader)
	}
	return err
}
