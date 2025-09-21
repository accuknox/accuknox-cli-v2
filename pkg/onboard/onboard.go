package onboard

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
)

// TODO: THIS FUNCTION IS UNUSABLE!!!
func CreateClusterConfig(clusterType ClusterType, userConfigPath string, vmMode VMMode,
	vmAdapterTag, kubeArmorRelayServerTag, peaVersionTag, siaVersionTag, feederVersionTag, sumEngineTag, discoverVersionTag, hardeningAgentVersionTag string,
	kubearmorVersion, releaseVersion, kubearmorImage, kubearmorInitImage,
	vmAdapterImage, relayServerImage, siaImage, peaImage,
	feederImage, rmqImage, sumEngineImage, hardeningAgentImage, spireImage, waitForItImage, discoverImage, nodeAddress string, dryRun, workerNode, deployRMQ bool,
	imagePullPolicy, visibility, hostVisibility, sumengineViz, audit, block, hostAudit, hostBlock string,
	alertThrottling bool, maxAlertPerSec, throttleSec int,
	cidr string, secureContainers, skipBTF bool, systemMonitorPath string,
	rmqAddr string, deploySumengine bool, registry, registryConfigPath string, insecureRegistryConnection, httpRegistryConnection, preserveUpstream bool, topicPrefix, connName, sumEngineCronTime string, tls TLS, enableHostPolicyDiscovery bool, splunk SplunkConfig, stateRefreshTime int, spireEnabled, spireCert bool, logRotate string, parallel int) (*ClusterConfig, error) {

	cc := new(ClusterConfig)

	if splunk.Enabled {
		if err := validateSplunkCredential(splunk); err != nil {
			return nil, err
		}
	}

	cc.Parallel = parallel
	cc.SpireEnabled = spireEnabled
	cc.SpireCert = spireCert

	// check if a config path is given by user
	if userConfigPath != "" {
		cleanUserConfigPath := filepath.Clean(userConfigPath)
		_, err := os.Stat(userConfigPath)
		if err != nil {
			return nil, err
		}

		defaultConfigPath, err := cm.GetDefaultConfigPath()
		if err != nil {
			return nil, err
		}

		// userConfigPath can't be defaultConfigPath
		if strings.Compare(defaultConfigPath, cleanUserConfigPath) == 0 {
			return nil, fmt.Errorf("config path cannot be %s", defaultConfigPath)
		}

		cc.UserConfigPath = cleanUserConfigPath
	}

	cc.RMQTopicPrefix = topicPrefix

	cc.RMQConnectionName = connName

	cronTime, cronErr := time.ParseDuration(sumEngineCronTime)
	if cronErr != nil {
		cronTime = 15 * time.Minute
	}
	cc.SumEngineCronTime = cronTime

	// systemd or docker
	cc.Mode = vmMode

	cc.SecureContainers = secureContainers
	cc.SkipBTFCheck = skipBTF

	if systemMonitorPath != "" {
		_, err := os.Stat(systemMonitorPath)
		if err != nil {
			return nil, fmt.Errorf("failed to find system monitor at path %s", systemMonitorPath)
		}
	}
	cc.SystemMonitorPath = systemMonitorPath

	cc.ClusterType = clusterType

	cc.Visibility = visibility
	if cc.Visibility == "" {
		cc.Visibility = "process,network"
	}

	cc.HostVisibility = hostVisibility
	if cc.HostVisibility == "" {
		cc.HostVisibility = "process,network"
	}

	// if audit or no default posture specified
	cc.DefaultFilePosture = getDefaultPosture(audit, block, "file")
	cc.DefaultNetworkPosture = getDefaultPosture(audit, block, "network")
	cc.DefaultCapPosture = getDefaultPosture(audit, block, "capabilities")

	cc.DefaultHostFilePosture = getDefaultPosture(hostAudit, hostBlock, "file")
	cc.DefaultHostNetworkPosture = getDefaultPosture(hostAudit, hostBlock, "network")
	cc.DefaultHostCapPosture = getDefaultPosture(hostAudit, hostBlock, "capabilities")

	cc.AlertThrottling = alertThrottling
	cc.MaxAlertsPerSec = maxAlertPerSec
	cc.ThrottleSec = throttleSec

	var releaseInfo cm.ReleaseMetadata
	if releaseVersion == "" {
		_, releaseInfo = cm.GetLatestReleaseInfo()
	} else if releaseInfoTemp, ok := cm.ReleaseInfo[releaseVersion]; ok {
		releaseInfo = releaseInfoTemp
	} else {
		// TODO: publish release JSON as OCI artifact to remove dependency
		// on needing to build knoxctl again and again
		return nil, fmt.Errorf("Unknown image tag %s", releaseVersion)
	}
	cc.AgentsVersion = releaseVersion

	cc.ProcessOperation = isOperationDisabled(sumengineViz, cc.Visibility, cc.HostVisibility, "process")
	cc.FileOperation = isOperationDisabled(sumengineViz, cc.Visibility, cc.HostVisibility, "file")
	cc.NetworkOperation = isOperationDisabled(sumengineViz, cc.Visibility, cc.HostVisibility, "network")

	cc.DeployRMQ = deployRMQ
	if rmqAddr != "" {
		rmqHost, rmqPort, err := parseURL(rmqAddr)
		if err != nil {
			return nil, fmt.Errorf("parsing RMQ Address: %s", err.Error())
		}

		cc.RMQServer = rmqHost + ":" + rmqPort
	}

	cc.WorkerNode = workerNode
	cc.DeploySumengine = deploySumengine
	cc.DryRun = dryRun

	cc.CPNodeAddr = nodeAddress

	if cidr != "" {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, err
		}

		cc.CIDR = network.String()
	}

	// TODO: image pull policy in systemd mode
	switch strings.ToLower(imagePullPolicy) {
	case string(ImagePullPolicy_Always):
		cc.ImagePullPolicy = ImagePullPolicy_Always
	case string(ImagePullPolicy_IfNotPresent):
		cc.ImagePullPolicy = ImagePullPolicy_IfNotPresent
	case string(ImagePullPolicy_Never):
		cc.ImagePullPolicy = ImagePullPolicy_Never
	default:
		return nil, fmt.Errorf("Image pull policy %s unrecognized", imagePullPolicy)
	}

	cc.EnableHostPolicyDiscovery = enableHostPolicyDiscovery

	// mode specific config
	var err error
	switch cc.Mode {
	case VMMode_Docker:
		cc.KubeArmorImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultKubeArmorRepo, kubearmorImage, cm.DefaultKubeArmorImage,
			kubearmorVersion, releaseInfo.KubeArmorTag, "", "", preserveUpstream)
		if err != nil {
			return nil, err
		}

		cc.KubeArmorInitImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultKubeArmorRepo, kubearmorInitImage, cm.DefaultKubeArmorInitImage,
			kubearmorVersion, releaseInfo.KubeArmorTag, "", "", preserveUpstream)
		if err != nil {
			return nil, err
		}

		cc.KubeArmorVMAdapterImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, vmAdapterImage, cm.DefaultVMAdapterImage,
			vmAdapterTag, releaseInfo.KubeArmorVMAdapterTag, "", "", preserveUpstream)
		if err != nil {
			return nil, err
		}

		cc.KubeArmorRelayServerImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, relayServerImage, cm.DefaultRelayServerImage,
			kubeArmorRelayServerTag, releaseInfo.KubeArmorRelayTag, "", "", preserveUpstream)
		if err != nil {
			return nil, err
		}

		cc.SIAImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, siaImage, releaseInfo.SIAImage,
			siaVersionTag, releaseInfo.SIATag, "", "", preserveUpstream)
		if err != nil {
			return nil, err
		}

		cc.PEAImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, peaImage, releaseInfo.PEAImage,
			peaVersionTag, releaseInfo.PEATag, "", "", preserveUpstream)
		if err != nil {
			return nil, err
		}

		cc.FeederImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, feederImage, releaseInfo.FeederServiceImage,
			feederVersionTag, releaseInfo.FeederServiceTag, "", "", preserveUpstream)
		if err != nil {
			return nil, err
		}

		cc.SPIREAgentImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, spireImage, cm.DefaultSPIREAgentImage,
			"latest", releaseInfo.SPIREAgentImageTag, "", "", preserveUpstream)
		if err != nil {
			return nil, err
		}

		cc.WaitForItImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, waitForItImage, cm.DefaultWaitForItImage,
			"latest", "", "", "", preserveUpstream)
		if err != nil {
			return nil, err
		}

		cc.DiscoverImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, discoverImage, releaseInfo.DiscoverImage,
			discoverVersionTag, releaseInfo.DiscoverTag, "", "", preserveUpstream)
		if err != nil {
			return nil, err
		}

		cc.SumEngineImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, sumEngineImage, releaseInfo.SumEngineImage,
			sumEngineTag, releaseInfo.SumEngineTag, "", "", preserveUpstream)
		if err != nil {
			return nil, err
		}

		cc.HardeningAgentImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, hardeningAgentImage, releaseInfo.HardeningAgentImage,
			hardeningAgentVersionTag, releaseInfo.HardeningAgentTag, "", "", preserveUpstream)
		if err != nil {
			return nil, err
		}

		cc.RMQImage, err = getImage(registry, cm.DefaultDockerRegistry,
			"", rmqImage, cm.DefaultRMQImage,
			"", cm.DefaultRMQImageTag, "", "", preserveUpstream)
		if err != nil {
			return nil, err
		}

	case VMMode_Systemd:
		kaVersion := kubearmorVersion
		if kubearmorVersion != "" && (kubearmorVersion == "stable" || kubearmorVersion == "latest") {
			fmt.Printf("%s tag not available for systemd package. Using values from release chart", kubearmorVersion)
			kaVersion = ""
		}
		cc.KubeArmorImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultKubeArmorRepo, kubearmorImage, cm.AgentRepos[cm.KubeArmor],
			kaVersion, releaseInfo.KubeArmorTag, "v", cm.SystemdTagSuffix, preserveUpstream)
		if err != nil {
			return nil, err
		}

		cc.KubeArmorVMAdapterImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, vmAdapterImage, cm.AgentRepos[cm.VMAdapter],
			vmAdapterTag, releaseInfo.KubeArmorVMAdapterTag, "v", cm.SystemdTagSuffix, preserveUpstream)
		if err != nil {
			return nil, err
		}

		cc.KubeArmorRelayServerImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, relayServerImage, cm.AgentRepos[cm.RelayServer],
			kubeArmorRelayServerTag, releaseInfo.KubeArmorRelayTag, "v", cm.SystemdTagSuffix, preserveUpstream)
		if err != nil {
			return nil, err
		}

		cc.SIAImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, siaImage, cm.AgentRepos[cm.SIAAgent],
			siaVersionTag, releaseInfo.SIATag, "v", cm.SystemdTagSuffix, preserveUpstream)
		if err != nil {
			return nil, err
		}

		cc.PEAImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, peaImage, cm.AgentRepos[cm.PEAAgent],
			peaVersionTag, releaseInfo.PEATag, "v", cm.SystemdTagSuffix, preserveUpstream)
		if err != nil {
			return nil, err
		}

		cc.FeederImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, feederImage, cm.AgentRepos[cm.FeederService],
			feederVersionTag, releaseInfo.FeederServiceTag, "v", cm.SystemdTagSuffix, preserveUpstream)
		if err != nil {
			return nil, err
		}

		cc.SPIREAgentImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, spireImage, cm.AgentRepos[cm.SpireAgent],
			"", releaseInfo.SPIREAgentImageTag, "v", cm.SystemdTagSuffix, preserveUpstream)
		if err != nil {
			return nil, err
		}

		cc.SumEngineImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, sumEngineImage, cm.AgentRepos[cm.SummaryEngine],
			sumEngineTag, releaseInfo.SumEngineTag, "v", cm.SystemdTagSuffix, preserveUpstream)
		if err != nil {
			return nil, err
		}

		cc.DiscoverImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, discoverImage, cm.AgentRepos[cm.DiscoverAgent],
			discoverVersionTag, releaseInfo.DiscoverTag, "v", cm.SystemdTagSuffix, preserveUpstream)
		if err != nil {
			return nil, err
		}

		cc.HardeningAgentImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, hardeningAgentImage, cm.AgentRepos[cm.HardeningAgent],
			hardeningAgentVersionTag, releaseInfo.HardeningAgentTag, "v", cm.SystemdTagSuffix, preserveUpstream)
		if err != nil {
			return nil, err
		}
		// log file size
		cc.LogRotate = strings.ToUpper(logRotate)
		// create systemd service objects
		cc.CreateSystemdServiceObjects()

		// prepare OAuth credentials
		loginOptions := LoginOptions{
			Insecure:           insecureRegistryConnection,
			PlainHTTP:          httpRegistryConnection,
			Registry:           registry,
			RegistryConfigPath: registryConfigPath,
		}
		loginOptions.PlainHTTP = loginOptions.isPlainHttp(registry)
		cc.PlainHTTP = loginOptions.PlainHTTP

		cc.ORASClient, err = loginOptions.ORASGetAuthClient()
		if err != nil {
			return nil, err
		}
	}
	cc.Tls = tls
	cc.Splunk = splunk

	cc.NodeStateRefreshTime = stateRefreshTime

	return cc, nil
}

