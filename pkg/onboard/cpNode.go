package onboard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/sprig"
	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"golang.org/x/mod/semver"
)

type agentConfigMeta struct {
	agentName                string
	configDir                string
	configFilePath           string
	configTemplateString     string
	kmuxConfigPath           string
	kmuxConfigTemplateString string
	kmuxConfigFileName       string
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

		VmMode:         ic.Mode,
		RMQServer:      ic.RMQServer,
		RMQTopicPrefix: ic.RMQTopicPrefix,

		EnableHostPolicyDiscovery: ic.EnableHostPolicyDiscovery,

		ProcessOperation:     ic.ProcessOperation,
		FileOperation:        ic.FileOperation,
		NetworkOperation:     ic.NetworkOperation,
		RATConfigObject:      ic.RATConfigObject,
		SumEngineCronTime:    ic.SumEngineCronTime,
		NodeStateRefreshTime: ic.NodeStateRefreshTime,
	}
	return nil
}

func (ic *InitConfig) InitializeControlPlane() error {
	// validate this environment
	dockerStatus, err := ic.ValidateEnv()
	if err != nil {
		return err
	}
	logger.Info1(dockerStatus)

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
	if ic.Tls.Enabled {
		ic.TCArgs.TlsEnabled = ic.Tls.Enabled
		ic.TCArgs.TlsCertFile = fmt.Sprintf("%s%s%s/%s", ic.UserConfigPath, configPath, common.DefaultCACertDir, common.DefaultEncodedFileName)
		if err := ic.handleTLS(); err != nil {
			return err
		}
	}
	kmuxConfigArgs.RMQUsername = ic.TCArgs.RMQUsername
	kmuxConfigArgs.RMQPassword = ic.TCArgs.RMQPassword
	kmuxConfigArgs.TlsEnabled = ic.TCArgs.TlsEnabled

	ic.TCArgs.NodeStateRefreshTime = ic.NodeStateRefreshTime

	ic.populateCommonArgs()

	if ic.TCArgs.SplunkConfigObject.Enabled {
		if err := validateSplunkCredential(ic.TCArgs.SplunkConfigObject); err != nil {
			return err
		}
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
	agentMeta := getAgentConfigMeta(ic.Tls.Enabled)

	// Generate or copy config files
	for _, agentObj := range agentMeta {
		tcArgs := ic.TCArgs
		tcArgs.KmuxConfigPath = agentObj.kmuxConfigPath
		agentConfigPath := filepath.Join(configPath, agentObj.configDir)

		// generate config file if not empty
		if agentObj.configFilePath != "" {
			populateAgentArgs(&tcArgs, agentObj.configDir)
			if _, err := copyOrGenerateFile(ic.UserConfigPath, agentConfigPath, agentObj.configFilePath, sprigFuncs, agentObj.configTemplateString, tcArgs); err != nil {
				return err
			}
		}
		// generate kmux config only if it exists for this agent
		if agentObj.kmuxConfigPath != "" {
			populateKmuxArgs(&kmuxConfigArgs, agentObj.agentName, agentObj.kmuxConfigFileName, ic.TCArgs.RMQTopicPrefix, tcArgs.Hostname, ic.RMQConnectionName)
			if _, err := copyOrGenerateFile(ic.UserConfigPath, agentConfigPath, agentObj.kmuxConfigFileName, sprigFuncs, agentObj.kmuxConfigTemplateString, kmuxConfigArgs); err != nil {
				return err
			}
		}
	}

	// Diagnose if necessary and run compose command
	return ic.runComposeCommand(composeFilePath)
}

func (ic *InitConfig) populateCommonArgs() {

	ic.TCArgs.PoliciesKmuxConfig = common.KmuxPoliciesFileName
	ic.TCArgs.StateKmuxConfig = common.KmuxStateEventFileName
	ic.TCArgs.AlertsKmuxConfig = common.KmuxAlertsFileName
	ic.TCArgs.LogsKmuxConfig = common.KmuxLogsFileName
	ic.TCArgs.SummaryKmuxConfig = common.KmuxSummaryFileName
	ic.TCArgs.PolicyKmuxConfig = common.KmuxPolicyFileName

	ic.TCArgs.DiscoverRules = combineVisibilities(ic.Visibility, ic.HostVisibility)

	// To get routing key name with cluster-name as prefix
	ic.TCArgs.PoliciesTopic = getTopicName(ic.RMQTopicPrefix, "policies")
	ic.TCArgs.LogsTopic = getTopicName(ic.RMQTopicPrefix, "logs")
	ic.TCArgs.AlertsTopic = getTopicName(ic.RMQTopicPrefix, "alerts")
	ic.TCArgs.StateEventTopic = getTopicName(ic.RMQTopicPrefix, "state-event")
	ic.TCArgs.PolicyV1Topic = getTopicName(ic.RMQTopicPrefix, "policy-v1")
	ic.TCArgs.SummaryV2Topic = getTopicName(ic.RMQTopicPrefix, "summary-v2")

	ic.TCArgs.SplunkConfigObject = ic.Splunk

}

func populateAgentArgs(tcArgs *TemplateConfigArgs, configDir string) {
	tcArgs.PoliciesKmuxConfig = fmt.Sprintf("%s/%s/%s", common.InContainerConfigDir, configDir, common.KmuxPoliciesFileName)
	tcArgs.StateKmuxConfig = fmt.Sprintf("%s/%s/%s", common.InContainerConfigDir, configDir, common.KmuxStateEventFileName)
	tcArgs.AlertsKmuxConfig = fmt.Sprintf("%s/%s/%s", common.InContainerConfigDir, configDir, common.KmuxAlertsFileName)
	tcArgs.LogsKmuxConfig = fmt.Sprintf("%s/%s/%s", common.InContainerConfigDir, configDir, common.KmuxLogsFileName)
	tcArgs.SummaryKmuxConfig = fmt.Sprintf("%s/%s/%s", common.InContainerConfigDir, configDir, common.KmuxSummaryFileName)
	tcArgs.PolicyKmuxConfig = fmt.Sprintf("%s/%s/%s", common.InContainerConfigDir, configDir, common.KmuxPolicyFileName)
}

func populateKmuxArgs(kmuxConfigArgs *KmuxConfigTemplateArgs, agentName, kmuxFile, prefix, hostname, connName string) {

	if prefix == "" {
		prefix = "agents"
	}

	kmuxConfigArgs.ConsumerTag = agentName
	kmuxConfigArgs.QueueDurability = getQueueDurability(kmuxFile)
	kmuxConfigArgs.TlsCertFile = fmt.Sprintf("/opt%s/%s", common.DefaultCACertDir, common.DefaultEncodedFileName)
	if kmuxFile == common.KmuxPoliciesFileName {
		kmuxConfigArgs.ExchangeType = "fanout"
		kmuxConfigArgs.ExchangeName = fmt.Sprintf("%s-fanout", prefix)
	} else {
		kmuxConfigArgs.ExchangeType = "direct"
		kmuxConfigArgs.ExchangeName = prefix
	}

	if qn, ok := common.QueueName[kmuxFile]; ok {
		kmuxConfigArgs.QueueName = fmt.Sprintf("%s-%s", prefix, qn)
	}

	if (kmuxFile == common.KmuxStateEventFileName || kmuxFile == common.KmuxSummaryFileName) && agentName != common.VMAdapter {
		kmuxConfigArgs.QueueName = fmt.Sprintf("%s-%s", kmuxConfigArgs.QueueName, agentName)
	}

	if agentName == common.VMAdapter && kmuxFile == common.KmuxPoliciesFileName {
		if hostname == "" {
			var err error
			hostname, err = os.Hostname()
			if err != nil {
				hostname = common.VMAdapter
			}
		}
		kmuxConfigArgs.QueueName = fmt.Sprintf("%s-%s-%v", kmuxConfigArgs.QueueName, hostname, time.Now().Unix())

		if len(kmuxConfigArgs.QueueName) > common.MaxQueueLength {
			kmuxConfigArgs.QueueName = kmuxConfigArgs.QueueName[:common.MaxQueueLength]
		}
	}

	if connName == "" {
		if hostname == "" {
			hostname, _ = os.Hostname()
		}
		connName = fmt.Sprintf("%s-%s-%s-%v", prefix, hostname, agentName, time.Now().Unix())
		connName = strings.TrimPrefix(connName, "-")
	}
	kmuxConfigArgs.ConnectionName = connName
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

func (ic *InitConfig) handleTLS() error {

	paths := oldCertPaths(ic.TCArgs.ConfigPath)

	if ic.Tls.Enabled && ic.Tls.CaPath == "" && len(paths) == 0 {
		ic.Tls.Generate = true
	}
	if ic.RMQCredentials == "" {
		ic.TCArgs.RMQUsername, ic.TCArgs.RMQPassword = GenerateUserAndPassword()
		ic.TCArgs.RMQPasswordHash = GetHash(ic.TCArgs.RMQPassword)
		ic.RMQCredentials = Encode([]byte(ic.TCArgs.RMQUsername + ":" + ic.TCArgs.RMQPassword))
	}
	ic.TCArgs.RMQTlsPort = "5672"

	storeData, caBytes, err := ic.GenerateOrUpdateCert(paths)
	if err != nil {
		return err
	}

	ic.CaCert = Encode(caBytes)

	storeData[ic.TCArgs.TlsCertFile] = ic.CaCert

	if err := StoreCert(storeData); err != nil {
		return err
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

func getAgentConfigMeta(tlsEnabled bool) []agentConfigMeta {
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
			kmuxConfigFileName:       common.KmuxConfigFileName,
		},
		{
			agentName:                "pea",
			configDir:                "pea",
			configFilePath:           "application.yaml",
			configTemplateString:     peaConfig,
			kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "pea", common.KmuxConfigFileName),
			kmuxConfigTemplateString: kmuxConfig,
			kmuxConfigFileName:       common.KmuxConfigFileName,
		},

		{
			agentName:                "feeder-service",
			configDir:                "feeder-service",
			kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "feeder-service", common.KmuxConfigFileName),
			kmuxConfigTemplateString: kmuxConfig,
			kmuxConfigFileName:       common.KmuxConfigFileName,
		},
		{
			agentName:                "sumengine",
			configDir:                "sumengine",
			configFilePath:           "config.yaml",
			configTemplateString:     sumEngineConfig,
			kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "sumengine", common.KmuxConfigFileName),
			kmuxConfigTemplateString: kmuxConfig,
			kmuxConfigFileName:       common.KmuxConfigFileName,
		},
		{
			agentName:                "discover",
			configDir:                "discover",
			configFilePath:           "config.yaml",
			configTemplateString:     discoverConfig,
			kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "discover", common.KmuxConfigFileName),
			kmuxConfigTemplateString: kmuxConfig,
			kmuxConfigFileName:       common.KmuxConfigFileName,
		},
		{
			agentName:                "hardening-agent",
			configDir:                "hardening-agent",
			configFilePath:           "config.yaml",
			configTemplateString:     hardeningAgentConfig,
			kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "hardening-agent", common.KmuxConfigFileName),
			kmuxConfigTemplateString: kmuxConfig,
			kmuxConfigFileName:       common.KmuxConfigFileName,
		},
	}
	if tlsEnabled {
		agentMeta = append(agentMeta, []agentConfigMeta{
			{
				agentName:            "rabbitmq",
				configDir:            "rabbitmq",
				configFilePath:       "rabbitmq.conf",
				configTemplateString: rabbitmqConfig,
			},
			{
				agentName:            "rabbitmq",
				configDir:            "rabbitmq",
				configFilePath:       "definitions.json",
				configTemplateString: rabbitmqDefinitions,
			},
			{
				agentName:                "vm-adapter",
				configDir:                "vm-adapter",
				kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "vm-adapter", common.KmuxAlertsFileName),
				kmuxConfigTemplateString: kmuxPublisherConfig,
				kmuxConfigFileName:       common.KmuxAlertsFileName,
			},
			{
				agentName:                "vm-adapter",
				configDir:                "vm-adapter",
				kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "vm-adapter", common.KmuxLogsFileName),
				kmuxConfigTemplateString: kmuxPublisherConfig,
				kmuxConfigFileName:       common.KmuxLogsFileName,
			},
			{
				agentName:                "vm-adapter",
				configDir:                "vm-adapter",
				kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "vm-adapter", common.KmuxStateEventFileName),
				kmuxConfigTemplateString: kmuxPublisherConfig,
				kmuxConfigFileName:       common.KmuxStateEventFileName,
			},
			{
				agentName:                "vm-adapter",
				configDir:                "vm-adapter",
				kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "vm-adapter", common.KmuxPoliciesFileName),
				kmuxConfigTemplateString: kmuxConsumerConfig,
				kmuxConfigFileName:       common.KmuxPoliciesFileName,
			},
		}...)
	}

	agentMeta = append(agentMeta, getAgentsKmuxConfigs()...)

	return agentMeta
}

