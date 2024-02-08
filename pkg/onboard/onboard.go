package onboard

import "fmt"

const (
	DefaultKubeArmorImage     = "kubearmor/kubearmor:"
	DefaultKubeArmorInitImage = "kubearmor/kubearmor-init:"
	DefaultRelayServerImage   = "accuknox/kubearmor-relay-server:"
	DefaultVMAdapterImage     = "accuknox/vm-adapter:"
	DefaultPEAImage           = "public.ecr.aws/k9v9d5v2/policy-enforcement-agent:"
	DefaultSIAImage           = "public.ecr.aws/k9v9d5v2/shared-informer-agent:"
	DefaultFeederImage        = "public.ecr.aws/k9v9d5v2/feeder-service:"
)

func CreateClusterConfig(clusterType ClusterType, kubearmorVersion, agentsVersion, kubearmorImage, kubearmorInitImage, vmAdapterImage, relayServerImage, siaImage, peaImage, feederImage string, workerNode bool) (*ClusterConfig, error) {

	cc := new(ClusterConfig)

	if kubearmorImage != "" {
		cc.KubeArmorImage = kubearmorImage
	} else if kubearmorVersion != "" {
		cc.KubeArmorImage = DefaultKubeArmorImage + kubearmorVersion
	} else {
		cc.KubeArmorImage = DefaultKubeArmorImage + "stable"
	}

	if kubearmorInitImage != "" {
		cc.KubeArmorInitImage = kubearmorInitImage
	} else if kubearmorVersion != "" {
		cc.KubeArmorInitImage = DefaultKubeArmorInitImage + kubearmorVersion
	} else {
		cc.KubeArmorInitImage = DefaultKubeArmorInitImage + "stable"
	}

	if relayServerImage != "" {
		cc.KubeArmorRelayServerImage = relayServerImage
	} else {
		cc.KubeArmorRelayServerImage = DefaultRelayServerImage + "private-relay"
	}

	if vmAdapterImage != "" {
		cc.KubeArmorVMAdapterImage = vmAdapterImage
	} else {
		cc.KubeArmorVMAdapterImage = DefaultVMAdapterImage + "latest"
	}

	cc.WorkerNode = workerNode
	if workerNode {
		return cc, nil
	}

	if siaImage != "" {
		cc.SIAImage = siaImage
	} else if agentsVersion != "" {
		cc.SIAImage = DefaultSIAImage + agentsVersion
	} else {
		return nil, fmt.Errorf("No tag found for SIA")
	}

	if peaImage != "" {
		cc.PEAImage = peaImage
	} else if agentsVersion != "" {
		cc.PEAImage = DefaultPEAImage + agentsVersion
	} else {
		return nil, fmt.Errorf("No tag found for PEA")
	}

	if feederImage != "" {
		cc.FeederImage = feederImage
	} else if agentsVersion != "" {
		cc.FeederImage = DefaultFeederImage + agentsVersion
	} else {
		return nil, fmt.Errorf("No tag found for feeder-service")
	}

	return cc, nil
}
