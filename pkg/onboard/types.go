package onboard

type ClusterType string
type VMMode string

const (
	ClusterType_VM  ClusterType = "vm"
	ClusterType_ECS ClusterType = "ecs"

	VMMode_Docker  VMMode = "docker"
	VMMode_Systemd VMMode = "systemd"
)

var (
	ClusterTypeValues = map[string]ClusterType{
		"vm":  ClusterType_VM,
		"ecs": ClusterType_ECS,
	}

	ClusterTypeKeys = map[ClusterType]string{
		ClusterType_VM:  "vm",
		ClusterType_ECS: "ecs",
	}
)

type NodeType string

const (
	NodeType_ControlPlane NodeType = "control-plane"
	NodeType_WorkerNode   NodeType = "worker-node"
)

var (
	NodeTypeValues = map[string]NodeType{
		"control-plane": NodeType_ControlPlane,
		"worker-node":   NodeType_WorkerNode,
	}
)

type ImagePullPolicy string

const (
	ImagePullPolicy_Always       ImagePullPolicy = "always"
	ImagePullPolicy_Never        ImagePullPolicy = "never"
	ImagePullPolicy_IfNotPresent ImagePullPolicy = "missing"
)

type ClusterConfig struct {
	DefaultConfigPath string
	UserConfigPath    string

	ClusterType      ClusterType
	KubeArmorVersion string
	AgentsVersion    string

	KubeArmorImage            string
	KubeArmorInitImage        string
	KubeArmorVMAdapterImage   string
	KubeArmorRelayServerImage string
	SPIREAgentImage           string
	SIAImage                  string
	PEAImage                  string
	FeederImage               string
	DiscoverImage             string
	SumEngineImage            string

	CPNodeAddr string

	WorkerNode bool
	DryRun     bool

	ImagePullPolicy ImagePullPolicy

	// KubeArmor config
	Visibility                string
	HostVisibility            string
	DefaultFilePosture        string
	DefaultNetworkPosture     string
	DefaultCapPosture         string
	DefaultHostFilePosture    string
	DefaultHostNetworkPosture string
	DefaultHostCapPosture     string

	CIDR string

	// internal
	composeCmd     string
	composeVersion string

	//kubearmor systemd configs
	Mode           VMMode
	KubeArmorTag   string
	VmAdapterTag   string
	RelayServerTag string
	PeaTag         string
	SiaTag         string
	SpireTag       string
	FsTag          string
	SumEngineTag   string
	DiscoverTag    string

	DeployDiscover  bool
	DeploySumengine bool

	// container security
	SecureContainers bool
}

type InitConfig struct {
	// basic
	ClusterConfig
	JoinToken   string
	SpireHost   string
	PPSHost     string
	KnoxGateway string

	// advanced
	SpireTrustBundleURL string
	EnableLogs          bool

	// internal
	TCArgs TemplateConfigArgs
}

type JoinConfig struct {
	ClusterConfig
	KubeArmorAddr   string
	RelayServerAddr string
	SIAAddr         string
	PEAAddr         string

	// internal
	TCArgs TemplateConfigArgs
}

type TemplateConfigArgs struct {
	ReleaseVersion string

	// kubearmor configuration
	KubeArmorImage            string
	KubeArmorInitImage        string
	KubeArmorVMAdapterImage   string
	KubeArmorRelayServerImage string

	KubeArmorVisibility     string
	KubeArmorHostVisibility string

	KubeArmorFilePosture    string
	KubeArmorNetworkPosture string
	KubeArmorCapPosture     string

	KubeArmorHostFilePosture    string
	KubeArmorHostNetworkPosture string
	KubeArmorHostCapPosture     string

	SPIREAgentImage string

	SIAImage       string
	PEAImage       string
	FeederImage    string
	DiscoverImage  string
	SumEngineImage string

	DeployDiscover  bool
	DeploySumengine bool

	DiscoverRules   string
	ImagePullPolicy string

	KubeArmorPort string
	Hostname      string

	// vm-adapter configuration
	KubeArmorURL   string
	RelayServerURL string
	SIAAddr        string
	PEAAddr        string
	WorkerNode     bool

	VmMode VMMode

	// generic agent configuration
	ConfigPath string

	// feeder service configuration
	RelayServerAddr string
	RelayServerPort string
	EnableLogs      bool

	// policy-enforcement-agent config
	PPSHost string

	// spire agent
	JoinToken           string
	SpireHostAddr       string
	SpireHostPort       string
	SpireTrustBundleURL string

	// docker config
	NetworkCIDR string

	// kmux config paths for agents
	KmuxConfigPathFS        string
	KmuxConfigPathSIA       string
	KmuxConfigPathPEA       string
	KmuxConfigPathDiscover  string
	KmuxConfigPathSumengine string

	// container security
	SecureContainers bool

	//summary engine configuration
	ProcessOperation bool
	FileOperation    bool
	NetworkOperation bool
}

type KmuxConfigTemplateArgs struct {
	ReleaseVersion string
	StreamName     string
	ServerURL      string
	RMQServer      string
}
