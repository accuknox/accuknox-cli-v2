package onboard

import (
	"fmt"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
)

const (
	DefaultKubeArmorImage     = "kubearmor/kubearmor:"
	DefaultKubeArmorInitImage = "kubearmor/kubearmor-init:"
	DefaultRelayServerImage   = "accuknox/kubearmor-relay-server:"
	DefaultVMAdapterImage     = "accuknox/vm-adapter:"
	DefaultPEAImage           = "public.ecr.aws/k9v9d5v2/policy-enforcement-agent:"
	DefaultSIAImage           = "public.ecr.aws/k9v9d5v2/shared-informer-agent:"
	DefaultFeederImage        = "public.ecr.aws/k9v9d5v2/feeder-service:"
)

func CreateClusterConfig(clusterType ClusterType, kubearmorVersion, releaseVersion, kubearmorImage, kubearmorInitImage, vmAdapterImage, relayServerImage, siaImage, peaImage, feederImage, nodeAddress string, dryRun, workerNode bool) (*ClusterConfig, error) {

	cc := new(ClusterConfig)

	cc.ClusterType = clusterType

	var imageTags common.ImageTags
	if releaseVersion == "" {
		_, imageTags = common.GetLatestReleaseInfo()
	} else if imageTagsValue, ok := common.ReleaseInfo[releaseVersion]; ok {
		imageTags = imageTagsValue
	} else {
		return nil, fmt.Errorf("Unknown image tag %s", releaseVersion)
	}

	if kubearmorImage != "" {
		cc.KubeArmorImage = kubearmorImage
	} else if kubearmorVersion != "" {
		cc.KubeArmorImage = DefaultKubeArmorImage + kubearmorVersion
	} else {
		cc.KubeArmorImage = DefaultKubeArmorImage + imageTags.KubeArmorTag
	}

	if kubearmorInitImage != "" {
		cc.KubeArmorInitImage = kubearmorInitImage
	} else if kubearmorVersion != "" {
		cc.KubeArmorInitImage = DefaultKubeArmorInitImage + kubearmorVersion
	} else {
		cc.KubeArmorInitImage = DefaultKubeArmorInitImage + imageTags.KubeArmorTag
	}

	if relayServerImage != "" {
		cc.KubeArmorRelayServerImage = relayServerImage
	} else {
		cc.KubeArmorRelayServerImage = DefaultRelayServerImage + imageTags.KubeArmorRelayTag
	}

	if vmAdapterImage != "" {
		cc.KubeArmorVMAdapterImage = vmAdapterImage
	} else {
		cc.KubeArmorVMAdapterImage = DefaultVMAdapterImage + imageTags.KubeArmorVMAdapterTag
	}

	cc.WorkerNode = workerNode
	if workerNode {
		return cc, nil
	}

	if siaImage != "" {
		cc.SIAImage = siaImage
	} else if releaseVersion != "" {
		cc.SIAImage = DefaultSIAImage + imageTags.SIATag
	} else {
		return nil, fmt.Errorf("No tag found for SIA")
	}

	if peaImage != "" {
		cc.PEAImage = peaImage
	} else if releaseVersion != "" {
		cc.PEAImage = DefaultPEAImage + imageTags.PEATag
	} else {
		return nil, fmt.Errorf("No tag found for PEA")
	}

	if feederImage != "" {
		cc.FeederImage = feederImage
	} else if releaseVersion != "" {
		cc.FeederImage = DefaultFeederImage + imageTags.FeederServiceTag
	} else {
		return nil, fmt.Errorf("No tag found for feeder-service")
	}

	cc.DryRun = dryRun
	cc.CPNodeAddr = nodeAddress

	return cc, nil
}

// prints join command - currently only with the default ports
// TODO: handle complex configuration
func (cc *ClusterConfig) PrintJoinCommand() {
	command := fmt.Sprintf("knoxctl onboard node --type=%s --cp-addr=%s", ClusterTypeKeys[cc.ClusterType], cc.CPNodeAddr)

	fmt.Println(command)
}