// prints join command - currently only with the default ports
// TODO: handle complex configuration
func (cc *ClusterConfig) PrintJoinCommand() {
	command := "knoxctl onboard vm node"

	cpNodeAddr := cc.CPNodeAddr
	if cc.CPNodeAddr == "" {
		cpNodeAddr = "<address-of-this-node>"
	}

	switch cc.Mode {
	case VMMode_Docker:
		command = fmt.Sprintf("%s --vm-mode=\"docker\"", command)

	case VMMode_Systemd:
		command = fmt.Sprintf("%s --vm-mode=\"systemd\" ", command)
	}

	if cc.CaCert != "" {
		command = fmt.Sprintf("%s --tls --ca-cert=\"%s\" --auth=\"%s\"", command, cc.CaCert, cc.RMQCredentials)
	}
	if cc.Tls.Enabled {
		command = fmt.Sprintf("%s --deploy-summary-engine", command)
	}

	command = fmt.Sprintf("%s --version=%s --cp-node-addr=%s", command, cc.AgentsVersion, cpNodeAddr)

	logger.Print("%s", command)
}

func getDefaultPosture(auditPostureVal, blockPostureVal, ruleType string) string {
	if auditPostureVal == "all" || (auditPostureVal == "" && blockPostureVal == "") {
		return "audit"
	} else if blockPostureVal == "all" {
		return "block"
	}

	if strings.Contains(auditPostureVal, ruleType) {
		return "audit"
	} else if strings.Contains(blockPostureVal, ruleType) {
		return "block"
	}

	// unrecognized or default
	return "audit"
}

