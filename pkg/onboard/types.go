package onboard

type ClusterType string

const (
	ClusterType_VM  ClusterType = "vm"
	ClusterType_ECS ClusterType = "ecs"
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
	SIAImage                  string
	PEAImage                  string
	FeederImage               string

	CPNodeAddr string

	WorkerNode bool
	DryRun     bool

	ImagePullPolicy ImagePullPolicy

	// internal
	composeCmd string
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
	// kubearmor configuration
	KubeArmorImage            string
	KubeArmorInitImage        string
	KubeArmorVMAdapterImage   string
	KubeArmorRelayServerImage string
	SIAImage                  string
	PEAImage                  string
	FeederImage               string

	ImagePullPolicy string

	KubeArmorPort string
	Hostname      string

	// vm-adapter configuration
	KubeArmorURL   string
	RelayServerURL string
	SIAAddr        string
	PEAAddr        string
	WorkerNode     bool

	// generic agent configuration
	ConfigPath string

	// feeder service configuration
	RelayServerAddr string
	RelayServerPort string

	// policy-enforcement-agent config
	PPSHost string

	// spire agent
	JoinToken           string
	SpireHostAddr       string
	SpireHostPort       string
	SpireTrustBundleURL string
}

type KmuxConfigTemplateArgs struct {
	StreamName string
	ServerURL  string
}
