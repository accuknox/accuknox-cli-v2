package cmd

import (
	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/spf13/cobra"
)

var (
	clusterType onboard.ClusterType
	vmMode      onboard.VMMode
	tls         onboard.TLS
	splunk      onboard.SplunkConfig

	kubearmorVersion string
	releaseVersion   string

	// for systemd mode
	vmAdapterTag string
	rraTag       string

	kubeArmorImage          string
	kubeArmorInitImage      string
	kubeArmorVMAdapterImage string
	imagePullPolicy         string
	rraImage                string

	preserveUpstream bool

	visibility       string
	hostVisibility   string
	audit            string
	block            string
	hostAudit        string
	hostBlock        string
	cidr             string
	kubeArmorPolicy  bool
	topicPrefix      string
	secureContainers bool

	alertThrottling bool
	maxAlertPerSec  int
	throttleSec     int

	skipBTF           bool
	systemMonitorPath string

	//flags for RRA scan
	enableVMScan bool
	profile      string
	benchmark    string
	schedule     string
	authToken    string
	url          string
	tenantID     string
	clusterName  string
	clusterID    string
	label        string

	// different meaning for both worker node but
	// declared here as common global variables
	rmqAddress string
	nodeAddr   string

	rmqConnectionName string

	sumEngineVisibility  string
	sumEngineCronTime    string
	nodeStateRefreshTime int

	joinToken string
	spireHost string
	spireDir  = "/opt/spire-agent/spire-data"

	logRotate string

	parallel int

	releaseFile string

	printInspectOutput bool
	kaResource         onboard.ResourceConfig
	agentsResource     onboard.ResourceConfig
)

