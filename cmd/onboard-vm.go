package cmd

import (
	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/spf13/cobra"
)

var (
	clusterType onboard.ClusterType
	vmMode      onboard.VMMode

	kubearmorVersion string
	releaseVersion   string

	// for systemd mode
	vmAdapterTag string

	kubeArmorImage          string
	kubeArmorInitImage      string
	kubeArmorVMAdapterImage string
	imagePullPolicy         string

	preserveUpstream bool

	visibility       string
	hostVisibility   string
	audit            string
	block            string
	hostAudit        string
	hostBlock        string
	cidr             string
	kubeArmorPolicy  bool
	secureContainers bool

	alertThrottling bool
	maxAlertPerSec  int
	throttleSec     int

	skipBTF           bool
	systemMonitorPath string

	// different meaning for both worker node but
	// declared here as common global variables
	rmqAddress string
	nodeAddr   string
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
	onboardVMCmd.PersistentFlags().StringVar(&hostVisibility, "hostViz", "process,network", "Kubearmor host visibility. Possible values: \"none\" or any combo of [process,network,file,capabilities]")
	onboardVMCmd.PersistentFlags().StringVar(&audit, "audit", "", "Kubearmor container audit posture. Possible values: \"all\" or combo of [file,network,capabilities]")
	onboardVMCmd.PersistentFlags().StringVar(&block, "block", "", "Kubearmor container block posture. Possible values: \"all\" or combo of [file,network,capabilities]")
	onboardVMCmd.PersistentFlags().StringVar(&hostAudit, "hostAudit", "", "Kubearmor host audit posture. Possible values: \"all\" or combo of [file,network,capabilities]")
	onboardVMCmd.PersistentFlags().StringVar(&hostBlock, "hostBlock", "", "Kubearmor host block posture. Possible values: \"all\" or combo of [file,network,capabilities]")

	onboardVMCmd.PersistentFlags().BoolVarP(&alertThrottling, "alert-throttling", "", true, "to toggle alert-throttling")
	onboardVMCmd.PersistentFlags().IntVarP(&maxAlertPerSec, "max-alerts-per-sec", "", 10, "specifes maximum alert rate past which throttling will be triggered")
	onboardVMCmd.PersistentFlags().IntVarP(&throttleSec, "throttle-sec", "", 30, "duration (in seconds) for which subsequent alerts will be dropped once alert throttling comes into action")

	onboardVMCmd.PersistentFlags().StringVar(&rmqAddress, "rmq-address", "", "RabbitMQ address")

	onboardVMCmd.PersistentFlags().StringVar(&cidr, "network-cidr", "172.20.32.0/27", "CIDR for accuknox network")

	onboardCmd.AddCommand(onboardVMCmd)
}
