package onboard

import (
	"errors"

	"oras.land/oras-go/v2/registry/remote/auth"
)

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
	DefaultConfigPath  string
	UserConfigPath     string
	RegistryConfigPath string

	ClusterType      ClusterType
	KubeArmorVersion string
	AgentsVersion    string

	KubeArmorImage            string
	KubeArmorInitImage        string
	KubeArmorVMAdapterImage   string
	KubeArmorRelayServerImage string
	SPIREAgentImage           string
	WaitForItImage            string
	SIAImage                  string
	PEAImage                  string
	FeederImage               string
	RMQImage                  string
	DiscoverImage             string
	SumEngineImage            string
	HardeningAgentImage       string
	RATAgentImage             string

	CPNodeAddr string

	WorkerNode bool
	DryRun     bool
	DeployRMQ  bool

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
	AlertThrottling           bool
	MaxAlertsPerSec           int
	ThrottleSec               int

	// summary engine config
	ProcessOperation bool
	FileOperation    bool
	NetworkOperation bool

	CIDR string

	TemplateFuncs map[string]interface{}

	// internal
	composeCmd     string
	composeVersion string

	//kubearmor systemd configs
	Mode VMMode

	// container security
	SecureContainers bool
	// host policy discovery
	EnableHostPolicyDiscovery bool

	SkipBTFCheck      bool
	SystemMonitorPath string

	SystemdServiceObjects []SystemdServiceObject
	DeploySumengine       bool
	RMQServer             string

	// Risk assessment scanning
	EnableVMScan    bool
	RATConfigObject RATConfig

	PlainHTTP   bool
	InsecureTLS bool
	ORASClient  *auth.Client

	// tls configs
	CaCert         string
	RMQCredentials string

	RMQTopicPrefix string

	Tls TLS
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
	HardenAddr      string

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

	KubeArmorAlertThrottling bool
	KubeArmorMaxAlertsPerSec int
	KubeArmorThrottleSec     int

	SPIREAgentImage string
	WaitForItImage  string

	SIAImage            string
	PEAImage            string
	FeederImage         string
	RMQImage            string
	DiscoverImage       string
	SumEngineImage      string
	HardeningAgentImage string

	DiscoverRules   string
	ImagePullPolicy string

	KubeArmorAddr string
	KubeArmorPort string
	Hostname      string

	// vm-adapter configuration
	KubeArmorURL   string
	RelayServerURL string
	SIAAddr        string
	PEAAddr        string
	HardenAddr     string
	RMQAddr        string

	WorkerNode bool
	DeployRMQ  bool

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
	PoliciesKmuxConfig string
	StateKmuxConfig    string
	AlertsKmuxConfig   string
	LogsKmuxConfig     string
	KmuxConfigPath     string
	PolicyKmuxConfig   string
	SummaryKmuxConfig  string

	// container security
	SecureContainers bool
	// host policy discovery
	EnableHostPolicyDiscovery bool

	//summary engine configuration
	ProcessOperation bool
	FileOperation    bool
	NetworkOperation bool

	RMQTlsPort      string
	RMQPasswordHash string
	RMQUsername     string
	RMQPassword     string
	RMQServer       string
	RMQTopicPrefix  string

	TlsEnabled  bool
	TlsCertFile string

	// topic config
	PoliciesTopic   string
	StateEventTopic string
	LogsTopic       string
	AlertsTopic     string
	PolicyV1Topic   string
	SummaryV2Topic  string
}

type KmuxConfigTemplateArgs struct {
	ReleaseVersion  string
	StreamName      string
	ServerURL       string
	RMQServer       string
	RMQUsername     string
	RMQPassword     string
	TlsEnabled      bool
	TlsCertFile     string
	ConsumerTag     string
	ExchangeType    string
	ExchangeName    string
	QueueName       string
	QueueDurability bool
}

type TokenResponse struct {
	// if success join_token and message will be populated
	JoinToken string `json:"join_token"`
	Message   string `json:"message"`

	// if failure error_code and error_message will be populated
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
}

type SystemdServiceObject struct {
	// generic
	AgentName   string
	PackageName string
	ServiceName string

	AgentDir              string
	ConfigFilePath        string
	ServiceTemplateString string
	TimerTemplateString   string
	ConfigTemplateString  string

	// TODO: Package instead of just tag
	AgentImage string
	//AgentPackage string
	InstallOnWorkerNode bool

	KmuxConfigPath           string
	KmuxConfigTemplateString string
	KmuxConfigFileName       string

	// map of file name and path
	ExtraFilePathSrc  map[string]string
	ExtraFilePathDest map[string]string
}

type RATConfig struct {
	AuthToken   string
	Url         string
	TenantId    string
	ClusterName string
	ClusterId   string
	Label       string
	Schedule    string
	Benchmark   string
	Profile     string
}

var (
	ErrInvalidToken = errors.New("invalid JWT format")
)

const AccessKeyEndpoint = "/access-token/api/v1/process"

type TLS struct {
	CaPath         string
	Generate       bool
	Enabled        bool
	CaCert         string
	RMQCredentials string
	Organization   []string
	CommonName     string
	IPs            []string
	DNS            []string
}
