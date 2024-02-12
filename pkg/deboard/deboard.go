package deboard

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
)

func Deboard(nodeType onboard.NodeType, dryRun bool) (string, error) {
	userHomedir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configPath := filepath.Join(userHomedir, ".accuknox")
	composeFilePath := filepath.Join(configPath, "docker-compose.yaml")
	_, err = os.Stat(composeFilePath)
	if err != nil {
		return configPath, err
	}

	composeCmd := onboard.GetComposeCommand()
	switch nodeType {
	case onboard.NodeType_ControlPlane:
		_, err = onboard.ExecComposeCommand(true, dryRun, composeCmd,
			"-f", composeFilePath, "--profile", "spire-agent",
			"--profile", "kubearmor", "--profile", "accuknox-agents", "down")
	case onboard.NodeType_WorkerNode:
		_, err = onboard.ExecComposeCommand(true, dryRun, composeCmd,
			"-f", composeFilePath, "--profile", "kubearmor", "down")
	}
	if err != nil {
		return configPath, fmt.Errorf("Error: %s", err.Error())
	}

	err = os.RemoveAll(configPath)
	if err != nil {
		return configPath, err
	}

	return configPath, nil
}
