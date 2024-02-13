package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	dryRun         bool
	nodeAddr       string
	userConfigPath string
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

	onboardCmd.PersistentFlags().StringVarP(&userConfigPath, "config-path", "", "", "path to read configuration files from")
	err := onboardCmd.MarkPersistentFlagDirname("config-path")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	onboardCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "only generate manifests and don't onboard anything")
	onboardCmd.PersistentFlags().Lookup("dry-run").NoOptDefVal = "true"

	rootCmd.AddCommand(onboardCmd)
}
