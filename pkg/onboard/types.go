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
	DefaultConfigPath  string `json:"default_config_path,omitempty"`
	UserConfigPath     string `json:"user_config_path,omitempty"`
	RegistryConfigPath string `json:"registry_config_path,omitempty"`

	ClusterType      ClusterType `json:"cluster_type,omitempty"`
	KubeArmorVersion string      `json:"kubearmor_version,omitempty"`
	AgentsVersion    string      `json:"agents_version,omitempty"`

	KubeArmorImage            string `json:"kubearmor_image,omitempty"`
	KubeArmorInitImage        string `json:"kubearmor_init_image,omitempty"`
	KubeArmorVMAdapterImage   string `json:"kubearmor_vm_adapter_image,omitempty"`
	KubeArmorRelayServerImage string `json:"kubearmor_relay_server_image,omitempty"`
	SPIREAgentImage           string `json:"spire_agent_image,omitempty"`
	WaitForItImage            string `json:"wait_for_it_image,omitempty"`
	SIAImage                  string `json:"sia_image,omitempty"`
	PEAImage                  string `json:"pea_image,omitempty"`
	FeederImage               string `json:"feeder_image,omitempty"`
	RMQImage                  string `json:"rmq_image,omitempty"`
	DiscoverImage             string `json:"discover_image,omitempty"`
	SumEngineImage            string `json:"sumengine_image,omitempty"`
	HardeningAgentImage       string `json:"hardening_agent_image,omitempty"`
	RATImage                  string `json:"rat_image,omitempty"`

	CPNodeAddr string `json:"cp_node_addr,omitempty"`

	WorkerNode bool `json:"worker_node,omitempty"`
	DryRun     bool `json:"dry_run,omitempty"`
	DeployRMQ  bool `json:"deploy_rmq,omitempty"`

	ImagePullPolicy ImagePullPolicy `json:"image_pull_policy,omitempty"`

	// KubeArmor config
	Visibility                string `json:"visibility,omitempty"`
	HostVisibility            string `json:"host_visibility,omitempty"`
	DefaultFilePosture        string `json:"default_file_posture,omitempty"`
	DefaultNetworkPosture     string `json:"default_network_posture,omitempty"`
	DefaultCapPosture         string `json:"default_cap_posture,omitempty"`
	DefaultHostFilePosture    string `json:"default_host_file_posture,omitempty"`
	DefaultHostNetworkPosture string `json:"default_host_network_posture,omitempty"`
	DefaultHostCapPosture     string `json:"default_host_cap_posture,omitempty"`
	AlertThrottling           bool   `json:"alert_throttling,omitempty"`
	MaxAlertsPerSec           int    `json:"max_alerts_per_sec,omitempty"`
	ThrottleSec               int    `json:"throttle_sec,omitempty"`

	// summary engine config
	ProcessOperation bool `json:"process_operation,omitempty"`
	FileOperation    bool `json:"file_operation,omitempty"`
	NetworkOperation bool `json:"network_operation,omitempty"`

	CIDR string `json:"cidr,omitempty"`

	TemplateFuncs map[string]interface{} `json:"-"`

	// internal
	composeCmd     string
	composeVersion string

	//kubearmor systemd configs
	Mode VMMode `json:"mode,omitempty"`

	// container security
	SecureContainers bool `json:"secure_containers,omitempty"`
	// host policy discovery
	EnableHostPolicyDiscovery bool `json:"enable_host_policy_discovery,omitempty"`

	SkipBTFCheck      bool   `json:"skip_btf_check,omitempty"`
	SystemMonitorPath string `json:"system_monitor_path,omitempty"`

	SystemdServiceObjects []SystemdServiceObject `json:"-"`
	DeploySumengine       bool                   `json:"deploy_sumengine,omitempty"`
	RMQServer             string                 `json:"rmq_server,omitempty"`

	// Risk assessment scanning
	EnableVMScan    bool      `json:"enable_vmscan,omitempty"`
	RATConfigObject RATConfig `json:"-"`

	PlainHTTP   bool         `json:"plain_http,omitempty"`
	InsecureTLS bool         `json:"insecure_tls,omitempty"`
	ORASClient  *auth.Client `json:"-"`

	// tls configs
	CaCert         string `json:"ca_cert,omitempty"`
	RMQCredentials string `json:"rmq_credentials,omitempty"`

	RMQTopicPrefix string `json:"rmq_topic_prefix,omitempty"`

	Tls    TLS          `json:"tls,omitempty"`
	Splunk SplunkConfig `json:"splunk,omitempty"`
}

