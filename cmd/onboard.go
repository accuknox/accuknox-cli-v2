/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/spf13/cobra"
)

var (
	clusterType      string
	kubearmorVersion string
	agentVersion     string

	kubeArmorImage            string
	kubeArmorInitImage        string
	kubeArmorVMAdapterImage   string
	kubeArmorRelayServerImage string
	siaImage                  string
	peaImage                  string
	feederImage               string
)

// onboardCmd represents the cluster command
var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Parent command for onboarding non-k8s clusters",
	Long:  "Parent command for onboarding non-k8s clusters",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := cmd.Help()
		if err != nil {
			return err
		}

		return nil
	},
}

func init() {
	// local configuration
	onboardCmd.PersistentFlags().StringVarP(&clusterType, "type", "t", "", "type of cluster to onboard. possible values VM")
	onboardCmd.PersistentFlags().StringVarP(&kubearmorVersion, "kubearmor-version", "", "stable", "version of KubeArmor to use")
	onboardCmd.PersistentFlags().StringVarP(&agentVersion, "version", "v", "", "version of agents to be used")

	onboardCmd.PersistentFlags().StringVar(&kubeArmorImage, "kubearmor-image", "", "KubeArmor image to use")
	onboardCmd.PersistentFlags().StringVar(&kubeArmorInitImage, "kubearmor-init-image", "", "KubeArmor init image to use")
	onboardCmd.PersistentFlags().StringVar(&kubeArmorVMAdapterImage, "kubearmor-vm-adapter-image", "", "KubeArmor vm-adapter image to use")
	onboardCmd.PersistentFlags().StringVar(&kubeArmorRelayServerImage, "kubearmor-relay-server", "", "KubeArmor relay-server image to use")
	onboardCmd.PersistentFlags().StringVar(&siaImage, "sia-image", "", "sia image to use")
	onboardCmd.PersistentFlags().StringVar(&peaImage, "pea-image", "", "pea image to use")
	onboardCmd.PersistentFlags().StringVar(&feederImage, "feeder-image", "", "feeder-service image to use")

	onboardCmd.MarkPersistentFlagRequired("type")
	onboardCmd.MarkPersistentFlagRequired("version")

	rootCmd.AddCommand(onboardCmd)
}
