package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	dryRun             bool
	userConfigPath     string
	registryConfigPath string
	registry           string
	plainHTTP          bool
	insecure           bool
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
	onboardCmd.PersistentFlags().StringVarP(&kubearmorVersion, "kubearmor-version", "", "", "version of KubeArmor to use")

	onboardCmd.PersistentFlags().StringVarP(&userConfigPath, "config-path", "", "", "path to read configuration files from")
	err := onboardCmd.MarkPersistentFlagDirname("config-path")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	onboardCmd.PersistentFlags().StringVarP(&registry, "registry", "r", "docker.io", "the registry to authneticate with (default - DockerHub)")
	onboardCmd.PersistentFlags().StringVarP(&registryConfigPath, "registry-config-path", "", "", "path to pre-existing OCI registry config")

	onboardCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "only generate manifests and don't onboard anything")
	onboardCmd.PersistentFlags().Lookup("dry-run").NoOptDefVal = "true"

	// TODO: custom CA path support
	onboardCmd.PersistentFlags().BoolVarP(&plainHTTP, "plain-http", "", false, "use plain HTTP everywhere")
	onboardCmd.PersistentFlags().BoolVarP(&insecure, "insecure", "", false, "skip verifying TLS certs")
	onboardCmd.PersistentFlags().Lookup("plain-http").NoOptDefVal = "true"
	onboardCmd.PersistentFlags().Lookup("insecure").NoOptDefVal = "true"
	onboardCmd.PersistentFlags().StringVar(&joinToken, "join-token", "", "join-token to use")

	rootCmd.AddCommand(onboardCmd)
}
