package onboard

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/mholt/archives"
)

const AgentsVersionFile = "agents-version"

// TODO: THIS FUNCTION IS UNUSABLE!!!
func CreateClusterConfig(clusterType ClusterType, userConfigPath string, vmMode VMMode,
	imageVersions *ImageVersions, releaseVersion,
	nodeAddress string, dryRun, workerNode, deployRMQ bool,
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
	sourceVersion := ""
	if fromSource != "" {
		path, version, err := resolveSource(fromSource, releaseVersion)
		if err != nil {
			return nil, err
		}
		cc.FromSource = path

		sourceVersion = version
	}

	if releaseVersion != "" && sourceVersion != "" && releaseVersion != sourceVersion {
		return nil, fmt.Errorf("release version %s and source version %s do not match", releaseVersion, sourceVersion)
	}

	if releaseVersion == "" && sourceVersion != "" {
		releaseVersion = sourceVersion
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

	err = cc.PopulateImageDetails(releaseInfo,
		imageVersions,
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

	cc.AgentsVersionFile = filepath.Join(path, AgentsVersionFile)

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
		ClusterName: clusterName,
		Endpoint:    endpoint,
		NodeName:    vmName,
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
	if vmName == "" {
		if hostname, err := os.Hostname(); err == nil {
			cc.AccessKey.NodeName = fmt.Sprintf("%v-%v", hostname, time.Now().Unix())
		}
	}

	return joinToken, nil
}

func (cc *ClusterConfig) PopulateImageDetails(releaseInfo cm.ReleaseMetadata, images *ImageVersions, registry, registryConfigPath, tagSuffix string, preserveUpstream, insecureRegistryConnection, httpRegistryConnection bool) error {

	if tagSuffix == "" {
		tagSuffix = cm.SystemdTagSuffix
	}

	var err error
	// mode specific config
	switch cc.Mode {
	case VMMode_Docker:
		cc.KubeArmorImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultKubeArmorRepo, images.KubearmorImage, cm.DefaultKubeArmorImage, images.KubearmorVersion, releaseInfo.KubeArmorTag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

		cc.KubeArmorInitImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultKubeArmorRepo, images.KubearmorInitImage, cm.DefaultKubeArmorInitImage,
			images.KubearmorVersion, releaseInfo.KubeArmorTag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

		cc.KubeArmorVMAdapterImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images.VmAdapterImage, cm.DefaultVMAdapterImage,
			images.VmAdapterTag, releaseInfo.KubeArmorVMAdapterTag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

		cc.KubeArmorRelayServerImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images.RelayServerImage, cm.DefaultRelayServerImage,
			images.KubeArmorRelayServerTag, releaseInfo.KubeArmorRelayTag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

		cc.SIAImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images.SIAImage, releaseInfo.SIAImage, images.SIAVersionTag, releaseInfo.SIATag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

		cc.PEAImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images.PEAImage, releaseInfo.PEAImage,
			images.PEAVersionTag, releaseInfo.PEATag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

		cc.FeederImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images.FeederImage, releaseInfo.FeederServiceImage,
			images.FeederVersionTag, releaseInfo.FeederServiceTag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

		cc.SPIREAgentImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images.SpireImage, cm.DefaultSPIREAgentImage,
			"latest", releaseInfo.SPIREAgentImageTag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

		cc.WaitForItImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images.WaitForItImage, cm.DefaultWaitForItImage,
			"latest", "", "", "", preserveUpstream)
		if err != nil {
			return err
		}

		cc.DiscoverImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images.DiscoverImage, releaseInfo.DiscoverImage,
			images.DiscoverVersionTag, releaseInfo.DiscoverTag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

		cc.SumEngineImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images.SumEngineImage, releaseInfo.SumEngineImage,
			images.SumEngineTag, releaseInfo.SumEngineTag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

		cc.HardeningAgentImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images.HardeningAgentImage, releaseInfo.HardeningAgentImage,
			images.HardeningAgentVersionTag, releaseInfo.HardeningAgentTag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

		cc.RMQImage, err = getImage(registry, cm.DefaultDockerRegistry,
			"", images.RMQImage, cm.DefaultRMQImage,
			"", cm.DefaultRMQImageTag, "", "", preserveUpstream)
		if err != nil {
			return err
		}
		cc.RRAImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images.RRAImage, releaseInfo.RraImage,
			images.RRAImageTag, releaseInfo.RraTag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

	case VMMode_Systemd:
		kaVersion := images.KubearmorVersion
		if images.KubearmorVersion != "" && (images.KubearmorVersion == "stable" || images.KubearmorVersion == "latest") {
			fmt.Printf("%s tag not available for systemd package. Using values from release chart", images.KubearmorVersion)
			kaVersion = ""
		}
		cc.KubeArmorImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultKubeArmorRepo, images.KubearmorImage, cm.AgentRepos[cm.KubeArmor],
			kaVersion, releaseInfo.KubeArmorTag, "v", tagSuffix, preserveUpstream)
		if err != nil {
			return err
		}

		cc.KubeArmorVMAdapterImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images.VmAdapterImage, cm.AgentRepos[cm.VMAdapter],
			images.VmAdapterTag, releaseInfo.KubeArmorVMAdapterTag, "v", tagSuffix, preserveUpstream)
		if err != nil {
			return err
		}

		cc.KubeArmorRelayServerImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images.RelayServerImage, cm.AgentRepos[cm.RelayServer],
			images.KubeArmorRelayServerTag, releaseInfo.KubeArmorRelayTag, "v", tagSuffix, preserveUpstream)
		if err != nil {
			return err
		}

		cc.SIAImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images.SIAImage, cm.AgentRepos[cm.SIAAgent],
			images.SIAVersionTag, releaseInfo.SIATag, "v", tagSuffix, preserveUpstream)
		if err != nil {
			return err
		}

		cc.PEAImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images.PEAImage, cm.AgentRepos[cm.PEAAgent],
			images.PEAVersionTag, releaseInfo.PEATag, "v", tagSuffix, preserveUpstream)
		if err != nil {
			return err
		}

		cc.FeederImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images.FeederImage, cm.AgentRepos[cm.FeederService],
			images.FeederVersionTag, releaseInfo.FeederServiceTag, "v", tagSuffix, preserveUpstream)
		if err != nil {
			return err
		}

		cc.SPIREAgentImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images.SpireImage, cm.AgentRepos[cm.SpireAgent],
			"", releaseInfo.SPIREAgentImageTag, "v", tagSuffix, preserveUpstream)
		if err != nil {
			return err
		}

		cc.SumEngineImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images.SumEngineImage, cm.AgentRepos[cm.SummaryEngine],
			images.SumEngineTag, releaseInfo.SumEngineTag, "v", tagSuffix, preserveUpstream)
		if err != nil {
			return err
		}

		cc.DiscoverImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images.DiscoverImage, cm.AgentRepos[cm.DiscoverAgent],
			images.DiscoverVersionTag, releaseInfo.DiscoverTag, "v", tagSuffix, preserveUpstream)
		if err != nil {
			return err
		}

		cc.HardeningAgentImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images.HardeningAgentImage, cm.AgentRepos[cm.HardeningAgent],
			images.HardeningAgentVersionTag, releaseInfo.HardeningAgentTag, "v", tagSuffix, preserveUpstream)
		if err != nil {
			return err
		}

		cc.RRAImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, images.RRAImage, cm.AgentRepos[cm.RRA],
			images.RRAImageTag, releaseInfo.RraTag, "v", cm.SystemdTagSuffix, preserveUpstream)
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

