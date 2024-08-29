package onboard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/sprig"
	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	"golang.org/x/mod/semver"
)

type agentConfigMeta struct {
	agentName                string
	configDir                string
	configFilePath           string
	configTemplateString     string
	kmuxConfigPath           string
	kmuxConfigTemplateString string
}

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
		WaitForItImage:            ic.WaitForItImage,
		SIAImage:                  ic.SIAImage,
		PEAImage:                  ic.PEAImage,
		FeederImage:               ic.FeederImage,
		RMQImage:                  ic.RMQImage,
		DiscoverImage:             ic.DiscoverImage,
		SumEngineImage:            ic.SumEngineImage,
		HardeningAgentImage:       ic.HardeningAgentImage,

		DeployRMQ: ic.DeployRMQ,

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
		HardenAddr: "hardening-agent:32771",

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

		KubeArmorAlertThrottling: ic.AlertThrottling,
		KubeArmorMaxAlertsPerSec: ic.MaxAlertsPerSec,
		KubeArmorThrottleSec:     ic.ThrottleSec,

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
	ic.TCArgs.DiscoverImage = ic.DiscoverImage
	ic.TCArgs.SumEngineImage = ic.SumEngineImage
	ic.TCArgs.HardeningAgentImage = ic.HardeningAgentImage

	ic.TCArgs.KubeArmorURL = "kubearmor:32767"
	ic.TCArgs.KubeArmorPort = "32767"

	ic.TCArgs.RelayServerURL = "kubearmor-relay-server:32768"
	ic.TCArgs.RelayServerAddr = "kubearmor-relay-server"
	ic.TCArgs.RelayServerPort = "32768"

	ic.TCArgs.WorkerNode = ic.WorkerNode

	ic.TCArgs.SIAAddr = "shared-informer-agent:32769"
	ic.TCArgs.PEAAddr = "policy-enforcement-agent:32770"
	ic.TCArgs.HardenAddr = "hardening-agent:32771"
	ic.TCArgs.ImagePullPolicy = string(ic.ImagePullPolicy)

	ic.TCArgs.ConfigPath = configPath

	ic.TCArgs.DiscoverRules = combineVisibilities(ic.Visibility, ic.HostVisibility)
	ic.TCArgs.ProcessOperation = isOperationDisabled(ic.Visibility, ic.HostVisibility, "process")
	ic.TCArgs.FileOperation = isOperationDisabled(ic.Visibility, ic.HostVisibility, "file")
	ic.TCArgs.NetworkOperation = isOperationDisabled(ic.Visibility, ic.HostVisibility, "network")

	kmuxConfigArgs := KmuxConfigTemplateArgs{
		ReleaseVersion: ic.AgentsVersion,
		StreamName:     "knox-gateway",
		ServerURL:      ic.KnoxGateway,
		RMQServer:      "rabbitmq:5672",
	}

	if ic.RMQServer != "" {
		ic.TCArgs.RMQAddr = ic.RMQServer
		kmuxConfigArgs.RMQServer = ic.RMQServer
	} else if ic.RMQServer == "" && !ic.DeployRMQ {
		return fmt.Errorf("RabbitMQ address must be specified if deployment is skipped")
	}

	// initialize sprig for templating
	sprigFuncs := sprig.GenericFuncMap()

	// write compose file
	composeFilePath, err := copyOrGenerateFile(ic.UserConfigPath, configPath, "docker-compose.yaml", sprigFuncs, cpComposeFileTemplate, ic.TCArgs)
	if err != nil {
		return err
	}

	// List of config files to be generated or copied
	// TODO: Refactor later
	agentMeta := getAgentConfigMeta()

	// Generate or copy config files
	for _, agentObj := range agentMeta {
		tcArgs := ic.TCArgs
		tcArgs.KmuxConfigPath = agentObj.kmuxConfigPath
		agentConfigPath := filepath.Join(configPath, agentObj.configDir)

		// generate config file if not empty
		if agentObj.configFilePath != "" {
			if _, err := copyOrGenerateFile(ic.UserConfigPath, agentConfigPath, agentObj.configFilePath, sprigFuncs, agentObj.configTemplateString, tcArgs); err != nil {
				return err
			}
		}

		// generate kmux config only if it exists for this agent
		if agentObj.kmuxConfigPath != "" {
			if _, err := copyOrGenerateFile(ic.UserConfigPath, agentConfigPath, common.KmuxConfigFileName, sprigFuncs, agentObj.kmuxConfigTemplateString, kmuxConfigArgs); err != nil {
				return err
			}
		}
	}

	// Diagnose if necessary and run compose command
	return ic.runComposeCommand(composeFilePath)
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
		diagnosisResult, diagErr := diagnose(NodeType_ControlPlane)
		if diagErr != nil {
			diagnosisResult = diagErr.Error()
		}
		return fmt.Errorf("Error: %s.\n\nDIAGNOSIS:\n%s", err.Error(), diagnosisResult)
	}
	return err
}

func combineVisibilities(visibility, hostVisibility string) string {
	visibilities := make(map[string]struct{})
	for _, vis := range strings.Split(visibility+","+hostVisibility, ",") {
		visibilities[vis] = struct{}{}
	}

	combined := make([]string, 0, len(visibilities))
	for vis := range visibilities {
		combined = append(combined, vis)
	}

	return strings.Join(combined, ",")
}

// isOperationDisabled returns true if the operation is not included in the combined visibility settings.
func isOperationDisabled(visibility, hostVisibility, operation string) bool {
	visibilities := make(map[string]struct{})
	for _, vis := range strings.Split(visibility+","+hostVisibility, ",") {
		visibilities[vis] = struct{}{}
	}
	_, exists := visibilities[operation]
	return !exists
}

func getAgentConfigMeta() []agentConfigMeta {
	agentMeta := []agentConfigMeta{
		{
			agentName:            "spire",
			configDir:            "spire",
			configFilePath:       "conf/agent.conf",
			configTemplateString: spireAgentConfig,
		},
		{
			agentName:                "sia",
			configDir:                "sia",
			configFilePath:           "app.yaml",
			configTemplateString:     siaConfig,
			kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "sia", common.KmuxConfigFileName),
			kmuxConfigTemplateString: kmuxConfig,
		},
		{
			agentName:                "pea",
			configDir:                "pea",
			configFilePath:           "application.yaml",
			configTemplateString:     peaConfig,
			kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "pea", common.KmuxConfigFileName),
			kmuxConfigTemplateString: kmuxConfig},
		{
			agentName:                "feeder-service",
			configDir:                "feeder-service",
			kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "feeder-service", common.KmuxConfigFileName),
			kmuxConfigTemplateString: kmuxConfig,
		},
		{
			agentName:                "sumengine",
			configDir:                "sumengine",
			configFilePath:           "config.yaml",
			configTemplateString:     sumEngineConfig,
			kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "sumengine", common.KmuxConfigFileName),
			kmuxConfigTemplateString: sumEngineKmuxConfig,
		},
		{
			agentName:                "discover",
			configDir:                "discover",
			configFilePath:           "config.yaml",
			configTemplateString:     discoverConfig,
			kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "discover", common.KmuxConfigFileName),
			kmuxConfigTemplateString: discoverKmuxConfig,
		},
		{
			agentName:                "hardening-agent",
			configDir:                "hardening-agent",
			configFilePath:           "config.yaml",
			configTemplateString:     hardeningAgentConfig,
			kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "hardening-agent", common.KmuxConfigFileName),
			kmuxConfigTemplateString: hardeningAgentKmuxConfig,
		},
	}

	return agentMeta
}
