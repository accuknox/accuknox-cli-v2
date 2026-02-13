package onboard

import (
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
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
	rmqAddr string, deploySumengine bool, registry, registryConfigPath string, insecureRegistryConnection, httpRegistryConnection, preserveUpstream bool, topicPrefix, connName, sumEngineCronTime string, tls TLS, enableHostPolicyDiscovery bool, splunk SplunkConfig, stateRefreshTime int, spireEnabled, spireCert bool, logRotate string, parallel int, hardeningService bool, releaseFile string, proxy Proxy, deployDiscover, skipDownload bool, fromSource string,
) (*ClusterConfig, error) {
	cc := new(ClusterConfig)

	if splunk.Enabled {
		if err := validateSplunkCredential(splunk); err != nil {
			return nil, err
		}
	}

	cc.DeployDiscover = deployDiscover

	cc.Parallel = parallel
	cc.SpireEnabled = spireEnabled
	cc.SpireCert = spireCert

	cc.EnableHardeningAgent = hardeningService

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

	var (
		err  error
		path = ""
	)

	if cc.UserConfigPath != "" {
		path = cc.UserConfigPath
	}

	if vmMode == VMMode_Systemd {
		path = "/opt"
	} else {
		path, err = cm.GetDefaultConfigPath()
		if err != nil {
			return nil, err
		}
	}

	if msg, err := cm.GetOrWriteReleaseInfo(releaseFile, path); err != nil {
		return nil, err
	} else {
		logger.Info1("%v", msg)
	}

	releaseInfo, ok := cm.ReleaseInfo[releaseVersion]
	if !ok && releaseVersion != "" {
		releaseVersion, releaseInfo = cm.GetReleaseFromBackup(path, releaseVersion)
		if releaseVersion != "" {
			logger.Warn("Release %v not found, using release %v from backup", releaseVersion, releaseVersion)
		}
	}
	if releaseVersion == "" {
		msg := "Release version not found"
		if !ok && releaseVersion != "" {
			msg = fmt.Sprintf("Release %v not found", releaseVersion)
		}
		releaseVersion, releaseInfo = cm.GetLatestReleaseInfoFromEmbedded()
		if releaseVersion == "" {
			return nil, fmt.Errorf("Release version not found")
		}
		logger.Warn("%v", fmt.Sprintf("%v, using latest release %v", msg, releaseVersion))
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
	images := map[string]string{
		"kubearmorImage":          kubearmorImage,
		"kubearmorVersion":        kubearmorVersion,
		"kubearmorInitImage":      kubearmorInitImage,
		"vmAdapterImage":          vmAdapterImage,
		"vmAdapterTag":            vmAdapterTag,
		"relayServerImage":        relayServerImage,
		"kubeArmorRelayServerTag": kubeArmorRelayServerTag,
		"siaImage":                siaImage,
		"siaVersionTag":           siaVersionTag,
		"peaImage":                peaImage,
		"peaVersionTag":           peaVersionTag,
		"feederImage":             feederImage,
		"feederVersionTag":        feederVersionTag,
		"waitForItImage":          waitForItImage,
		"discoverImage":           discoverImage,
		"discoverVersionTag":      discoverVersionTag,
		"sumEngineImage":          sumEngineImage,
		"sumEngineTag":            sumEngineTag,
		"hardeningAgentImage":     hardeningAgentImage,
		"hardeningAgentTag":       hardeningAgentVersionTag,
		"rmqImage":                rmqImage,
		"spireImage":              spireImage,
	}

	err = cc.PopulateImageDetails(releaseInfo,
		images,
		registry,
		registryConfigPath,
		"",
		preserveUpstream,
		insecureRegistryConnection,
		httpRegistryConnection)
	if err != nil {
		return nil, err
	}

	cc.Tls = tls
	cc.Splunk = splunk

	cc.NodeStateRefreshTime = stateRefreshTime

	cc.Proxy = proxy

	cc.SkipDownload = skipDownload

	fromPath, err := resolveSource(fromSource)
	if err != nil {
		return nil, err
	}
	cc.FromSource = fromPath

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
	spireTrustBundleURL := tbAddr
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

func (cc *ClusterConfig) PopulateImageDetails(releaseInfo cm.ReleaseMetadata, images map[string]string, registry, registryConfigPath, tagSuffix string, preserveUpstream, insecureRegistryConnection, httpRegistryConnection bool) error {

	if tagSuffix == "" {
		tagSuffix = cm.SystemdTagSuffix
	}

	var err error
	// mode specific config
	switch cc.Mode {
	case VMMode_Docker:
		cc.KubeArmorImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultKubeArmorRepo, images["kubearmorImage"], cm.DefaultKubeArmorImage,
			images["kubearmorVersion"], releaseInfo.KubeArmorTag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

		cc.KubeArmorInitImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultKubeArmorRepo, images["kubearmorInitImage"], cm.DefaultKubeArmorInitImage,
			images["kubearmorVersion"], releaseInfo.KubeArmorTag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

		cc.KubeArmorVMAdapterImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images["vmAdapterImage"], cm.DefaultVMAdapterImage,
			images["vmAdapterTag"], releaseInfo.KubeArmorVMAdapterTag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

		cc.KubeArmorRelayServerImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images["relayServerImage"], cm.DefaultRelayServerImage,
			images["kubeArmorRelayServerTag"], releaseInfo.KubeArmorRelayTag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

		cc.SIAImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images["siaImage"], releaseInfo.SIAImage,
			images["siaVersionTag"], releaseInfo.SIATag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

		cc.PEAImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images["peaImage"], releaseInfo.PEAImage,
			images["peaVersionTag"], releaseInfo.PEATag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

		cc.FeederImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images["feederImage"], releaseInfo.FeederServiceImage,
			images["feederVersionTag"], releaseInfo.FeederServiceTag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

		cc.SPIREAgentImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images["spireImage"], cm.DefaultSPIREAgentImage,
			"latest", releaseInfo.SPIREAgentImageTag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

		cc.WaitForItImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images["waitForItImage"], cm.DefaultWaitForItImage,
			"latest", "", "", "", preserveUpstream)
		if err != nil {
			return err
		}

		cc.DiscoverImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images["discoverImage"], releaseInfo.DiscoverImage,
			images["discoverVersionTag"], releaseInfo.DiscoverTag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

		cc.SumEngineImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images["sumEngineImage"], releaseInfo.SumEngineImage,
			images["sumEngineTag"], releaseInfo.SumEngineTag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

		cc.HardeningAgentImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images["hardeningAgentImage"], releaseInfo.HardeningAgentImage,
			images["hardeningAgentVersionTag"], releaseInfo.HardeningAgentTag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

		cc.RMQImage, err = getImage(registry, cm.DefaultDockerRegistry,
			"", images["rmqImage"], cm.DefaultRMQImage,
			"", cm.DefaultRMQImageTag, "", "", preserveUpstream)
		if err != nil {
			return err
		}
		cc.RRAImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images["rraImage"], releaseInfo.RraImage,
			images["rraVersionTag"], releaseInfo.RraTag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

	case VMMode_Systemd:
		kaVersion := images["kubearmorVersion"]
		if images["kubearmorVersion"] != "" && (images["kubearmorVersion"] == "stable" || images["kubearmorVersion"] == "latest") {
			fmt.Printf("%s tag not available for systemd package. Using values from release chart", images["kubearmorVersion"])
			kaVersion = ""
		}
		cc.KubeArmorImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultKubeArmorRepo, images["kubearmorImage"], cm.AgentRepos[cm.KubeArmor],
			kaVersion, releaseInfo.KubeArmorTag, "v", tagSuffix, preserveUpstream)
		if err != nil {
			return err
		}

		cc.KubeArmorVMAdapterImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images["vmAdapterImage"], cm.AgentRepos[cm.VMAdapter],
			images["vmAdapterTag"], releaseInfo.KubeArmorVMAdapterTag, "v", tagSuffix, preserveUpstream)
		if err != nil {
			return err
		}

		cc.KubeArmorRelayServerImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images["relayServerImage"], cm.AgentRepos[cm.RelayServer],
			images["kubeArmorRelayServerTag"], releaseInfo.KubeArmorRelayTag, "v", tagSuffix, preserveUpstream)
		if err != nil {
			return err
		}

		cc.SIAImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images["siaImage"], cm.AgentRepos[cm.SIAAgent],
			images["siaVersionTag"], releaseInfo.SIATag, "v", tagSuffix, preserveUpstream)
		if err != nil {
			return err
		}

		cc.PEAImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images["peaImage"], cm.AgentRepos[cm.PEAAgent],
			images["peaVersionTag"], releaseInfo.PEATag, "v", tagSuffix, preserveUpstream)
		if err != nil {
			return err
		}

		cc.FeederImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images["feederImage"], cm.AgentRepos[cm.FeederService],
			images["feederVersionTag"], releaseInfo.FeederServiceTag, "v", tagSuffix, preserveUpstream)
		if err != nil {
			return err
		}

		cc.SPIREAgentImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images["spireImage"], cm.AgentRepos[cm.SpireAgent],
			"", releaseInfo.SPIREAgentImageTag, "v", tagSuffix, preserveUpstream)
		if err != nil {
			return err
		}

		cc.SumEngineImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images["sumEngineImage"], cm.AgentRepos[cm.SummaryEngine],
			images["sumEngineTag"], releaseInfo.SumEngineTag, "v", tagSuffix, preserveUpstream)
		if err != nil {
			return err
		}

		cc.DiscoverImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images["discoverImage"], cm.AgentRepos[cm.DiscoverAgent],
			images["discoverVersionTag"], releaseInfo.DiscoverTag, "v", tagSuffix, preserveUpstream)
		if err != nil {
			return err
		}

		cc.HardeningAgentImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images["hardeningAgentImage"], cm.AgentRepos[cm.HardeningAgent],
			images["hardeningAgentVersionTag"], releaseInfo.HardeningAgentTag, "v", tagSuffix, preserveUpstream)
		if err != nil {
			return err
		}

		cc.RRAImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images["rraImage"], cm.AgentRepos[cm.RRA],
			images["rraVersionTag"], releaseInfo.RraTag, "v", cm.SystemdTagSuffix, preserveUpstream)
		if err != nil {
			return err
		}

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
			return err
		}
	}

	return nil
}

func resolveSource(source string) (string, error) {
	if isURL(source) {
		fmt.Println("Source detected as URL, downloading...")
		tempPath, err := downloadToTemp(source)
		return tempPath, err
	}

	if _, err := os.Stat(source); err != nil {
		return "", fmt.Errorf("file does not exist: %s", source)
	}

	return source, nil
}

func isURL(input string) bool {
	u, err := url.Parse(input)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func createRandomTempFile() (string, error) {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	name := fmt.Sprintf("download-%x", b)
	return filepath.Join(os.TempDir(), name), nil
}

func downloadToTemp(sourceURL string) (string, error) {
	// #nosec G107 -- false positive
	resp, err := http.Get(sourceURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch url: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %s", resp.Status)
	}

	tempFile, err := createRandomTempFile()
	if err != nil {
		return "", err
	}

	// #nosec G304 -- false positive
	out, err := os.Create(tempFile)
	if err != nil {
		return "", err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	return tempFile, nil
}
