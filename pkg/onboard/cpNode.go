package onboard

import (
	"fmt"
	"os"
	"strings"

	"github.com/Masterminds/sprig"
)

func InitCPNodeConfig(cc ClusterConfig, joinToken, spireHost, ppsHost, knoxGateway, spireTrustBundle string) *InitConfig {
	return &InitConfig{
		ClusterConfig: cc,
		JoinToken:     joinToken,
		SpireHost:     spireHost,
		PPSHost:       ppsHost,
		KnoxGateway:   knoxGateway,

		SpireTrustBundleURL: spireTrustBundle,
	}
}

func (ic *InitConfig) InitializeControlPlane() error {
	// validate this environment
	err := ic.validateEnv()
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

	spireHost, spirePort, err := parseURL(ic.SpireHost)
	if spirePort == "80" {
		// default spire port
		spirePort = "8081"
	}

	// currently unused as we use insecure bootstrap
	var spireTrustBundleURL = ic.SpireTrustBundleURL
	if spireTrustBundleURL == "" {
		if strings.Contains(ic.SpireHost, "spire.dev.accuknox.com") {
			spireTrustBundleURL = spireTrustBundleURLMap["dev"]
		} else if strings.Contains(ic.SpireHost, "spire.stage.accuknox.com") {
			spireTrustBundleURL = spireTrustBundleURLMap["stage"]
		} else if strings.Contains(ic.SpireHost, "spire.demo.accuknox.com") {
			spireTrustBundleURL = spireTrustBundleURLMap["demo"]
		} else if strings.Contains(ic.SpireHost, "spire.prod.accuknox.com") {
			spireTrustBundleURL = spireTrustBundleURLMap["prod"]
		} else if strings.Contains(ic.SpireHost, "spire.xcitium.accuknox.com") {
			spireTrustBundleURL = spireTrustBundleURLMap["xcitium"]
		}
	}

	ic.TCArgs = TemplateConfigArgs{
		KubeArmorImage:            ic.KubeArmorImage,
		KubeArmorInitImage:        ic.KubeArmorInitImage,
		KubeArmorRelayServerImage: ic.KubeArmorRelayServerImage,
		KubeArmorVMAdapterImage:   ic.KubeArmorVMAdapterImage,
		SIAImage:                  ic.SIAImage,
		PEAImage:                  ic.PEAImage,
		FeederImage:               ic.FeederImage,

		Hostname: hostname,
		// TODO: make configurable
		KubeArmorURL:  "kubearmor:32767",
		KubeArmorPort: "32767",

		RelayServerURL:  "kubearmor-relay-server:32768",
		RelayServerAddr: "kubearmor-relay-server",
		RelayServerPort: "32768",

		WorkerNode: ic.WorkerNode,

		SIAAddr: "shared-informer-agent:32769",
		PEAAddr: "policy-enforcement-agent:32770",

		PPSHost: ic.PPSHost,

		JoinToken:     ic.JoinToken,
		SpireHostAddr: spireHost,
		SpireHostPort: spirePort,

		SpireTrustBundleURL: spireTrustBundleURL,
		ImagePullPolicy: string(ic.ImagePullPolicy),

		ConfigPath: configPath,
	}

	// initialize sprig for templating
	sprigFuncs := sprig.GenericFuncMap()

	// write compose file
	composeFilePath, err := copyOrGenerateFile(ic.UserConfigPath, configPath, "docker-compose.yaml", sprigFuncs, cpComposeFileTemplate, ic.TCArgs)
	if err != nil {
		return err
	}

	_, err = copyOrGenerateFile(ic.UserConfigPath, configPath, "pea/application.yaml", sprigFuncs, peaConfig, ic.TCArgs)
	if err != nil {
		return err
	}

	_, err = copyOrGenerateFile(ic.UserConfigPath, configPath, "sia/app.yaml", sprigFuncs, siaConfig, ic.TCArgs)
	if err != nil {
		return err
	}

	_, err = copyOrGenerateFile(ic.UserConfigPath, configPath, "spire/conf/agent.conf", sprigFuncs, spireAgentConfig, ic.TCArgs)
	if err != nil {
		return err
	}

	kmuxConfigArgs := KmuxConfigTemplateArgs{
		StreamName: "knox-gateway",
		ServerURL:  ic.KnoxGateway,
	}

	_, err = copyOrGenerateFile(ic.UserConfigPath, configPath, "sia/kmux-config.yaml", sprigFuncs, kmuxConfig, kmuxConfigArgs)
	if err != nil {
		return err
	}

	_, err = copyOrGenerateFile(ic.UserConfigPath, configPath, "feeder-service/kmux-config.yaml", sprigFuncs, kmuxConfig, kmuxConfigArgs)
	if err != nil {
		return err
	}

	// run compose command
	_, err = ExecComposeCommand(
		true, ic.DryRun,
		ic.composeCmd, "-f", composeFilePath,
		"--profile", "spire-agent", "--profile", "kubearmor",
		"--profile", "accuknox-agents", "up", "-d", "--wait", "--wait-timeout", "60")
	if err != nil {
		diagnosis, diagErr := diaganose(NodeType_ControlPlane)
		if diagErr != nil {
			diagnosis = diagErr.Error()
		}
		return fmt.Errorf("Error: %s.\n\nDIAGNOSIS:\n%s", err.Error(), diagnosis)
	}

	return nil
}
