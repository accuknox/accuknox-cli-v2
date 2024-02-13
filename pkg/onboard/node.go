package onboard

import (
	"fmt"
	"os"

	"github.com/Masterminds/sprig"
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

func (jc *JoinConfig) JoinWorkerNode() error {
	// validate this environment
	err := jc.validateEnv()
	if err != nil {
		return err
	}

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	configPath, err := createDefaultConfigPath()
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
		//KubeArmorVersion: kubeArmorVersion,
		KubeArmorImage:          jc.KubeArmorImage,
		KubeArmorInitImage:      jc.KubeArmorInitImage,
		KubeArmorVMAdapterImage: jc.KubeArmorVMAdapterImage,

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

		ConfigPath: configPath,
	}

	// initialize sprig for templating
	sprigFuncs := sprig.GenericFuncMap()

	// write compose file
	composeFilePath, err := copyOrGenerateFile(jc.UserConfigPath, configPath, "docker-compose.yaml", sprigFuncs, workerNodeComposeFileTemplate, jc.TCArgs)
	if err != nil {
		return err
	}

	// pull latest images
	_, err = ExecComposeCommand(true, jc.DryRun, jc.composeCmd, "-f", composeFilePath, "--profile", "kubearmor-only", "pull")
	if err != nil {
		return err
	}

	// run compose command
	_, err = ExecComposeCommand(true, jc.DryRun, jc.composeCmd, "-f", composeFilePath, "--profile", "kubearmor-only", "up", "-d")
	if err != nil {
		return err
	}

	return nil
}