// onboardVMCmd represents the sub-command to onboard VM clusters
var onboardVMCmd = &cobra.Command{
	Use:   "vm",
	Short: "sub-command for onboarding VM clusters",
	Long:  "sub-command for onboarding VM clusters",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := cmd.Help()
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	// all flags are optional
	// add a mode flag here for systemd or docker
	onboardVMCmd.PersistentFlags().StringVar((*string)(&vmMode), "vm-mode", "", "Mode of installation (systemd/docker)")
	onboardVMCmd.PersistentFlags().BoolVar(&secureContainers, "secure-containers", true, "to monitor containers")

	onboardVMCmd.PersistentFlags().BoolVar(&skipBTF, "skip-btf-check", false, "to install even if BTF is not present")
	onboardVMCmd.PersistentFlags().StringVar(&systemMonitorPath, "system-monitor-path", "", "path to system monitor, must be specified is BTF not present")

	onboardVMCmd.PersistentFlags().StringVar(&vmAdapterTag, "vm-adapter-tag", "", "version tag for vm adapter")
	onboardVMCmd.PersistentFlags().StringVar(&kubeArmorImage, "kubearmor-image", "", "KubeArmor image to use")
	onboardVMCmd.PersistentFlags().StringVar(&kubeArmorInitImage, "kubearmor-init-image", "", "KubeArmor init image to use")
	onboardVMCmd.PersistentFlags().StringVar(&kubeArmorVMAdapterImage, "kubearmor-vm-adapter-image", "", "KubeArmor vm-adapter image to use")

	onboardVMCmd.PersistentFlags().BoolVarP(&preserveUpstream, "preserve-upstream-repo", "", true, "to keep upstream repo name e.g \"accuknox\" from accuknox/shared-informer-agent")

	onboardVMCmd.PersistentFlags().StringVar(&imagePullPolicy, "image-pull-policy", "always", "image pull policy to use. Either of: missing | never | always")
	onboardVMCmd.PersistentFlags().StringVar(&visibility, "viz", "process,network", "Kubearmor visibility. Possible values: \"none\" or any combo of [process,network,file]")
	onboardVMCmd.PersistentFlags().StringVar(&audit, "audit", "", "Kubearmor container audit posture. Possible values: \"all\" or combo of [file,network,capabilities]")
	onboardVMCmd.PersistentFlags().StringVar(&block, "block", "", "Kubearmor container block posture. Possible values: \"all\" or combo of [file,network,capabilities]")

	// deprecated but kept for backwards compat
	onboardVMCmd.PersistentFlags().StringVar(&hostVisibility, "hostViz", "process,network", "Kubearmor host visibility. Possible values: \"none\" or any combo of [process,network,file,capabilities]")
	onboardVMCmd.PersistentFlags().StringVar(&hostAudit, "hostAudit", "", "Kubearmor host audit posture. Possible values: \"all\" or combo of [file,network,capabilities]")
	onboardVMCmd.PersistentFlags().StringVar(&hostBlock, "hostBlock", "", "Kubearmor host block posture. Possible values: \"all\" or combo of [file,network,capabilities]")

	// earlier flags were not POSIX compliant
	onboardVMCmd.PersistentFlags().StringVar(&hostVisibility, "host-viz", "process,network", "Kubearmor host visibility. Possible values: \"none\" or any combo of [process,network,file,capabilities]")
	onboardVMCmd.PersistentFlags().StringVar(&hostAudit, "host-audit", "", "Kubearmor host audit posture. Possible values: \"all\" or combo of [file,network,capabilities]")
	onboardVMCmd.PersistentFlags().StringVar(&hostBlock, "host-block", "", "Kubearmor host block posture. Possible values: \"all\" or combo of [file,network,capabilities]")

	onboardVMCmd.MarkFlagsMutuallyExclusive("hostViz", "host-viz")
	onboardVMCmd.MarkFlagsMutuallyExclusive("hostAudit", "host-audit")
	onboardVMCmd.MarkFlagsMutuallyExclusive("hostBlock", "host-block")

	onboardVMCmd.PersistentFlags().BoolVarP(&alertThrottling, "alert-throttling", "", true, "to toggle alert-throttling")
	onboardVMCmd.PersistentFlags().IntVarP(&maxAlertPerSec, "max-alerts-per-sec", "", 10, "specifies maximum alert rate past which throttling will be triggered")
	onboardVMCmd.PersistentFlags().IntVarP(&throttleSec, "throttle-sec", "", 30, "duration (in seconds) for which subsequent alerts will be dropped once alert throttling comes into action")

	onboardVMCmd.PersistentFlags().StringVar(&rmqAddress, "rmq-address", "", "RabbitMQ address")

	onboardVMCmd.PersistentFlags().StringVar(&cidr, "network-cidr", "172.20.32.0/27", "CIDR for accuknox network")

	onboardVMCmd.PersistentFlags().StringVar(&sumEngineImage, "sumengine-image", "", "summary-engine image to use")
	onboardVMCmd.PersistentFlags().StringVar(&sumEngineVersionTag, "sumengine-version", "", "summary-engine version to use")

	onboardVMCmd.PersistentFlags().StringVar(&sumEngineCronTime, "sumengine-cron-time", "15m", "cron time for summary-engine in minutes (default: 15m)")

	onboardVMCmd.PersistentFlags().BoolVar(&tls.Enabled, "tls", false, "enable TLS for rabbitmq connection")
	onboardVMCmd.PersistentFlags().BoolVar(&tls.Generate, "tls-gen", false, "generate TLS certificates for rabbitmq connection (generates CA, Cert, and Key)")
	onboardVMCmd.PersistentFlags().StringVar(&tls.CaPath, "ca-path", "", "path to ca certificate file")

	onboardVMCmd.PersistentFlags().StringVar(&topicPrefix, "cp-name", "", "control plane node name to be used as topic prefix")

	onboardVMCmd.PersistentFlags().StringArrayVar(&tls.Organization, "tls-org", []string{"accuknox"}, "Organization for TLS certificates")

	onboardVMCmd.PersistentFlags().StringVar(&tls.CommonName, "tls-cn", "accuknox", "CommonName for TLS certificates")

	onboardVMCmd.PersistentFlags().StringVar(&tls.RMQCredentials, "auth", "", "rabbitmq credentials in base64 encoded key:value format")
	onboardVMCmd.PersistentFlags().StringArrayVar(&tls.DNS, "dns", []string{}, "DNS names for TLS certificates")

	onboardVMCmd.PersistentFlags().StringArrayVar(&tls.IPs, "ips", []string{}, "List of IPs for TLS certificates")

	onboardVMCmd.MarkFlagsMutuallyExclusive("tls-gen", "ca-path")
	onboardVMCmd.PersistentFlags().StringVar(&sumEngineVisibility, "sumengine-viz", "process,network,file", "Events other than these won't be processed by summary engine. Possible values: \"none\" or any combo of [process,network,file]")

	onboardVMCmd.PersistentFlags().StringVar(&logRotate, "log-rotate", "50M", "log rotate file size. Acceptable format similar to journalctl(10K, 200M, 2G, etc). Default: 50M")

	// flags for RRA
	onboardVMCmd.PersistentFlags().StringVar(&rraTag, "rra-tag", "", "version tag for RRA( rapid risk assessment tool)")
	onboardVMCmd.PersistentFlags().StringVar(&rraImage, "rra-image", "", "RRA(Rapid Risk assessment tool) image to use")
	onboardVMCmd.PersistentFlags().BoolVarP(&enableVMScan, "enable-vmscan", "", false, " Set to true to install RRA along with other kubearmor and accuknox-agents ")
	onboardVMCmd.PersistentFlags().StringVar((*string)(&profile), "profile", "", "ubuntu,rhel")
	onboardVMCmd.PersistentFlags().StringVar((*string)(&benchmark), "benchmark", "", "security benchmark (stig,soc2)")
	onboardVMCmd.PersistentFlags().StringVar((*string)(&schedule), "schedule", "", "schedule for RRA to run")
	onboardVMCmd.PersistentFlags().StringVar((*string)(&authToken), "auth-token", "", "authentication token")
	onboardVMCmd.PersistentFlags().StringVar((*string)(&tenantID), "tenant-id", "", "tenant id of the cluster")
	onboardVMCmd.PersistentFlags().StringVar((*string)(&clusterName), "cluster-name", "", "cluster name")
	onboardVMCmd.PersistentFlags().StringVar((*string)(&clusterID), "cluster-id", "", "cluster id")
	onboardVMCmd.PersistentFlags().StringVar((*string)(&url), "url", "", "url")
	onboardVMCmd.PersistentFlags().StringVar((*string)(&label), "label", "", "label")
	onboardVMCmd.MarkFlagsRequiredTogether("benchmark", "profile", "auth-token", "url", "tenant-id", "cluster-name", "label", "schedule")

	// splunk flags
	onboardVMCmd.PersistentFlags().BoolVar(&splunk.Enabled, "splunk", false, "enable Splunk")
	onboardVMCmd.PersistentFlags().StringVar(&splunk.Url, "splunk-url", "", "Splunk url")
	onboardVMCmd.PersistentFlags().StringVar(&splunk.Token, "splunk-token", "", "Splunk token")
	onboardVMCmd.PersistentFlags().StringVar(&splunk.Index, "splunk-index", "", "Splunk index")
	onboardVMCmd.PersistentFlags().StringVar(&splunk.Source, "splunk-source", "", "Splunk source")
	onboardVMCmd.PersistentFlags().StringVar(&splunk.SourceType, "splunk-sourcetype", "", "Splunk sourcetype")
	onboardVMCmd.PersistentFlags().StringVar(&splunk.Certificate, "splunk-cert", "", "Splunk certificate in base64 encoded format")
	onboardVMCmd.PersistentFlags().BoolVar(&splunk.SkipTls, "splunk-skip-tls", false, "Skip tls verification")

	onboardVMCmd.PersistentFlags().StringVar(&rmqConnectionName, "rmq-connection-name", "", "Rabbitmq connection name")

	onboardVMCmd.PersistentFlags().IntVar(&nodeStateRefreshTime, "node-state-refresh-time", 10, "Refresh time for node state (default 10 minutes)")

	onboardVMCmd.PersistentFlags().StringVar(&spireHost, "spire-host", "", "address of spire-host to connect for authenticating with accuknox SaaS")

	onboardVMCmd.PersistentFlags().StringVar(&vmName, "vm-name", "", "vm name for onboarding")

	onboardVMCmd.PersistentFlags().StringVar(&spireAgentImage, "spire-agent-image", "", "spire-agent image to use")
	onboardVMCmd.PersistentFlags().StringVar(&waitForItImage, "wait-for-it-image", "", "wait-for-it image to use")

	onboardVMCmd.PersistentFlags().IntVar(&parallel, "parallel", 0, "number of images to pull in parallel (0 for unlimited, 1 for sequential, >1 for limited parallelism)")

	onboardVMCmd.PersistentFlags().StringVar(&knoxGateway, "knox-gateway", "", "address of knox-gateway to connect with for pushing telemetry data")

	onboardVMCmd.PersistentFlags().StringVar(&releaseFile, "release-file", "", "release file containing release versions of accuknox agents")

	onboardVMCmd.PersistentFlags().BoolVar(&printInspectOutput, "print-inspect", false, "print output of inspect command")

	// resource config for agents and kubearmor
	onboardVMCmd.PersistentFlags().Int64Var(&kaResource.CPUQuota, "kubearmor.cpu-quota", 5, "cpu quota for kubearmor in percentage. eg: 5")
	onboardVMCmd.PersistentFlags().Int64Var(&kaResource.MemoryMax, "kubearmor.memory-max", 300, "memory max for kubearmor in MB. eg: 300")
	onboardVMCmd.PersistentFlags().Int64Var(&kaResource.MemoryHigh, "kubearmor.memory-high", 240, "memory quota for kubearmor in MB. eg: 240")

	onboardVMCmd.PersistentFlags().Int64Var(&agentsResource.CPUQuota, "agents.cpu-quota", 5, "cpu quota for agents in percentage. eg: 5")
	onboardVMCmd.PersistentFlags().Int64Var(&agentsResource.MemoryMax, "agents.memory-max", 100, "memory max for agents in MB. eg: 100")
	onboardVMCmd.PersistentFlags().Int64Var(&agentsResource.MemoryHigh, "agents.memory-high", 80, "memory quota for agents in MB. eg: 80")

	onboardVMCmd.PersistentFlags().BoolVar(&proxy.Enabled, "proxy", false, "bypass spire and use proxy")

	onboardVMCmd.PersistentFlags().StringVar(&proxy.Address, "proxy-address", "", "proxy address")
	onboardVMCmd.PersistentFlags().StringVar(&proxy.SaaSAddr, "proxy-addr-saas", "", "saas proxy address")

	onboardVMCmd.PersistentFlags().StringArrayVar(&proxy.ExtraArgs, "proxy-args", []string{}, "extra env variables for proxy")

	onboardCmd.AddCommand(onboardVMCmd)
}
