package deboard

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	dockerTypes "github.com/docker/docker/api/types"
	dockerContainerTypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
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
			installedContainers, installedVolumes, err := GetInstalledObjects()
			if err != nil {
				return "", err
			}

			err = removeInstalledObjects(installedContainers, installedVolumes, nil)
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
				containerOrder := []string{"kubearmor", "kubearmor-vm-adapter"}
				// Remove kubearmor, VMA and then delete/down all other containers
				containers, err := getContainerObjects(containerOrder)
				if err != nil {
					logger.Warn("error:%s", err.Error())
				}
				if len(containers) > 0 {
					err = removeInstalledObjects(containers, nil, containerOrder)
					if err != nil {
						fmt.Println("error", err.Error())
					}
				}
				_, err = onboard.ExecComposeCommand(true, dryRun, composeCmd,
					"-f", composeFilePath, "--profile", "spire-agent",
					"--profile", "kubearmor", "--profile", "accuknox-agents", "down",
					"--volumes")
			case onboard.NodeType_WorkerNode:
				_, err = onboard.ExecComposeCommand(true, dryRun, composeCmd,
					"-f", composeFilePath, "--profile", "kubearmor", "--profile", "accuknox-agents", "--profile", "spire-agent", "down",
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
func GetInstalledObjects() (map[string]dockerContainerTypes.Summary, []string, error) {
	allContainers := onboard.GetKnownContainerMap()
	installedContainers := make(map[string]dockerContainerTypes.Summary, 0)

	dockerClient, err := onboard.CreateDockerClient()
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create docker client. %s", err.Error())
	}

	containerList, err := dockerClient.ContainerList(context.Background(), dockerContainerTypes.ListOptions{
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

func removeInstalledObjects(installedContainers map[string]dockerTypes.Container, installedVolumes, orderedContainerNames []string) error {
	dockerClient, err := onboard.CreateDockerClient()
	if err != nil {
		return fmt.Errorf("Failed to create docker client. %s", err.Error())
	}

	for _, name := range orderedContainerNames {
		if container, ok := installedContainers[name]; ok {
			removeContainer(dockerClient, container.Names[0], container.ID)
			delete(installedContainers, name)
		}
	}

	for _, container := range installedContainers {
		removeContainer(dockerClient, container.Names[0], container.ID)
	}
	for _, volume := range installedVolumes {
		fmt.Printf("Removing volume %s...\n", volume)
		if err := dockerClient.VolumeRemove(context.Background(), volume, true); err != nil {
			fmt.Println(color.YellowString("Failed to remove volume %s: %s", volume, err.Error()))
		}
	}

	return nil
}

func removeContainer(dockerClient *client.Client, containerName string, containerID string) {
	containerName = strings.TrimPrefix(containerName, "/")
	fmt.Printf("Stopping container %s...\n", containerName)
	err := dockerClient.ContainerStop(context.Background(), containerID, dockerContainerTypes.StopOptions{})
	if err != nil {
		fmt.Println(color.YellowString("Failed to stop container %s: %s", containerName, err.Error()))
	}

	fmt.Printf("Removing container %s...\n", containerName)
	err = dockerClient.ContainerRemove(context.Background(), containerID, dockerContainerTypes.RemoveOptions{})
	if err != nil {
		fmt.Println(color.YellowString("Failed to remove container %s: %s", containerName, err.Error()))
	}
}

func UninstallRRA() error {
	//check for RRA systemd installation

	exists, err := onboard.CheckRRASystemdInstallation()
	if err != nil {
		fmt.Println(color.RedString("error checking RRA systemd installation"))
	}
	if exists {
		fmt.Println(color.BlueString("RRA found running in systemd mode"))
		rraFiles := []string{"accuknox-rra.service", "accuknox-rra.timer"}
		for _, file := range rraFiles {
			err := onboard.StopSystemdService(file, false, true)
			if err != nil {
				logger.Error("error stopping %s: %s\n", file, err)
				return err
			}
			onboard.Deletedir(cm.RRAPath)
		}
		return err
	}

	if err = deleteContainer(cm.RRA); err != nil {
		return err
	}

	return os.ErrNotExist
}

func UninstallImagescan() error {
	//check for imagescan systemd installation

	exists, err := onboard.CheckImagescanSystemdInstallation()
	if err != nil {
		fmt.Println(color.RedString("error checking imagescan systemd installation"))
	}

	// Delete systemd service
	if exists {
		fmt.Println(color.BlueString("Image scanner found running in systemd mode"))
		imagescanFiles := []string{cm.Imagescan + ".service", cm.Imagescan + ".timer"}
		for _, file := range imagescanFiles {
			err := onboard.StopSystemdService(file, false, true)
			if err != nil {
				logger.Error("error stopping %s: %s\n", file, err)
				return err
			}
			onboard.Deletedir(cm.ImageScanConfigPath)
		}
		return nil
	}

	// Delete docker service
	if err = deleteContainer(cm.Imagescan); err != nil {
		return err
	}

	return os.ErrNotExist
}

func deleteContainer(serviceName string) error {
	var cc onboard.ClusterConfig

	// validate docker environment
	_, err := cc.ValidateEnv()
	if err != nil {
		return os.ErrNotExist
	}

	// check for docker installation for the provided service name
	containerObj, err := getContainerObjects([]string{serviceName})
	if err != nil {
		logger.Warn("error:%s", err.Error())
	}

	// No containers found for the provided service name prefix
	if len(containerObj) == 0 {
		return os.ErrNotExist // exit
	}

	// logic for deleting the docker container
	fmt.Println(color.BlueString("%s docker installation found", serviceName))

	configPath, err := cm.GetDefaultConfigPath()
	if err != nil {
		return err
	}

	composeFilePath := filepath.Join(configPath, fmt.Sprintf("docker-compose_%s.yaml", serviceName))
	_, err = os.Stat(composeFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			err = removeInstalledObjects(containerObj, nil, nil)
			if err != nil {
				fmt.Println("error", err.Error())
			}
			return err
		} else {
			return err
		}
	}

	composeCmd, _, err := onboard.GetComposeCommand()
	if err != nil {
		return err
	}

	if _, err = onboard.ExecComposeCommand(true, false, composeCmd,
		"-f", composeFilePath, "--profile", "accuknox-agents", "down"); err != nil {
		return fmt.Errorf("error: %s", err.Error())
	}

	if err = os.Remove(composeFilePath); err != nil && !os.IsNotExist(err) {
		return err
	}

	// delete configdir if it is empty(for cases if only RRA is installed)
	if err = os.Remove(configPath); err != nil {
		if !os.IsNotExist(err) && !errors.Is(err, syscall.ENOTEMPTY) && !errors.Is(err, syscall.EEXIST) {
			return err
		}
	}

	return nil
}

func getContainerObjects(containerNames []string) (map[string]dockerTypes.Container, error) {
	installedContainers := make(map[string]dockerTypes.Container, 0)
	dockerClient, err := onboard.CreateDockerClient()
	if err != nil {
		return nil, fmt.Errorf("Failed to create docker client. %s", err.Error())
	}
	containerList, err := dockerClient.ContainerList(context.Background(), dockerContainerTypes.ListOptions{
		All: true,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to list containers. %s", err.Error())
	}

	for _, container := range containerList {
		containerName := strings.TrimPrefix(container.Names[0], "/")
		if slices.Contains(containerNames, containerName) {
			installedContainers[containerName] = container
		}
	}

	return installedContainers, nil
}