type InitConfig struct {
	// basic
	ClusterConfig `json:"cluster_config,omitempty"`
	JoinToken     string `json:"join_token,omitempty"`
	SpireHost     string `json:"spire_host,omitempty"`
	PPSHost       string `json:"pps_host,omitempty"`
	KnoxGateway   string `json:"knox_gateway,omitempty"`

	// advanced
	SpireTrustBundleURL string `json:"spire_trust_bundle_url,omitempty"`
	EnableLogs          bool   `json:"enable_logs,omitempty"`

	// internal
	TCArgs TemplateConfigArgs `json:"tc_args,omitempty"`
}

type JoinConfig struct {
	ClusterConfig   `json:"cluster_config,omitempty"`
	KubeArmorAddr   string `json:"kubearmor_addr,omitempty"`
	RelayServerAddr string `json:"relay_server_addr,omitempty"`
	SIAAddr         string `json:"sia_addr,omitempty"`
	PEAAddr         string `json:"pea_addr,omitempty"`
	HardenAddr      string `json:"harden_addr,omitempty"`

	// internal
	TCArgs TemplateConfigArgs `json:"tc_args,omitempty"`
}

type TemplateConfigArgs struct {
	ReleaseVersion string `json:"release_version,omitempty"`

	// kubearmor configuration
	KubeArmorImage            string `json:"kubearmor_image,omitempty"`
	KubeArmorInitImage        string `json:"kubearmor_init_image,omitempty"`
	KubeArmorVMAdapterImage   string `json:"kubearmor_vm_adapter_image,omitempty"`
	KubeArmorRelayServerImage string `json:"kubearmor_relay_server_image,omitempty"`

	KubeArmorVisibility     string `json:"kubearmor_visibility,omitempty"`
	KubeArmorHostVisibility string `json:"kubearmor_host_visibility,omitempty"`

	KubeArmorFilePosture    string `json:"kubearmor_file_posture,omitempty"`
	KubeArmorNetworkPosture string `json:"kubearmor_network_posture,omitempty"`
	KubeArmorCapPosture     string `json:"kubearmor_cap_posture,omitempty"`

	KubeArmorHostFilePosture    string `json:"kubearmor_host_file_posture,omitempty"`
	KubeArmorHostNetworkPosture string `json:"kubearmor_host_network_posture,omitempty"`
	KubeArmorHostCapPosture     string `json:"kubearmor_host_cap_posture,omitempty"`

	KubeArmorAlertThrottling bool `json:"kubearmor_alert_throttling,omitempty"`
	KubeArmorMaxAlertsPerSec int  `json:"kubearmor_max_alerts_per_sec,omitempty"`
	KubeArmorThrottleSec     int  `json:"kubearmor_throttle_sec,omitempty"`

	SPIREAgentImage string `json:"spire_agent_image,omitempty"`
	WaitForItImage  string `json:"wait_for_it_image,omitempty"`

	SIAImage            string `json:"sia_image,omitempty"`
	PEAImage            string `json:"pea_image,omitempty"`
	FeederImage         string `json:"feeder_image,omitempty"`
	RMQImage            string `json:"rmq_image,omitempty"`
	DiscoverImage       string `json:"discover_image,omitempty"`
	SumEngineImage      string `json:"sumengine_image,omitempty"`
	HardeningAgentImage string `json:"hardening_agent_image,omitempty"`

	DiscoverRules   string `json:"discover_rules,omitempty"`
	ImagePullPolicy string `json:"image_pull_policy,omitempty"`

	KubeArmorAddr string `json:"kubearmor_addr,omitempty"`
	KubeArmorPort string `json:"kubearmor_port,omitempty"`
	Hostname      string `json:"hostname,omitempty"`

	// vm-adapter configuration
	KubeArmorURL   string `json:"kubearmor_url,omitempty"`
	RelayServerURL string `json:"relay_server_url,omitempty"`
	SIAAddr        string `json:"sia_addr,omitempty"`
	PEAAddr        string `json:"pea_addr,omitempty"`
	HardenAddr     string `json:"harden_addr,omitempty"`
	RMQAddr        string `json:"rmq_addr,omitempty"`

	WorkerNode bool `json:"worker_node,omitempty"`
	DeployRMQ  bool `json:"deploy_rmq,omitempty"`

	VmMode VMMode `json:"vm_mode,omitempty"`

	// generic agent configuration
	ConfigPath string `json:"config_path,omitempty"`

	// feeder service configuration
	RelayServerAddr string `json:"relay_server_addr,omitempty"`
	RelayServerPort string `json:"relay_server_port,omitempty"`
	EnableLogs      bool   `json:"enable_logs,omitempty"`

	// policy-enforcement-agent config
	PPSHost string `json:"pps_host,omitempty"`

	// spire agent
	JoinToken           string `json:"join_token,omitempty"`
	SpireHostAddr       string `json:"spire_host_addr,omitempty"`
	SpireHostPort       string `json:"spire_host_port,omitempty"`
	SpireTrustBundleURL string `json:"spire_trust_bundle_url,omitempty"`

	// docker config
	NetworkCIDR string `json:"network_cidr,omitempty"`

	// kmux config paths for agents
	PoliciesKmuxConfig string `json:"policies_kmux_config,omitempty"`
	StateKmuxConfig    string `json:"state_kmux_config,omitempty"`
	AlertsKmuxConfig   string `json:"alerts_kmux_config,omitempty"`
	LogsKmuxConfig     string `json:"logs_kmux_config,omitempty"`
	KmuxConfigPath     string `json:"kmux_config_path,omitempty"`
	PolicyKmuxConfig   string `json:"policy_kmux_config,omitempty"`
	SummaryKmuxConfig  string `json:"summary_kmux_config,omitempty"`

	// container security
	SecureContainers bool `json:"secure_containers,omitempty"`
	// host policy discovery
	EnableHostPolicyDiscovery bool `json:"enable_host_policy_discovery,omitempty"`

	//summary engine configuration
	ProcessOperation bool `json:"process_operation,omitempty"`
	FileOperation    bool `json:"file_operation,omitempty"`
	NetworkOperation bool `json:"network_operation,omitempty"`

	RMQTlsPort      string `json:"rmq_tls_port,omitempty"`
	RMQPasswordHash string `json:"rmq_password_hash,omitempty"`
	RMQUsername     string `json:"rmq_username,omitempty"`
	RMQPassword     string `json:"rmq_password,omitempty"`
	RMQServer       string `json:"rmq_server,omitempty"`
	RMQTopicPrefix  string `json:"rmq_topic_prefix,omitempty"`

	TlsEnabled  bool   `json:"tls_enabled,omitempty"`
	TlsCertFile string `json:"tls_cert_file,omitempty"`

	// topic config
	PoliciesTopic   string `json:"policies_topic,omitempty"`
	StateEventTopic string `json:"state_event_topic,omitempty"`
	LogsTopic       string `json:"logs_topic,omitempty"`
	AlertsTopic     string `json:"alerts_topic,omitempty"`
	PolicyV1Topic   string `json:"policyv1_topic,omitempty"`
	SummaryV2Topic  string `json:"summaryv2_topic,omitempty"`

	// rat configs
	RATConfigObject RATConfig `json:"-"`

	// splunk config
	SplunkConfigObject SplunkConfig `json:"-"`
}