func resolveSource(source, version string) (string, string, error) {
	if isURL(source) {
		fmt.Println("Source detected as URL, downloading...")
		folder, err := downloadToTemp(source)
		if err != nil {
			return "", "", err
		}
		source = folder
	}

	if _, err := os.Stat(source); err != nil {
		return "", "", fmt.Errorf("file does not exist: %s", source)
	}

	releaseVersion := version

	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.Name() == "release.json" {

			// #nosec G306,G304  -- parameters are controlled
			data, err := os.ReadFile(filepath.Join(source, info.Name()))
			if err != nil {
				return fmt.Errorf("failed to get release.json: %w", err)
			}
			var releaseData map[string]any
			if err := json.Unmarshal(data, &releaseData); err != nil {
				return err
			}

			if version == "" {
				v, ok := releaseData["version"].(string)
				if !ok {
					return fmt.Errorf("failed to get version from release.json")
				}
				releaseVersion = v
			} else {
				if releaseData["version"] != version {
					return fmt.Errorf("version mismatch: %v != %v", releaseData["version"], version)
				}
			}

		}

		if info.IsDir() {
			return nil
		}
		if shouldExtract(info.Name()) {
			fileName := filepath.Join(source, info.Name())
			if _, err := extractTarGz(fileName); err != nil {
				return err
			}
		}
		return nil

	})

	if err != nil {
		return "", "", err
	}

	return source, releaseVersion, nil
}

func isURL(input string) bool {
	u, err := url.Parse(input)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func createRandomTempFolder() (string, error) {
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

	tempFolder, err := createRandomTempFolder()
	if err != nil {
		return "", err
	}

	tempFile := filepath.Join(tempFolder, "file")

	if err := os.MkdirAll(tempFolder, 0750); err != nil {
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

	folder, err := extractTarGz(tempFile)
	if err != nil {
		return "", err
	}

	return folder, os.RemoveAll(tempFile)

}

func extractTarGz(fileName string) (string, error) {
	ctx := context.Background()

	f, err := os.Open(filepath.Clean(fileName))
	if err != nil {
		return "", err
	}
	defer f.Close()

	format, input, err := archives.Identify(ctx, fileName, f)
	if err != nil {
		return "", err
	}

	extractor, ok := format.(archives.Extractor)
	if !ok {
		return "", fmt.Errorf("unsupported archive format: %s", format)
	}

	tempFolder := filepath.Dir(fileName)

	handler := func(ctx context.Context, f archives.FileInfo) error {
		targetPath := filepath.Join(tempFolder, f.NameInArchive)
		if f.IsDir() {
			return os.MkdirAll(targetPath, f.Mode())
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0750); err != nil {
			return err
		}

		out, err := os.OpenFile(filepath.Clean(targetPath), os.O_CREATE|os.O_RDWR, f.Mode())
		if err != nil {
			return err
		}
		defer out.Close()

		in, err := f.Open()
		if err != nil {
			return err
		}
		defer in.Close()

		_, err = io.Copy(out, in)
		return err
	}

	err = extractor.Extract(ctx, input, handler)
	if err != nil {
		return "", err
	}
	return tempFolder, nil
}

func shouldExtract(path string) bool {
	return slices.ContainsFunc([]string{
		"systemd-arm64.tar.gz",
		"systemd-amd64.tar.gz",
		"docker-arm64.tar.gz",
		"docker-amd64.tar.gz",
	}, func(s string) bool {
		return strings.Contains(path, s)
	})
}
