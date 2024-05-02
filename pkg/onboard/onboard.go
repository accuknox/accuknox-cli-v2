package onboard

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
)

func CreateClusterConfig(clusterType ClusterType, userConfigPath, kubearmorVersion, releaseVersion, kubearmorImage, kubearmorInitImage, vmAdapterImage, relayServerImage, spireImage, siaImage, peaImage, feederImage, nodeAddress string, dryRun, workerNode bool, imagePullPolicy, visibility, hostVisibility, audit, block, cidr string) (*ClusterConfig, error) {

	cc := new(ClusterConfig)

	// check if a config path is given by user
	if userConfigPath != "" {
		cleanUserConfigPath := filepath.Clean(userConfigPath)
		_, err := os.Stat(userConfigPath)
		if err != nil {
			return nil, err
		}

		defaultConfigPath, err := common.GetDefaultConfigPath()
		if err != nil {
			return nil, err
		}

		// userConfigPath can't be defaultConfigPath
		if strings.Compare(defaultConfigPath, cleanUserConfigPath) == 0 {
			return nil, fmt.Errorf("config path cannot be %s", defaultConfigPath)
		}

		cc.UserConfigPath = cleanUserConfigPath
	}

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

	cc.DefaultHostFilePosture = getDefaultPosture(audit, block, "file")
	cc.DefaultHostNetworkPosture = getDefaultPosture(audit, block, "network")
	cc.DefaultHostCapPosture = getDefaultPosture(audit, block, "capabilities")

	var releaseInfo common.ReleaseMetadata
	if releaseVersion == "" {
		_, releaseInfo = common.GetLatestReleaseInfo()
	} else if releaseInfoTemp, ok := common.ReleaseInfo[releaseVersion]; ok {
		releaseInfo = releaseInfoTemp
	} else {
		// TODO: publish release JSON as OCI artifact to remove dependency
		// on needing to build knoxctl again and again
		return nil, fmt.Errorf("Unknown image tag %s", releaseVersion)
	}

	cc.AgentsVersion = releaseVersion

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

	if kubearmorImage != "" {
		cc.KubeArmorImage = kubearmorImage
	} else if kubearmorVersion != "" {
		cc.KubeArmorImage = common.DefaultKubeArmorImage + kubearmorVersion
	} else {
		cc.KubeArmorImage = common.DefaultKubeArmorImage + releaseInfo.KubeArmorTag
	}

	if kubearmorInitImage != "" {
		cc.KubeArmorInitImage = kubearmorInitImage
	} else if kubearmorVersion != "" {
		cc.KubeArmorInitImage = common.DefaultKubeArmorInitImage + kubearmorVersion
	} else {
		cc.KubeArmorInitImage = common.DefaultKubeArmorInitImage + releaseInfo.KubeArmorTag
	}

	if relayServerImage != "" {
		cc.KubeArmorRelayServerImage = relayServerImage
	} else {
		cc.KubeArmorRelayServerImage = common.DefaultRelayServerImage + releaseInfo.KubeArmorRelayTag
	}

	if vmAdapterImage != "" {
		cc.KubeArmorVMAdapterImage = vmAdapterImage
	} else {
		cc.KubeArmorVMAdapterImage = common.DefaultVMAdapterImage + releaseInfo.KubeArmorVMAdapterTag
	}

	cc.WorkerNode = workerNode
	cc.DryRun = dryRun

	cc.CPNodeAddr = nodeAddress
	if cc.CPNodeAddr == "" {
		cc.CPNodeAddr = "<address-of-this-node>"
	}

	if cidr != "" {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, err
		}

		cc.CIDR = network.String()
	}

	if workerNode {
		return cc, nil
	}

	if spireImage != "" {
		cc.SPIREAgentImage = spireImage
	} else if releaseVersion != "" {
		cc.SPIREAgentImage = "public.ecr.aws/k9v9d5v2/spire-agent:latest"
		// TODO: once the image is pushed to dockerhub
		//cc.SPIREAgentImage = "accuknox/spire-agent" + ":" + releaseInfo.SPIREAgentImageTag
	} else {
		return nil, fmt.Errorf("No tag found for spire-agent")
	}

	if siaImage != "" {
		cc.SIAImage = siaImage
	} else if releaseVersion != "" {
		cc.SIAImage = releaseInfo.SIAImage + ":" + releaseInfo.SIATag
	} else {
		return nil, fmt.Errorf("No tag found for SIA")
	}

	if peaImage != "" {
		cc.PEAImage = peaImage
	} else if releaseVersion != "" {
		cc.PEAImage = releaseInfo.PEAImage + ":" + releaseInfo.PEATag
	} else {
		return nil, fmt.Errorf("No tag found for PEA")
	}

	if feederImage != "" {
		cc.FeederImage = feederImage
	} else if releaseVersion != "" {
		cc.FeederImage = releaseInfo.FeederServiceImage + ":" + releaseInfo.FeederServiceTag
	} else {
		return nil, fmt.Errorf("No tag found for feeder-service")
	}

	return cc, nil
}

// prints join command - currently only with the default ports
// TODO: handle complex configuration
func (cc *ClusterConfig) PrintJoinCommand() {
	command := fmt.Sprintf("knoxctl onboard vm node --cp-node-addr=%s", cc.CPNodeAddr)

	fmt.Println(command)
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