type KmuxConfigTemplateArgs struct {
	ReleaseVersion  string `json:"release_version,omitempty"`
	StreamName      string `json:"stream_name,omitempty"`
	ServerURL       string `json:"server_url,omitempty"`
	RMQServer       string `json:"rmq_server,omitempty"`
	RMQUsername     string `json:"rmq_username,omitempty"`
	RMQPassword     string `json:"rmq_password,omitempty"`
	TlsEnabled      bool   `json:"tls_enabled,omitempty"`
	TlsCertFile     string `json:"tls_cert_file,omitempty"`
	ConsumerTag     string `json:"consumer_tag,omitempty"`
	ExchangeType    string `json:"exchange_type,omitempty"`
	ExchangeName    string `json:"exchange_name,omitempty"`
	QueueName       string `json:"queue_name,omitempty"`
	QueueDurability bool   `json:"queue_durability,omitempty"`
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
	Hostname     string
	RATImage     string
	EnableVMScan bool
	AuthToken    string
	Url          string
	TenantID     string
	ClusterName  string
	ClusterID    string
	Label        string
	Schedule     string
	Benchmark    string
	Profile      string
}

var (
	ErrInvalidToken = errors.New("invalid JWT format")
)

const AccessKeyEndpoint = "/access-token/api/v1/process"

type TLS struct {
	CaPath         string   `json:"ca_path,omitempty"`
	Generate       bool     `json:"generate,omitempty"`
	Enabled        bool     `json:"enabled,omitempty"`
	CaCert         string   `json:"ca_cert,omitempty"`
	RMQCredentials string   `json:"rmq_credentials,omitempty"`
	Organization   []string `json:"organization,omitempty"`
	CommonName     string   `json:"common_name,omitempty"`
	IPs            []string `json:"ips,omitempty"`
	DNS            []string `json:"dns,omitempty"`
}

type SplunkConfig struct {
	Enabled     bool
	Url         string
	Token       string
	Source      string
	SourceType  string
	Index       string
	Certificate string
	SkipTls     bool
}
