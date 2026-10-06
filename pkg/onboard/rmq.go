package onboard

import (
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
)

// prepareRMQCredentials prepares the broker account advertised to worker nodes.
func (ic *InitConfig) prepareRMQCredentials() error {
	credentials := ic.Tls.RMQCredentials
	if credentials == "" {
		credentials = ic.RMQCredentials
	}
	if credentials == "" && (ic.Tls.Enabled || ic.Tls.RMQEnabled) && (ic.DeployRMQ || ic.Mode == VMMode_Systemd) {
		secret := make([]byte, 24)
		if _, err := rand.Read(secret); err != nil {
			return fmt.Errorf("generating RabbitMQ password: %w", err)
		}
		credentials = Encode([]byte("accuknox:" + base64.RawURLEncoding.EncodeToString(secret)))
	}
	username, password, err := getRMQUserPass(credentials)
	if err != nil {
		return err
	}
	if (ic.Tls.Enabled || ic.Tls.RMQEnabled) && username == "guest" {
		return fmt.Errorf("RabbitMQ remote connections require a non-guest user; supply --auth=base64(username:password)")
	}
	ic.RMQCredentials = credentials
	ic.Tls.RMQCredentials = credentials
	ic.TCArgs.RMQUsername = username
	ic.TCArgs.RMQPassword = password
	if password != "" {
		ic.TCArgs.RMQPasswordHash = GetHash(password)
	}
	return nil
}

func (ic *InitConfig) controlPlaneRMQConnection(address string, hasAuth bool) (string, string, string) {
	if ic.Mode != VMMode_Systemd || hasAuth || !(ic.Tls.Enabled || ic.Tls.RMQEnabled) {
		return address, ic.TCArgs.RMQUsername, ic.TCArgs.RMQPassword
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil || !isLocalRMQHost(host) {
		return address, ic.TCArgs.RMQUsername, ic.TCArgs.RMQPassword
	}
	ip := net.ParseIP(host)
	if !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port), "guest", "guest"
}

func isLocalRMQHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip != nil && (ip.IsLoopback() || ip.IsUnspecified()) {
		return true
	}
	if hostname, err := os.Hostname(); err == nil && strings.EqualFold(host, hostname) {
		return true
	}
	addresses, err := net.InterfaceAddrs()
	if err != nil || ip == nil {
		return false
	}
	for _, address := range addresses {
		if local, _, err := net.ParseCIDR(address.String()); err == nil && ip.Equal(local) {
			return true
		}
	}
	return false
}

//go:embed templates/rabbitmq-setup.sh
var rmqSetupShell string

// Generate self-contained scripts; execution is left to the user on the broker host.
func (ic *InitConfig) saveSystemdRMQConfiguration() error {
	quote := func(value string) string {
		return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
	}
	port := "5672"
	if _, externalPort, err := net.SplitHostPort(ic.RMQServer); err == nil {
		port = externalPort
	}

	workerUsername, workerPassword, err := getRMQUserPass(ic.RMQCredentials)
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("locating RabbitMQ setup directory: %w", err)
	}
	script := "#!/usr/bin/env bash\nset -euo pipefail\n" +
		"export RMQ_USERNAME=" + quote(workerUsername) + "\n" +
		"export RMQ_PASSWORD=" + quote(workerPassword) + "\n" +
		"export RMQ_AUTH=" + quote(ic.RMQCredentials) + "\n" +
		"export RMQ_TLS=" + quote(strconv.FormatBool(ic.Tls.Enabled)) + "\n" +
		"export RMQ_PORT=" + quote(port) + "\n" +
		rmqSetupShell
	if err := saveRMQInstructions(cwd, "rabbitmq-setup.sh", script); err != nil {
		return err
	}
	ic.rmqSetupDir = cwd
	return nil
}

// PrintRMQSetupScripts reports generated scripts after onboarding succeeds.
func (ic *InitConfig) PrintRMQSetupScripts() {
	if ic.rmqSetupDir == "" {
		return
	}
	logger.Print("RabbitMQ setup script saved in %s/rabbitmq-setup.sh. Make it executable, then run it on the RabbitMQ host with your installation mode:\n\nsudo chmod u+x rabbitmq-setup.sh\n ./rabbitmq-setup.sh docker\n ./rabbitmq-setup.sh docker-compose\n ./rabbitmq-setup.sh systemd", ic.rmqSetupDir)
}

// Keep credentials private, but let the invoking user read files after sudo.
func saveRMQInstructions(dir, name, content string) error {
	file, err := os.CreateTemp(dir, ".rabbitmq-setup-*")
	if err != nil {
		return fmt.Errorf("creating RabbitMQ instructions: %w", err)
	}
	defer os.Remove(file.Name())
	if os.Geteuid() == 0 && os.Getenv("SUDO_UID") != "" {
		uid, uidErr := strconv.Atoi(os.Getenv("SUDO_UID"))
		gid, gidErr := strconv.Atoi(os.Getenv("SUDO_GID"))
		if uidErr != nil || gidErr != nil || uid < 0 || gid < 0 {
			file.Close()
			return fmt.Errorf("invalid invoking user IDs for RabbitMQ instructions")
		}
		if err := file.Chown(uid, gid); err != nil {
			file.Close()
			return fmt.Errorf("setting RabbitMQ instructions owner: %w", err)
		}
	}
	if _, err := file.WriteString(content); err != nil {
		file.Close()
		return fmt.Errorf("writing RabbitMQ instructions: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("closing RabbitMQ instructions: %w", err)
	}
	if err := os.Rename(file.Name(), filepath.Join(dir, name)); err != nil {
		return fmt.Errorf("saving RabbitMQ instructions: %w", err)
	}
	return nil
}
