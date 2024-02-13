package cmd

import (
	"github.com/spf13/cobra"
)

// deboardCmd represents the deboard command
var deboardCmd = &cobra.Command{
	Use:   "deboard",
	Short: "Deboard your cluster from SaaS",
	Long:  "Deboard your cluster from SaaS",
}

func init() {
	deboardCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "only generate manifests and don't onboard anything")
	deboardCmd.PersistentFlags().Lookup("dry-run").NoOptDefVal = "true"

	rootCmd.AddCommand(deboardCmd)
}