func getAgentsKmuxConfigs() []agentConfigMeta {
	return []agentConfigMeta{
		{
			agentName:                "feeder-service",
			configDir:                "feeder-service",
			kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "feeder-service", common.KmuxAlertsFileName),
			kmuxConfigTemplateString: kmuxConsumerConfig,
			kmuxConfigFileName:       common.KmuxAlertsFileName,
		},
		{
			agentName:                "feeder-service",
			configDir:                "feeder-service",
			kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "feeder-service", common.KmuxLogsFileName),
			kmuxConfigTemplateString: kmuxConsumerConfig,
			kmuxConfigFileName:       common.KmuxLogsFileName,
		},
		{
			agentName:                "feeder-service",
			configDir:                "feeder-service",
			kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "feeder-service", common.KmuxPolicyFileName),
			kmuxConfigTemplateString: kmuxConsumerConfig,
			kmuxConfigFileName:       common.KmuxPolicyFileName,
		},
		{
			agentName:                "feeder-service",
			configDir:                "feeder-service",
			kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "feeder-service", common.KmuxSummaryFileName),
			kmuxConfigTemplateString: kmuxConsumerConfig,
			kmuxConfigFileName:       common.KmuxSummaryFileName,
		},
		{
			agentName:                "pea",
			configDir:                "pea",
			kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "pea", common.KmuxPoliciesFileName),
			kmuxConfigTemplateString: kmuxPublisherConfig,
			kmuxConfigFileName:       common.KmuxPoliciesFileName,
		},
		{
			agentName:                "pea",
			configDir:                "pea",
			kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "pea", common.KmuxStateEventFileName),
			kmuxConfigTemplateString: kmuxConsumerConfig,
			kmuxConfigFileName:       common.KmuxStateEventFileName,
		},
		{
			agentName:                "sumengine",
			configDir:                "sumengine",
			kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "sumengine", common.KmuxSummaryFileName),
			kmuxConfigTemplateString: kmuxPublisherConfig,
			kmuxConfigFileName:       common.KmuxSummaryFileName,
		},
		{
			agentName:                "sumengine",
			configDir:                "sumengine",
			kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "sumengine", common.KmuxAlertsFileName),
			kmuxConfigTemplateString: kmuxConsumerConfig,
			kmuxConfigFileName:       common.KmuxAlertsFileName,
		},
		{
			agentName:                "sumengine",
			configDir:                "sumengine",
			kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "sumengine", common.KmuxLogsFileName),
			kmuxConfigTemplateString: kmuxConsumerConfig,
			kmuxConfigFileName:       common.KmuxAlertsFileName,
		},
		{
			agentName:                "discover",
			configDir:                "discover",
			kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "discover", common.KmuxSummaryFileName),
			kmuxConfigTemplateString: kmuxConsumerConfig,
			kmuxConfigFileName:       common.KmuxSummaryFileName,
		},
		{
			agentName:                "discover",
			configDir:                "discover",
			kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "discover", common.KmuxPolicyFileName),
			kmuxConfigTemplateString: kmuxPublisherConfig,
			kmuxConfigFileName:       common.KmuxPolicyFileName,
		},
		{
			agentName:                "hardening-agent",
			configDir:                "hardening-agent",
			kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "hardening-agent", common.KmuxStateEventFileName),
			kmuxConfigTemplateString: kmuxConsumerConfig,
			kmuxConfigFileName:       common.KmuxStateEventFileName,
		},
		{
			agentName:                "hardening-agent",
			configDir:                "hardening-agent",
			kmuxConfigPath:           filepath.Join(common.InContainerConfigDir, "hardening-agent", common.KmuxPolicyFileName),
			kmuxConfigTemplateString: kmuxPublisherConfig,
			kmuxConfigFileName:       common.KmuxPolicyFileName,
		},
	}
}

// getTopicName Returns:
// - The topic name with the prefix, if the prefix is not empty.
// - The topic name itself, if the prefix is empty.
func getTopicName(prefix, topic string) string {
	if prefix == "" {
		return topic
	}
	return prefix + "-" + topic
}

func oldCertPaths(root string) []string {

	paths := []string{}

	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(d.Name()) == ".pem" {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		return paths
	}

	return paths
}

func getQueueDurability(kmuxFile string) bool {
	durable, ok := common.QueueDurability[kmuxFile]
	if !ok {
		return false
	}
	return durable
}