// isOperationDisabled returns true if the operation is not included in the combined visibility settings.
func isOperationDisabled(sumengineViz, visibility, hostVisibility, operation string) bool {
	visibilities := make(map[string]struct{})
	for _, vis := range strings.Split(visibility+","+hostVisibility, ",") {
		visibilities[vis] = struct{}{}
	}
	_, exists := visibilities[operation]

	// if sumengine visibility is disabled for this operation, then return not exists
	if exists && !strings.Contains(sumengineViz, operation) {
		return true
	}

	return !exists
}

func getSpireDetails(addrs, tbAddr string) (string, string, string, error) {
	spireHost, spirePort, err := parseURL(addrs)
	if err != nil {
		return "", "", "", err
	}
	if spirePort == "80" {
		// default spire port
		spirePort = "8081"
	}

	// currently unused as we use insecure bootstrap
	var spireTrustBundleURL = tbAddr
	if spireTrustBundleURL == "" {
		switch {
		case strings.Contains(addrs, SpireDev):
			spireTrustBundleURL = spireTrustBundleURLMap["dev"]
		case strings.Contains(addrs, SpireStage):
			spireTrustBundleURL = spireTrustBundleURLMap["stage"]
		case strings.Contains(addrs, SpireDemo):
			spireTrustBundleURL = spireTrustBundleURLMap["demo"]
		case strings.Contains(addrs, SpireProd):
			spireTrustBundleURL = spireTrustBundleURLMap["prod"]
		case strings.Contains(addrs, SpireXcitium):
			spireTrustBundleURL = spireTrustBundleURLMap["xcitium"]
		}
	}
	return spireHost, spirePort, spireTrustBundleURL, nil
}

func (cc *ClusterConfig) PopulateAccessKeyConfig(url, key, clusterName, vmName, endpoint, mode string, insecure bool) (string, error) {

	cc.AccessKey = AccessKey{
		Key:         key,
		Url:         url,
		Insecure:    insecure,
		Mode:        mode,
		ClusterName: vmName,
		Endpoint:    endpoint,
	}

	var (
		joinToken string
		err       error
	)

	if strings.Contains(cc.SPIREAgentImage, "v1.9.4") {
		joinToken, err = GetJoinTokenFromAccessKey(key, clusterName, vmName, mode, url, insecure)
		if err != nil {
			return "", err
		}
	}
	if cc.Mode == VMMode_Docker {
		if hostname, err := os.Hostname(); err == nil {
			cc.AccessKey.NodeName = fmt.Sprintf("%v-%v", hostname, time.Now().Unix())
		}
	}

	return joinToken, nil

}
