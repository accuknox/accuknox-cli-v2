package onboard

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/mod/semver"
)

func getComposeCommand(minVersion string) (string, string, error) {
	var selected, selectedVersion string
	var failures []error
	// Prefer the Docker CLI plugin when versions are equal.
	for _, command := range []string{"docker compose", "docker-compose"} {
		output, err := ExecComposeCommand(false, false, command, "version", "--short")
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", command, err))
			continue
		}
		version := "v" + strings.TrimPrefix(strings.TrimSpace(output), "v")
		if !semver.IsValid(version) {
			failures = append(failures, fmt.Errorf("%s returned invalid version %q", command, strings.TrimSpace(output)))
			continue
		}
		if semver.Compare(version, minVersion) < 0 {
			failures = append(failures, fmt.Errorf("%s version %s is below %s", command, version, minVersion))
			continue
		}
		if selected == "" || semver.Compare(version, selectedVersion) > 0 {
			selected, selectedVersion = command, version
		}
	}
	if selected != "" {
		return selected, selectedVersion, nil
	}
	return "", "", fmt.Errorf("docker requirements not met: %w", errors.Join(failures...))
}

func composeExecutable(command string) (string, []string, error) {
	switch command {
	case "docker compose":
		return "docker", []string{"compose"}, nil
	case "docker-compose":
		return command, nil, nil
	default:
		return "", nil, fmt.Errorf("unsupported Compose command %q", command)
	}
}

func composeHelp(command string, subcommand ...string) (string, error) {
	executable, prefix, err := composeExecutable(command)
	if err != nil {
		return "", err
	}
	args := append(prefix, subcommand...)
	output, err := exec.Command(executable, append(args, "--help")...).CombinedOutput() // #nosec G204 -- executable is allowlisted
	if err != nil {
		return "", fmt.Errorf("checking %s capabilities: %w: %s", command, err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func composeHelpHasFlag(help, flag string) bool {
	for _, field := range strings.Fields(help) {
		if strings.TrimRight(field, ",=") == flag {
			return true
		}
	}
	return false
}

// composeSubcommand skips values of global options so a path or profile named
// "up" cannot be mistaken for the subcommand.
func composeSubcommand(args []string) int {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-f", "--file", "-p", "--project-name", "--profile", "--project-directory", "--env-file", "--parallel", "--ansi", "--progress", "--log-level", "--context", "-c", "--host", "-H":
			i++
		case "--":
			if i+1 < len(args) {
				return i + 1
			}
		default:
			if !strings.HasPrefix(args[i], "-") {
				return i
			}
		}
	}
	return -1
}

func prepareComposeCommand(dryRun bool, command string, args ...string) (*exec.Cmd, error) {
	executable, prefix, err := composeExecutable(command)
	if err != nil {
		return nil, err
	}
	args = append([]string(nil), args...)
	subcommand := composeSubcommand(args)
	parallelIndex := -1
	usesProfiles := false
	for i := 0; i < subcommand; i++ {
		switch args[i] {
		case "--profile":
			usesProfiles = true
			i++
		case "--parallel":
			parallelIndex = i
			i++
		case "-f", "--file", "-p", "--project-name", "--project-directory", "--env-file", "--ansi", "--progress", "--log-level", "--context", "-c", "--host", "-H":
			i++
		}
	}
	var environment []string
	if dryRun || parallelIndex >= 0 || usesProfiles {
		help, err := composeHelp(command)
		if err != nil {
			return nil, err
		}
		if dryRun && !composeHelpHasFlag(help, "--dry-run") {
			return nil, fmt.Errorf("%s does not support --dry-run; install a current Docker Compose plugin", command)
		}
		if usesProfiles && !composeHelpHasFlag(help, "--profile") {
			return nil, fmt.Errorf("%s does not support --profile; install a current Docker Compose plugin", command)
		}
		if parallelIndex >= 0 && !composeHelpHasFlag(help, "--parallel") {
			if parallelIndex+1 >= len(args) {
				return nil, fmt.Errorf("--parallel requires a positive integer")
			}
			limit := args[parallelIndex+1]
			if n, err := strconv.Atoi(limit); err != nil || n <= 0 {
				return nil, fmt.Errorf("invalid --parallel value %q: expected a positive integer", limit)
			}
			// Supported by both legacy Compose and the Docker CLI plugin.
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(entry, "COMPOSE_PARALLEL_LIMIT=") {
					environment = append(environment, entry)
				}
			}
			environment = append(environment, "COMPOSE_PARALLEL_LIMIT="+limit)
			args = append(args[:parallelIndex], args[parallelIndex+2:]...)
			subcommand -= 2
		}
	}
	if subcommand >= 0 && args[subcommand] == "up" {
		var flags []string
		for _, arg := range args[subcommand+1:] {
			flag := strings.SplitN(arg, "=", 2)[0]
			switch flag {
			case "--pull", "--wait", "--wait-timeout":
				flags = append(flags, flag)
			}
		}
		if len(flags) > 0 {
			help, err := composeHelp(command, "up")
			if err != nil {
				return nil, err
			}
			for _, flag := range flags {
				if !composeHelpHasFlag(help, flag) {
					return nil, fmt.Errorf("%s up does not support %s; install a current Docker Compose plugin", command, flag)
				}
			}
		}
	}
	if dryRun {
		prefix = append(prefix, "--dry-run")
	}
	cmd := exec.Command(executable, append(prefix, args...)...) // #nosec G204 -- executable is allowlisted
	cmd.Env = environment
	return cmd, nil
}

// stdout and stderr may be copied concurrently by os/exec when streaming.
type composeOutput struct {
	buffer bytes.Buffer
	mu     sync.Mutex
}

func (output *composeOutput) Write(p []byte) (int, error) {
	output.mu.Lock()
	defer output.mu.Unlock()
	return output.buffer.Write(p)
}

func (output *composeOutput) String() string {
	output.mu.Lock()
	defer output.mu.Unlock()
	return output.buffer.String()
}

func ExecComposeCommand(setStdOut, dryRun bool, command string, args ...string) (string, error) {
	cmd, err := prepareComposeCommand(dryRun, command, args...)
	if err != nil {
		return "", err
	}
	var output composeOutput
	cmd.Stdout = &output
	cmd.Stderr = &output
	if setStdOut {
		cmd.Stdout = io.MultiWriter(os.Stdout, &output)
		cmd.Stderr = io.MultiWriter(os.Stderr, &output)
	}
	if err := cmd.Run(); err != nil {
		return output.String(), fmt.Errorf("%s failed: %w: %s", command, err, strings.TrimSpace(output.String()))
	}
	if setStdOut {
		return "", nil
	}
	return output.String(), nil
}
