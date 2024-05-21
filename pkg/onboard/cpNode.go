package onboard

import (
	"fmt"
	"os"
	"strings"

	"github.com/Masterminds/sprig"
	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	"golang.org/x/mod/semver"
)

func InitCPNodeConfig(cc ClusterConfig, joinToken, spireHost, ppsHost, knoxGateway, spireTrustBundle string, enableLogs bool) *InitConfig {
	return &InitConfig{
		ClusterConfig: cc,
		JoinToken:     joinToken,
		SpireHost:     spireHost,
		PPSHost:       ppsHost,
		KnoxGateway:   knoxGateway,

		SpireTrustBundleURL: spireTrustBundle,
		EnableLogs:          enableLogs,
	}
}
func (ic *InitConfig) CreateBaseTemplateConfig() error {
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	spireHost, spirePort, err := parseURL(ic.SpireHost)
	if err != nil {
		return err
	}
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
		ReleaseVersion: ic.AgentsVersion,

		KubeArmorImage:            ic.KubeArmorImage,
		KubeArmorInitImage:        ic.KubeArmorInitImage,
		KubeArmorRelayServerImage: ic.KubeArmorRelayServerImage,
		KubeArmorVMAdapterImage:   ic.KubeArmorVMAdapterImage,
		SPIREAgentImage:           ic.SPIREAgentImage,
		SIAImage:                  ic.SIAImage,
		PEAImage:                  ic.PEAImage,
		FeederImage:               ic.FeederImage,
		SumengineImage:            ic.SumengineImage,

		Hostname: hostname,
		// TODO: make configurable
		KubeArmorURL:  "kubearmor:32767",
		KubeArmorPort: "32767",

		RelayServerURL:  "kubearmor-relay-server:32768",
		RelayServerAddr: "kubearmor-relay-server",
		RelayServerPort: "32768",

		WorkerNode: ic.WorkerNode,

		SIAAddr:    "shared-informer-agent:32769",
		PEAAddr:    "policy-enforcement-agent:32770",
		EnableLogs: ic.EnableLogs,

		PPSHost: ic.PPSHost,

		JoinToken:     ic.JoinToken,
		SpireHostAddr: spireHost,
		SpireHostPort: spirePort,

		SpireTrustBundleURL: spireTrustBundleURL,

		// kubearmor config
		KubeArmorVisibility:     ic.Visibility,
		KubeArmorHostVisibility: ic.HostVisibility,

		KubeArmorFilePosture:    ic.DefaultFilePosture,
		KubeArmorNetworkPosture: ic.DefaultNetworkPosture,
		KubeArmorCapPosture:     ic.DefaultCapPosture,

		KubeArmorHostFilePosture:    ic.DefaultHostFilePosture,
		KubeArmorHostNetworkPosture: ic.DefaultHostNetworkPosture,
		KubeArmorHostCapPosture:     ic.DefaultHostCapPosture,

		NetworkCIDR: ic.CIDR,

		SecureContainers: ic.SecureContainers,

		VmMode: ic.Mode,
	}
	return nil
}

func (ic *InitConfig) InitializeControlPlane() error {
	// validate this environment
	dockerStatus, err := ic.ValidateEnv()
	if err != nil {
		return err
	}
	fmt.Println(dockerStatus)

	configPath, err := createDefaultConfigPath()
	if err != nil {
		return err
	}
	ic.TCArgs.KubeArmorImage = ic.KubeArmorImage
	ic.TCArgs.KubeArmorInitImage = ic.KubeArmorInitImage
	ic.TCArgs.KubeArmorRelayServerImage = ic.KubeArmorRelayServerImage
	ic.TCArgs.KubeArmorVMAdapterImage = ic.KubeArmorVMAdapterImage

	// agents
	ic.TCArgs.SIAImage = ic.SIAImage
	ic.TCArgs.PEAImage = ic.PEAImage
	ic.TCArgs.FeederImage = ic.FeederImage
	ic.TCArgs.SumengineImage = ic.SumengineImage

	ic.TCArgs.KubeArmorURL = "kubearmor:32767"
	ic.TCArgs.KubeArmorPort = "32767"

	ic.TCArgs.RelayServerURL = "kubearmor-relay-server:32768"
	ic.TCArgs.RelayServerAddr = "kubearmor-relay-server"
	ic.TCArgs.RelayServerPort = "32768"

	ic.TCArgs.WorkerNode = ic.WorkerNode

	ic.TCArgs.SIAAddr = "shared-informer-agent:32769"
	ic.TCArgs.PEAAddr = "policy-enforcement-agent:32770"
	ic.TCArgs.ImagePullPolicy = string(ic.ImagePullPolicy)

	ic.TCArgs.ConfigPath = configPath

	// kmux config file paths
	ic.TCArgs.KmuxConfigPathFS = "/opt/feeder-service/kmux-config.yaml"
	ic.TCArgs.KmuxConfigPathSIA = "/opt/sia/kmux-config.yaml"
	ic.TCArgs.KmuxConfigPathPEA = "/opt/pea/kmux-config.yaml"
	ic.TCArgs.KmuxConfigPathSumengine = "/opt/sumengine/kmux-config.yaml"

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

	_, err = copyOrGenerateFile(ic.UserConfigPath, configPath, "conf/config.yaml", sprigFuncs, sumengineConfig, ic.TCArgs)
	if err != nil {
		return err
	}

	_, err = copyOrGenerateFile(ic.UserConfigPath, configPath, "spire/conf/agent.conf", sprigFuncs, spireAgentConfig, ic.TCArgs)
	if err != nil {
		return err
	}

	kmuxConfigArgs := KmuxConfigTemplateArgs{
		ReleaseVersion: ic.AgentsVersion,
		StreamName:     "knox-gateway",
		ServerURL:      ic.KnoxGateway,
	}

	_, err = copyOrGenerateFile(ic.UserConfigPath, configPath, "sia/kmux-config.yaml", sprigFuncs, kmuxConfig, kmuxConfigArgs)
	if err != nil {
		return err
	}

	_, err = copyOrGenerateFile(ic.UserConfigPath, configPath, "feeder-service/kmux-config.yaml", sprigFuncs, kmuxConfig, kmuxConfigArgs)
	if err != nil {
		return err
	}

	_, err = copyOrGenerateFile(ic.UserConfigPath, configPath, "pea/kmux-config.yaml", sprigFuncs, kmuxConfig, kmuxConfigArgs)
	if err != nil {
		return err
	}

	_, err = copyOrGenerateFile(ic.UserConfigPath, configPath, "conf/kmux-config.yaml", sprigFuncs, sumenginekmuxConfig, kmuxConfigArgs)
	if err != nil {
		return err
	}

	diagnosis := true
	args := []string{"-f", composeFilePath, "--profile",
		"spire-agent", "--profile", "kubearmor", "--profile", "accuknox-agents",
		"up", "-d"}

	if semver.Compare(ic.composeVersion, common.MinDockerComposeWithWaitSupported) >= 0 {
		args = append(args, "--wait", "--wait-timeout", "60")
	} else {
		diagnosis = false
	}

	// run compose command
	_, err = ExecComposeCommand(true, ic.DryRun, ic.composeCmd, args...)
	if err != nil && diagnosis {
		diagnosis, diagErr := diaganose(NodeType_ControlPlane)
		if diagErr != nil {
			diagnosis = diagErr.Error()
		}
		return fmt.Errorf("error: %s.\n\nDIAGNOSIS:\n%s", err.Error(), diagnosis)
	} else if err != nil {
		return err
	}

	return nil
}
