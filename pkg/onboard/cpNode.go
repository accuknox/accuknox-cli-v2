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
		SumEngineImage:            ic.SumEngineImage,

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
	// Validate environment
	dockerStatus, err := ic.ValidateEnv()
	if err != nil {
		return err
	}
	fmt.Println(dockerStatus)

	configPath, err := createDefaultConfigPath()
	if err != nil {
		return err
	}

	// Set TCArgs with appropriate values
	ic.setTCArgs(configPath)

	// Initialize sprig functions for templating
	sprigFuncs := sprig.GenericFuncMap()

	// List of config files to be generated or copied
	fileTemplateMap := map[string]string{
		"docker-compose.yaml":   cpComposeFileTemplate,
		"pea/application.yaml":  peaConfig,
		"sia/app.yaml":          siaConfig,
		"sumengine/config.yaml": sumEngineConfig,
		"spire/conf/agent.conf": spireAgentConfig,
	}

	// Generate or copy files
	for filePath, templateString := range fileTemplateMap {
		if _, err := copyOrGenerateFile(ic.UserConfigPath, configPath, filePath, sprigFuncs, templateString, ic.TCArgs); err != nil {
			return err
		}
	}

	kmuxConfigArgs := KmuxConfigTemplateArgs{
		ReleaseVersion: ic.AgentsVersion,
		StreamName:     "knox-gateway",
		ServerURL:      ic.KnoxGateway,
	}

	// List of kmux config files to be generated or copied
	kmuxConfigFileTemplateMap := map[string]string{
		"sia/kmux-config.yaml":            kmuxConfig,
		"feeder-service/kmux-config.yaml": kmuxConfig,
		"pea/kmux-config.yaml":            kmuxConfig,
		"sumengine/kmux-config.yaml":      sumEnginekmuxConfig,
	}

	// Generate or copy kmux config files
	for filePath, templateString := range kmuxConfigFileTemplateMap {
		if _, err := copyOrGenerateFile(ic.UserConfigPath, configPath, filePath, sprigFuncs, templateString, kmuxConfigArgs); err != nil {
			return err
		}
	}

	// Diagnose if necessary and run compose command
	return ic.runComposeCommand(configPath)
}

// setTCArgs sets the necessary TCArgs values
func (ic *InitConfig) setTCArgs(configPath string) {
	ic.TCArgs = TemplateConfigArgs{
		KubeArmorImage:            ic.KubeArmorImage,
		KubeArmorInitImage:        ic.KubeArmorInitImage,
		KubeArmorRelayServerImage: ic.KubeArmorRelayServerImage,
		KubeArmorVMAdapterImage:   ic.KubeArmorVMAdapterImage,
		SIAImage:                  ic.SIAImage,
		PEAImage:                  ic.PEAImage,
		FeederImage:               ic.FeederImage,
		SumEngineImage:            ic.SumEngineImage,
		KubeArmorURL:              "kubearmor:32767",
		KubeArmorPort:             "32767",
		RelayServerURL:            "kubearmor-relay-server:32768",
		RelayServerAddr:           "kubearmor-relay-server",
		RelayServerPort:           "32768",
		WorkerNode:                ic.WorkerNode,
		SIAAddr:                   "shared-informer-agent:32769",
		PEAAddr:                   "policy-enforcement-agent:32770",
		ImagePullPolicy:           string(ic.ImagePullPolicy),
		ConfigPath:                configPath,
		KmuxConfigPathFS:          "/opt/feeder-service/kmux-config.yaml",
		KmuxConfigPathSIA:         "/opt/sia/kmux-config.yaml",
		KmuxConfigPathPEA:         "/opt/pea/kmux-config.yaml",
		KmuxConfigPathSumengine:   "/opt/sumengine/kmux-config.yaml",
	}
}

// runComposeCommand runs the Docker Compose command with the necessary arguments
func (ic *InitConfig) runComposeCommand(composeFilePath string) error {
	diagnosis := true
	args := []string{
		"-f", composeFilePath, "--profile", "spire-agent",
		"--profile", "kubearmor", "--profile", "accuknox-agents",
		"up", "-d",
	}

	if semver.Compare(ic.composeVersion, common.MinDockerComposeWithWaitSupported) >= 0 {
		args = append(args, "--wait", "--wait-timeout", "60")
	} else {
		diagnosis = false
	}

	// run compose command
	_, err := ExecComposeCommand(true, ic.DryRun, ic.composeCmd, args...)
	if err != nil {
		// cleanup volumes
		_, volDelErr := ExecDockerCommand(true, false, "docker", "volume", "rm", "spire-vol", "kubearmor-init-vol")
		if volDelErr != nil {
			fmt.Println("Error while removing volumes:", volDelErr.Error())
		}
		return ic.handleComposeError(err, diagnosis)
	}
	return nil
}

// handleComposeError handles errors from the Docker Compose command
func (ic *InitConfig) handleComposeError(err error, diagnosis bool) error {
	if diagnosis {
		diagnosisResult, diagErr := diaganose(NodeType_ControlPlane)
		if diagErr != nil {
			diagnosisResult = diagErr.Error()
		}
		return fmt.Errorf("Error: %s.\n\nDIAGNOSIS:\n%s", err.Error(), diagnosisResult)
	}
	return err
}
