package onboard

import (
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

	configPath, err := createConfigPath()
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

	relayHost, relayPort, err := parseURL(jc.RelayServerAddr)
	if err != nil {
		return err
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

		RelayServerURL:  jc.RelayServerAddr,
		RelayServerAddr: relayHost,
		RelayServerPort: relayPort,

		SIAAddr: jc.SIAAddr,
		PEAAddr: jc.PEAAddr,

		WorkerNode: jc.WorkerNode,

		ConfigPath: configPath,
	}

	// initialize sprig for templating
	sprigFuncs := sprig.GenericFuncMap()

	// write compose file
	composeFilePath, err := writeFile(configPath, "docker-compose.yaml", sprigFuncs, workerNodeComposeFileTemplate, jc.TCArgs)
	if err != nil {
		return err
	}

	// pull latest images
	_, err = ExecComposeCommand(true, jc.composeCmd, "-f", composeFilePath, "--profile", "kubearmor-only", "pull")
	if err != nil {
		return err
	}

	// run compose command
	_, err = ExecComposeCommand(true, jc.composeCmd, "-f", composeFilePath, "--profile", "kubearmor-only", "up", "-d")
	if err != nil {
		return err
	}

	return nil
}
