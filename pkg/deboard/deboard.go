package deboard

import (
	"fmt"
	"os"
	"path/filepath"

	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
)

func Deboard(nodeType onboard.NodeType, vmMode onboard.VMMode, dryRun bool) (string, error) {

	// check for systemd installation
	switch vmMode {
	case onboard.VMMode_Systemd:
		fmt.Println("Deboarding VM....")
		err := onboard.DeboardSystemd(nodeType)
		return "", err

	case onboard.VMMode_Docker:
		configPath, err := cm.GetDefaultConfigPath()
		if err != nil {
			return "", err
		}

		composeFilePath := filepath.Join(configPath, "docker-compose.yaml")
		_, err = os.Stat(composeFilePath)
		if err != nil {
			return configPath, err
		}

		composeCmd, composeVersion := onboard.GetComposeCommand()
		fmt.Printf("Using %s version %s\n", composeCmd, composeVersion)

		switch nodeType {
		case onboard.NodeType_ControlPlane:
			_, err = onboard.ExecComposeCommand(true, dryRun, composeCmd,
				"-f", composeFilePath, "--profile", "spire-agent",
				"--profile", "kubearmor", "--profile", "accuknox-agents", "down",
				"--volumes")
		case onboard.NodeType_WorkerNode:
			_, err = onboard.ExecComposeCommand(true, dryRun, composeCmd,
				"-f", composeFilePath, "--profile", "kubearmor", "down",
				"--volumes")
		}
		if err != nil {
			return configPath, fmt.Errorf("error: %s", err.Error())
		}

		err = os.RemoveAll(configPath)
		if err != nil {
			return configPath, err
		}

		return configPath, nil

	}
	return "", nil
}
