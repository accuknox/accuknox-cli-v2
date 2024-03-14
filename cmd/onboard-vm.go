package cmd

import (
	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/spf13/cobra"
)

var (
	clusterType      onboard.ClusterType
	kubearmorVersion string
	releaseVersion   string

	kubeArmorImage          string
	kubeArmorInitImage      string
	kubeArmorVMAdapterImage string
	imagePullPolicy         string

	visibility     string
	hostVisibility string
	audit          string
	block          string

	cidr string
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
	onboardVMCmd.PersistentFlags().StringVar(&kubeArmorImage, "kubearmor-image", "", "KubeArmor image to use")
	onboardVMCmd.PersistentFlags().StringVar(&kubeArmorInitImage, "kubearmor-init-image", "", "KubeArmor init image to use")
	onboardVMCmd.PersistentFlags().StringVar(&kubeArmorVMAdapterImage, "kubearmor-vm-adapter-image", "", "KubeArmor vm-adapter image to use")

	onboardVMCmd.PersistentFlags().StringVar(&imagePullPolicy, "image-pull-policy", "always", "image pull policy to use. Either of: missing | never | always")
	onboardVMCmd.PersistentFlags().StringVar(&visibility, "viz", "process,network", "Kubearmor visibility. Possible values: \"none\" or any of [process,network,file]")
	onboardVMCmd.PersistentFlags().StringVar(&hostVisibility, "hostViz", "process,network", "Kubearmor host visibility. Possible values: \"none\" or any of [process,network,file,capabilities]")
	onboardVMCmd.PersistentFlags().StringVar(&audit, "audit", "", "Kubearmor audit posture. Possible values: \"all\" or any of [file,network,capabilities]")
	onboardVMCmd.PersistentFlags().StringVar(&block, "block", "", "Kubearmor block posture. Possible values: \"all\" or any of [file,network,capabilities]")

	onboardVMCmd.PersistentFlags().StringVar(&cidr, "network-cidr", "172.20.32.0/27", "CIDR for accuknox network")

	onboardCmd.AddCommand(onboardVMCmd)
}
