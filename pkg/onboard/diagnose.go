package onboard

import (
	"context"
	"fmt"
	"strings"

	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

const (
	kubeArmorInitName      = "kubearmor-init"
	kubeArmorName          = "kubearmor"
	kubeArmorRelayName     = "kubearmor-relay-server"
	kubearmorVMAdapterName = "kubearmor-vm-adapter"

	spireAgentName    = "spire-agent"
	waitForItName     = "wait-for-it"
	siaName           = "shared-informer-agent"
	peaName           = "policy-enforcement-agent"
	feederServiceName = "feeder-service"

	summaryEngineName  = "summary-engine"
	discoveryName      = "discovery"
	hardeningAgentName = "hardening-agent"
)

var (
	// for ordered maps
	workerNodePriorityList = []string{
		kubeArmorInitName,
		kubeArmorName,
		kubearmorVMAdapterName,
	}
	cpNodePriorityList = []string{
		spireAgentName,
		waitForItName,
		kubeArmorInitName,
		kubeArmorName,
		kubearmorVMAdapterName,
		kubeArmorRelayName,
		siaName,
		feederServiceName,
		peaName,
	}
)

func GetKnownContainerMap() map[string]dockerTypes.Container {
	knownContainerMap := make(map[string]dockerTypes.Container)

	knownContainerMap[kubeArmorInitName] = dockerTypes.Container{}
	knownContainerMap[kubeArmorName] = dockerTypes.Container{}
	knownContainerMap[kubeArmorRelayName] = dockerTypes.Container{}
	knownContainerMap[kubearmorVMAdapterName] = dockerTypes.Container{}
	knownContainerMap[spireAgentName] = dockerTypes.Container{}
	knownContainerMap[waitForItName] = dockerTypes.Container{}
	knownContainerMap[siaName] = dockerTypes.Container{}
	knownContainerMap[peaName] = dockerTypes.Container{}
	knownContainerMap[feederServiceName] = dockerTypes.Container{}

	knownContainerMap[summaryEngineName] = dockerTypes.Container{}
	knownContainerMap[discoveryName] = dockerTypes.Container{}
	knownContainerMap[hardeningAgentName] = dockerTypes.Container{}

	return knownContainerMap
}

// containerFailed checks container status
func containerFailed(containerJSON dockerTypes.ContainerJSON, containerName string) bool {

	if containerJSON.State != nil {
		switch containerJSON.State.Status {
		case "exited", "dead":
			// wait-for-it and kubearmor-init are expected to exit
			// they fail with non-zero exit codes
			if containerJSON.State.ExitCode != 0 {
				return true
			} else if containerName == waitForItName || containerName == kubeArmorInitName {
				return false
			} else {
				// all other exits are unexpected
				return true
			}
		}
	}

	return false
}

// getContainerDiagnosis returns a diagnosis, name of the failed container or an error
func getContainerDiagnosis(client *client.Client, knownContainerMap map[string]dockerTypes.Container, priorityList []string) (string, error) {
	var diagnosis, failedContainer string

	for _, container := range priorityList {
		containerJSON, err := client.ContainerInspect(context.Background(), knownContainerMap[container].ID)
		if err != nil {
			return "", err
		}

		if containerFailed(containerJSON, container) {
			failedContainer = container
			diagnosis = fmt.Sprintf("Container %s failed to run as", failedContainer)

			// enrich
			switch container {
			case spireAgentName, waitForItName:
				diagnosis = fmt.Sprintf("%s SPIRE was unhealthy", diagnosis)
				failedContainer = spireAgentName
			case kubeArmorInitName:
				diagnosis = fmt.Sprintf("%s it failed to compile KubeArmor BPF code", diagnosis)
			}

			diagnosis = fmt.Sprintf("%s. Please checkout logs for %s container.\nCOMMAND:\ndocker logs %s", diagnosis, failedContainer, failedContainer)

			return diagnosis, nil
		} else {
			continue
		}
	}

	return diagnosis, nil
}

func diagnose(nodeType NodeType) (string, error) {
	var (
		diagnosis string
	)

	dockerClient, err := CreateDockerClient()
	if err != nil {
		return diagnosis, fmt.Errorf("Failed to create docker client. %s", err.Error())
	}
	defer dockerClient.Close()

	containerList, err := dockerClient.ContainerList(context.Background(), dockerTypes.ContainerListOptions{
		All: true,
	})
	if err != nil {
		return diagnosis, fmt.Errorf("Failed to list containers. %s", err.Error())
	}

	knownContainerMap := GetKnownContainerMap()
	for _, containerInfo := range containerList {
		// only first index needed as name is specified in our templates
		// key exists, add value
		containerName := strings.TrimPrefix(containerInfo.Names[0], "/")
		if _, ok := knownContainerMap[containerName]; ok {
			knownContainerMap[containerName] = containerInfo
		}
	}

	var priorityList []string
	switch nodeType {
	case NodeType_ControlPlane:
		priorityList = cpNodePriorityList
	case NodeType_WorkerNode:
		priorityList = workerNodePriorityList
	}

	diagnosis, err = getContainerDiagnosis(dockerClient, knownContainerMap, priorityList)
	if err != nil {
		return diagnosis, fmt.Errorf("Failed to get container diagnosis. %s", err.Error())
	}

	return diagnosis, nil
}
