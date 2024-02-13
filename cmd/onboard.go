package cmd

import (
	"github.com/spf13/cobra"
)

var (
	dryRun   bool
	nodeAddr string
)

// onboardCmd represents the onboard non-k8s cluster command
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
	onboardCmd.PersistentFlags().StringVarP(&kubearmorVersion, "kubearmor-version", "", "stable", "version of KubeArmor to use")

	onboardCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "only generate manifests and don't onboard anything")
	onboardCmd.PersistentFlags().Lookup("dry-run").NoOptDefVal = "true"

	rootCmd.AddCommand(onboardCmd)
}
