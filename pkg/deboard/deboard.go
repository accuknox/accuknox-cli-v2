package deboard

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	dockerTypes "github.com/docker/docker/api/types"
	dockerContainerTypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/volume"
	"github.com/fatih/color"
)

func Deboard(nodeType onboard.NodeType, vmMode onboard.VMMode, dryRun bool) (string, error) {
	fmt.Println(color.MagentaString("Deboarding VM in mode %s...", vmMode))

	// check for systemd installation
	switch vmMode {
	case onboard.VMMode_Systemd:
		err := onboard.DeboardSystemd(nodeType)
		return "", err

	case onboard.VMMode_Docker:
		configPath, err := cm.GetDefaultConfigPath()
		if err != nil {
			return "", err
		}

		verifyInstallation := false
		composeFilePath := filepath.Join(configPath, "docker-compose.yaml")
		_, err = os.Stat(composeFilePath)
		if err != nil && os.IsNotExist(err) {
			// for handling cases when users might have deleted the docker compose file
			// but agent containers are left running
			verifyInstallation = true
		} else if err != nil {
			return configPath, err
		}

		if verifyInstallation {
			fmt.Println(color.YellowString("Docker compose file not found at %s. Checking installation of each agent...", composeFilePath))
			installedContainers, installedVolumes, err := getInstalledObjects()
			if err != nil {
				return "", err
			}

			err = removeInstalledObjects(installedContainers, installedVolumes)
			if err != nil {
				return "", err
			}

		} else {
			composeCmd, composeVersion, err := onboard.GetComposeCommand()
			if err != nil {
				return configPath, err
			}
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
		}

		err = os.RemoveAll(configPath)
		if err != nil && !os.IsNotExist(err) {
			return configPath, err
		}

		return configPath, nil
	}

	return "", nil
}

// returns installed containers and volumes
func getInstalledObjects() (map[string]dockerTypes.Container, []string, error) {
	allContainers := onboard.GetKnownContainerMap()
	installedContainers := make(map[string]dockerTypes.Container, 0)

	dockerClient, err := onboard.CreateDockerClient()
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create docker client. %s", err.Error())
	}

	containerList, err := dockerClient.ContainerList(context.Background(), dockerTypes.ContainerListOptions{
		All: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to list containers. %s", err.Error())
	}

	for _, container := range containerList {
		containerName := strings.TrimPrefix(container.Names[0], "/")
		if _, ok := allContainers[containerName]; ok {
			installedContainers[containerName] = container
		}
	}

	allVolumes := map[string]struct{}{
		"spire-vol":          {},
		"kubearmor-init-vol": {},
	}
	installedVolumes := make([]string, 0)

	volumeList, err := dockerClient.VolumeList(context.Background(), volume.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to list volumes. %s", err.Error())
	}

	for _, volume := range volumeList.Volumes {
		if _, ok := allVolumes[volume.Name]; ok {
			installedVolumes = append(installedVolumes, volume.Name)
		}
	}

	return installedContainers, installedVolumes, nil
}

func removeInstalledObjects(installedContainers map[string]dockerTypes.Container, installedVolumes []string) error {
	dockerClient, err := onboard.CreateDockerClient()
	if err != nil {
		return fmt.Errorf("Failed to create docker client. %s", err.Error())
	}

	for _, container := range installedContainers {
		containerName := strings.TrimPrefix(container.Names[0], "/")
		fmt.Printf("Stopping container %s...\n", containerName)

		err := dockerClient.ContainerStop(context.Background(), container.ID, dockerContainerTypes.StopOptions{})
		if err != nil {
			fmt.Println(color.YellowString("Failed to stop container %s: %s", containerName, err.Error()))
		}

		fmt.Printf("Removing container %s...\n", containerName)
		err = dockerClient.ContainerRemove(context.Background(), container.ID, dockerTypes.ContainerRemoveOptions{})
		if err != nil {
			fmt.Println(color.YellowString("Failed to remove container %s: %s", containerName, err.Error()))
		}
	}

	for _, volume := range installedVolumes {
		fmt.Printf("Removing volume %s...\n", volume)
		if err := dockerClient.VolumeRemove(context.Background(), volume, true); err != nil {
			fmt.Println(color.YellowString("Failed to remove volume %s: %s", volume, err.Error()))
		}
	}

	return nil
}
