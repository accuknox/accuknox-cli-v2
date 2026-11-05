package deboard

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/accuknox/accuknox-cli-v2/pkg/privilege"
	"github.com/fatih/color"
	dockerContainerTypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

func Deboard(nodeType onboard.NodeType, vmMode onboard.VMMode, dryRun bool) (string, error) {
	fmt.Println(color.MagentaString("Deboarding VM in mode %s...", vmMode))
	if dryRun {
		fmt.Println("Dry run: no installed resources will be removed.")
		return "", nil
	}

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
	defer dockerClient.Close()

	containerList, err := dockerClient.ContainerList(context.Background(), client.ContainerListOptions{
		All: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to list containers. %s", err.Error())
	}

	for _, container := range containerList.Items {
		if len(container.Names) <= 0 {
			continue
		}
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

	volumeList, err := dockerClient.VolumeList(context.Background(), client.VolumeListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to list volumes. %s", err.Error())
	}

	for _, volume := range volumeList.Items {
		if _, ok := allVolumes[volume.Name]; ok {
			installedVolumes = append(installedVolumes, volume.Name)
		}
	}

	return installedContainers, installedVolumes, nil
}

func removeInstalledObjects(installedContainers map[string]dockerContainerTypes.Summary, installedVolumes, orderedContainerNames []string) error {
	dockerClient, err := onboard.CreateDockerClient()
	if err != nil {
		return fmt.Errorf("Failed to create docker client. %s", err.Error())
	}
	defer dockerClient.Close()

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
		if _, err := dockerClient.VolumeRemove(context.Background(), volume, client.VolumeRemoveOptions{Force: true}); err != nil {
			fmt.Println(color.YellowString("Failed to remove volume %s: %s", volume, err.Error()))
		}
	}

	return nil
}

func removeContainer(dockerClient *client.Client, containerName string, containerID string) {
	containerName = strings.TrimPrefix(containerName, "/")
	fmt.Printf("Stopping container %s...\n", containerName)
	_, err := dockerClient.ContainerStop(context.Background(), containerID, client.ContainerStopOptions{})
	if err != nil {
		fmt.Println(color.YellowString("Failed to stop container %s: %s", containerName, err.Error()))
	}

	fmt.Printf("Removing container %s...\n", containerName)
	_, err = dockerClient.ContainerRemove(context.Background(), containerID, client.ContainerRemoveOptions{})
	if err != nil {
		fmt.Println(color.YellowString("Failed to remove container %s: %s", containerName, err.Error()))
	}
}

func UninstallRRA() error {
	//check for RRA systemd installation

	exists, err := onboard.CheckRRASystemdInstallation()
	if err != nil {
		return fmt.Errorf("checking RRA systemd installation: %w", err)
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

	return deleteContainer(cm.RRA)
}

func UninstallImagescan() error {
	//check for imagescan systemd installation

	exists, err := onboard.CheckImagescanSystemdInstallation()
	if err != nil {
		return fmt.Errorf("checking image scanner systemd installation: %w", err)
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
	return deleteContainer(cm.Imagescan)
}

func deleteContainer(serviceName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cli, err := onboard.CreateDockerClient()
	if err != nil {
		return err
	}
	defer cli.Close()
	err = deleteDockerScanner(ctx, cli, serviceName)
	removed := err == nil
	if err != nil && !os.IsNotExist(err) {
		if client.IsErrConnectionFailed(err) && !privilege.IsPermissionError(err) {
			return os.ErrNotExist
		}
		return err
	}
	configPath, err := cm.GetDefaultConfigPath()
	if err != nil {
		return err
	}
	composeFilePath := filepath.Join(configPath, fmt.Sprintf("docker-compose_%s.yaml", serviceName))
	if err := os.Remove(composeFilePath); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		removed = true
	}
	if !removed {
		return os.ErrNotExist
	}
	if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) && !isDirNotEmpty(err) {
		return err
	}
	return nil
}

type scannerDockerClient interface {
	ContainerList(context.Context, client.ContainerListOptions) (client.ContainerListResult, error)
	ContainerStop(context.Context, string, client.ContainerStopOptions) (client.ContainerStopResult, error)
	ContainerRemove(context.Context, string, client.ContainerRemoveOptions) (client.ContainerRemoveResult, error)
}

// Remove only the named scanner. Compose down would also act on shared
// services/networks defined in the scanner's Compose project.
func deleteDockerScanner(ctx context.Context, cli scannerDockerClient, name string) error {
	result, err := cli.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return fmt.Errorf("listing Docker scanners: %w", err)
	}
	found := false
	var failures []error
	for _, item := range result.Items {
		matched := false
		for _, candidate := range item.Names {
			if strings.TrimPrefix(candidate, "/") == name {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		found = true
		if item.State == dockerContainerTypes.StateRunning {
			if _, err := cli.ContainerStop(ctx, item.ID, client.ContainerStopOptions{}); err != nil {
				failures = append(failures, fmt.Errorf("stopping %s: %w", name, err))
			}
		}
		if _, err := cli.ContainerRemove(ctx, item.ID, client.ContainerRemoveOptions{}); err != nil {
			failures = append(failures, fmt.Errorf("removing %s: %w", name, err))
		}
	}
	if !found {
		return os.ErrNotExist
	}
	return errors.Join(failures...)
}

func getContainerObjects(containerNames []string) (map[string]dockerContainerTypes.Summary, error) {
	installedContainers := make(map[string]dockerContainerTypes.Summary, 0)
	dockerClient, err := onboard.CreateDockerClient()
	if err != nil {
		return nil, fmt.Errorf("Failed to create docker client. %s", err.Error())
	}
	defer dockerClient.Close()
	containerList, err := dockerClient.ContainerList(context.Background(), client.ContainerListOptions{
		All: true,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to list containers. %s", err.Error())
	}

	for _, container := range containerList.Items {
		containerName := strings.TrimPrefix(container.Names[0], "/")
		if slices.Contains(containerNames, containerName) {
			installedContainers[containerName] = container
		}
	}

	return installedContainers, nil
}
