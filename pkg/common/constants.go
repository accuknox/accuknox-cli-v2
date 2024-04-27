package common

const (
	SpecialRegexChars = `.*+?()|[]{}^$`

	ServiceName          = "discovery-engine" // Subject to change
	Port           int64 = 8090
	DELabel              = "app=discovery-engine"
	AccuknoxAgents       = "accuknox-agents"

	APIGroupCilium            = "cilium.io"
	APIGroupKubearmorSecurity = "security.kubearmor.com"
	APIGroupNetworking        = "networking.k8s.io"
	APIGroupRBACAuth          = "rbac.authorization.k8s.io"

	ClusterRole                 = "ClusterRole"
	ServiceAccount              = "ServiceAccount"
	GRPC                        = "grpc"
	AMQP                        = "amqp"
	AMQPPort              int32 = 5672
	GRPCPort              int32 = 8090
	Management                  = "management"
	ManagementPort        int32 = 15672
	CRDName                     = "discoveredpolicies.security.kubearmor.com"
	ConfigMapDirPath            = "pkg/configmaps"
	ClusterRoleViewName         = "dev2-view-cluster-resources"
	ClusterRoleManageName       = "dev2-manage-policies"

	SumEngine          = "summary-engine"
	SumEngineImage     = "accuknox/dev2-sumengine:latest"
	Offlaoder          = "offloader"
	OffloaderImage     = "public.ecr.aws/k9v9d5v2/discovery-engine-offloader:v0.1.0"
	Discover           = "discover"
	DiscoverImage      = "accx3435/dev2-discover:v1"
	Rabbitmq           = "rabbitmq"
	RabbitmqImage      = "rabbitmq:3.12.2-management"
	Hardening          = "hardening"
	HardeningImage     = "accuknox/dev2-hardening:latest"
	ServiceAccountName = "discovery-engine"

	DiscoverConfMap  = "dev2-discover"
	HardeningConfMap = "dev2-hardening"
	OffloaderConfMap = "dev2-offloader"
	SumengineConfmap = "dev2-sumengine"

	AccuknoxGithub  = "accuknox"
	AccuknoxCLIRepo = "accuknox-cli-v2"

	DefaultConfigPathDirName = ".accuknox-config"

	// KubeArmor related image/image registries are fixed as of now
	DefaultKubeArmorImage     = "kubearmor/kubearmor:"
	DefaultKubeArmorInitImage = "kubearmor/kubearmor-init:"
	DefaultRelayServerImage   = "accuknox/kubearmor-relay-server:"
	DefaultVMAdapterImage     = "accuknox/vm-adapter:"

	// Agent images change/have changed over release versions
	// deprecated - do not use
	DefaultPEAImage    = "public.ecr.aws/k9v9d5v2/policy-enforcement-agent:"
	DefaultSIAImage    = "public.ecr.aws/k9v9d5v2/shared-informer-agent:"
	DefaultFeederImage = "public.ecr.aws/k9v9d5v2/feeder-service:"

	MinDockerVersion                  = "v19.0.3"
	MinDockerComposeVersion           = "v1.27.0"
	MinDockerComposeWithWaitSupported = "v2.17.0"
)

var (
	MatchLabels = map[string]string{"app": "discovery-engine"}
	// TODO: Add action and few other coloumns in network
	// SysProcHeader variable contains source process, destination process path, count, timestamp and status
	SysProcHeader = []string{"Src Process", "Destination Process Path", "Count", "Last Updated Time"}
	// SysFileHeader variable contains source process, destination file path, count, timestamp and status
	SysFileHeader = []string{"Src Process", "Destination File Path", "Count", "Last Updated Time"}
	// SysNwHeader variable contains protocol, command, POD/SVC/IP, Port, Namespace, and Labels
	SysNwHeader = []string{"Protocol", "Command", "POD/SVC/IP", "Port", "Count", "Last Updated Time"}
	// SysBindNwHeader variable contains protocol, command, Bind Port, Bind Address, count and timestamp
	SysBindNwHeader = []string{"Protocol", "Command", "Bind Port", "Bind Address", "Count", "Last Updated Time"}
)
