package privilege

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/term"
)

// BootstrapCommand is an internal entry point for privileged command execution.
const BootstrapCommand = "__elevated"
const maxRequestSize = 4 * 1024 * 1024

// Only configuration and connection settings are restored after sudo. Loader,
// shell, PATH, and privilege-related environment variables remain sudo-managed.
var contextVariables = []string{
	"HOME", "XDG_CONFIG_HOME", "KUBECONFIG", "KUBEARMOR_SERVICE", "DISCOVERY_SERVICE",
	"OMNI_ARTIFACT_API_TOKEN", "DOCKER_CONFIG", "DOCKER_HOST", "DOCKER_API_VERSION",
	"DOCKER_CERT_PATH", "DOCKER_TLS_VERIFY", "DOCKER_CONTEXT",
	"TERM", "COLORTERM", "NO_COLOR", "CLICOLOR", "COMPOSE_ANSI", "COMPOSE_PROGRESS", "COMPOSE_STATUS_STDOUT",
	"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy",
}

type Request struct {
	Arguments     []string
	Environment   map[string]string
	TerminalInput bool
}

func NewRequest(args []string) Request {
	r := Request{Arguments: append([]string{}, args...), Environment: make(map[string]string)}
	for _, name := range contextVariables {
		if value, ok := os.LookupEnv(name); ok {
			r.Environment[name] = value
		}
	}
	return r
}

// Run asks sudo for authorization and executes the same binary as root. The
// original arguments and credentials travel over stdin rather than sudo argv.
// A length-delimited frame preserves piped input. Interactive input is
// reattached to the terminal in the child, so process exit never waits on Enter.
func Run(ctx context.Context, executable string, args []string, input io.Reader, output, stderr io.Writer) error {
	if output == nil {
		output = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	sudo, err := exec.LookPath("sudo")
	if err != nil {
		return fmt.Errorf("administrator privileges are required; install sudo or rerun Knoxctl from an administrator/root terminal")
	}
	if err := authorize(ctx, sudo, stderr); err != nil {
		return err
	}
	cmd, err := command(ctx, sudo, executable, NewRequest(args), input)
	if err != nil {
		return err
	}
	cmd.Stdout, cmd.Stderr = output, stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("elevated Knoxctl command failed: %w", err)
	}
	return nil
}

func authorize(ctx context.Context, sudo string, stderr io.Writer) error {
	if exec.CommandContext(ctx, sudo, "-n", "-v").Run() == nil {
		fmt.Fprintln(stderr, "This operation needs administrator privileges; using your existing sudo authorization.")
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	fmt.Fprintln(stderr, "This operation needs administrator privileges. sudo will ask for your password if required.")
	cmd := exec.CommandContext(ctx, sudo, "-v")
	cmd.Stdout, cmd.Stderr = stderr, stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("administrator authorization failed: %w; authorize sudo or rerun Knoxctl as root", err)
	}
	return nil
}

func command(ctx context.Context, sudo, executable string, request Request, input io.Reader) (*exec.Cmd, error) {
	if file, ok := input.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		request.TerminalInput = true
	}
	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	if len(data) > maxRequestSize {
		return nil, fmt.Errorf("elevation request is too large")
	}
	var frame bytes.Buffer
	if err := binary.Write(&frame, binary.BigEndian, uint32(len(data))); err != nil {
		return nil, err
	}
	frame.Write(data)
	cmd := exec.CommandContext(ctx, sudo, "--", executable, BootstrapCommand)
	if request.TerminalInput {
		// Reading a terminal after the frame makes os/exec's stdin-copy
		// goroutine block even after the child exits. Send only the finite
		// frame; the elevated process reconnects its stdin to /dev/tty.
		cmd.Stdin = &frame
		return cmd, nil
	}
	if input == nil {
		input = bytes.NewReader(nil)
	}
	cmd.Stdin = io.MultiReader(&frame, input)
	return cmd, nil
}

func ReadRequest(input io.Reader) (Request, error) {
	var request Request
	var size uint32
	if err := binary.Read(input, binary.BigEndian, &size); err != nil {
		return request, fmt.Errorf("reading elevation request: %w", err)
	}
	if size == 0 || size > maxRequestSize {
		return request, fmt.Errorf("invalid elevation request size")
	}
	data := make([]byte, int(size))
	if _, err := io.ReadFull(input, data); err != nil {
		return request, err
	}
	if err := json.Unmarshal(data, &request); err != nil {
		return request, err
	}
	if len(request.Arguments) == 0 {
		return request, fmt.Errorf("elevation request has no command")
	}
	for name := range request.Environment {
		allowed := false
		for _, key := range contextVariables {
			if name == key {
				allowed = true
				break
			}
		}
		if !allowed {
			return request, fmt.Errorf("unsupported elevation environment variable %q", name)
		}
	}
	return request, nil
}

func (r Request) RestoreEnvironment() error {
	for _, name := range contextVariables {
		if value, ok := r.Environment[name]; ok {
			if err := os.Setenv(name, value); err != nil {
				return err
			}
		} else if err := os.Unsetenv(name); err != nil {
			return err
		}
	}
	return nil
}

// IsPermissionError distinguishes local OS/Docker access errors from remote
// authorization failures, which sudo cannot resolve.
func IsPermissionError(err error) bool {
	if errors.Is(err, os.ErrPermission) {
		return true
	}
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	var exitError *exec.ExitError
	localProcess := errors.As(err, &exitError)
	if localProcess {
		message += " " + strings.ToLower(string(exitError.Stderr))
	}
	denied := strings.Contains(message, "permission denied") || strings.Contains(message, "operation not permitted")
	if denied {
		for _, marker := range []string{"open /", "mkdir /", "chmod /", "chown /", "remove /", "unlink /", "unix://", "docker.sock"} {
			if strings.Contains(message, marker) {
				return true
			}
		}
	}
	return localProcess && (denied || strings.Contains(message, "interactive authentication required"))
}
