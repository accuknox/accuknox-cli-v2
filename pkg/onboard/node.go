package onboard

import (
	"fmt"
	"os"

	"github.com/Masterminds/sprig"
	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	"golang.org/x/mod/semver"
)

func JoinClusterConfig(cc ClusterConfig, kubeArmorAddr, relayServerAddr, siaAddr, peaAddr string) *JoinConfig {
	return &JoinConfig{
		ClusterConfig:   cc,
		KubeArmorAddr:   kubeArmorAddr,
		RelayServerAddr: relayServerAddr,
		SIAAddr:         siaAddr,
		PEAAddr:         peaAddr,
	}
}
func (jc *JoinConfig) CreateBaseNodeConfig() error {
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	kubeArmorURL := "localhost:32767"
	kubeArmorPort := "32767"
	if jc.KubeArmorAddr != "" {
		kubeArmorURL = jc.KubeArmorAddr
		_, kubeArmorPort, err = parseURL(kubeArmorURL)
		if err != nil {
			return err
		}
	}

	// parse URL and assign default values as needed
	var relayHost, relayPort, relayAddr string
	if jc.RelayServerAddr != "" {
		relayAddr = jc.RelayServerAddr
		relayHost, relayPort, err = parseURL(jc.RelayServerAddr)
		if err != nil {
			return err
		}
	} else if jc.CPNodeAddr != "" {
		relayHost = jc.CPNodeAddr
		relayPort = "32768"
		relayAddr = jc.CPNodeAddr + ":" + relayPort
	} else {
		return fmt.Errorf("Relay server address cannot be empty")
	}

	var siaAddr string
	if jc.SIAAddr != "" {
		siaAddr = jc.SIAAddr
	} else if siaAddr == "" && jc.CPNodeAddr != "" {
		siaAddr = jc.CPNodeAddr + ":" + "32769"
	} else {
		return fmt.Errorf("SIA address cannot be empty")
	}

	var peaAddr string
	if jc.PEAAddr != "" {
		peaAddr = jc.PEAAddr
	} else if peaAddr == "" && jc.CPNodeAddr != "" {
		peaAddr = jc.CPNodeAddr + ":" + "32770"
	} else {
		return fmt.Errorf("PEA address cannot be empty")
	}
	jc.TCArgs = TemplateConfigArgs{
		Hostname: hostname,

		// for vm-adapter
		KubeArmorURL:  kubeArmorURL,
		KubeArmorPort: kubeArmorPort,

		RelayServerURL:  relayAddr,
		RelayServerAddr: relayHost,
		RelayServerPort: relayPort,

		SIAAddr: siaAddr,
		PEAAddr: peaAddr,

		WorkerNode: jc.WorkerNode,
		// kubearmor config
		KubeArmorVisibility:     jc.Visibility,
		KubeArmorHostVisibility: jc.HostVisibility,

		KubeArmorFilePosture:    jc.DefaultFilePosture,
		KubeArmorNetworkPosture: jc.DefaultNetworkPosture,
		KubeArmorCapPosture:     jc.DefaultCapPosture,

		KubeArmorHostFilePosture:    jc.DefaultHostFilePosture,
		KubeArmorHostNetworkPosture: jc.DefaultHostNetworkPosture,
		KubeArmorHostCapPosture:     jc.DefaultHostCapPosture,
		NetworkCIDR:                 jc.CIDR,
		VmMode:                      jc.Mode,
		SecureContainers:            jc.SecureContainers,
	}
	return nil
}

func (jc *JoinConfig) JoinWorkerNode() error {
	// validate this environment
	dockerStatus, err := jc.ValidateEnv()
	if err != nil {
		return err
	}
	fmt.Println(dockerStatus)

	configPath, err := createDefaultConfigPath()
	if err != nil {
		return err
	}
	// configs specific to docker mode of installation

	jc.TCArgs.KubeArmorImage = jc.KubeArmorImage
	jc.TCArgs.KubeArmorInitImage = jc.KubeArmorInitImage
	jc.TCArgs.KubeArmorVMAdapterImage = jc.KubeArmorVMAdapterImage
	jc.TCArgs.ImagePullPolicy = string(jc.ImagePullPolicy)
	jc.TCArgs.ConfigPath = configPath

	// initialize sprig for templating
	sprigFuncs := sprig.GenericFuncMap()

	// write compose file
	composeFilePath, err := copyOrGenerateFile(jc.UserConfigPath, configPath, "docker-compose.yaml", sprigFuncs, workerNodeComposeFileTemplate, jc.TCArgs)
	if err != nil {
		return err
	}

	diagnosis := true
	args := []string{"-f", composeFilePath, "--profile", "kubearmor-only", "up", "-d"}

	// need these flags for diagnosis
	if semver.Compare(jc.composeVersion, common.MinDockerComposeWithWaitSupported) >= 0 {
		args = append(args, "--wait", "--wait-timeout", "60")
	} else {
		diagnosis = false
	}

	// run compose command
	_, err = ExecComposeCommand(true, jc.DryRun, jc.composeCmd, args...)
	if err != nil {
		// cleanup volumes
		_, volDelErr := ExecDockerCommand(true, false, "docker", "volume", "rm", "kubearmor-init-vol")
		if volDelErr != nil {
			fmt.Println("Error while removing volumes:", volDelErr.Error())
		}

		if diagnosis {
			diagnosisResult, diagErr := diaganose(NodeType_WorkerNode)
			if diagErr != nil {
				diagnosisResult = diagErr.Error()
			}
			return fmt.Errorf("Error: %s.\n\nDIAGNOSIS:\n%s", err.Error(), diagnosisResult)
		}

		return err
	}

	return nil
}
