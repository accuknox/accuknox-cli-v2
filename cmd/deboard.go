package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// deboardCmd represents the deboard command
var deboardCmd = &cobra.Command{
	Use:   "deboard",
	Short: "Deboard your cluster from SaaS",
	Long:  "Deboard your cluster from SaaS",
}

func init() {
	deboardCmd.PersistentFlags().StringVarP(&clusterType, "type", "t", "", "type of cluster to onboard. possible values VM")

	deboardCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "only generate manifests and don't onboard anything")
	deboardCmd.PersistentFlags().Lookup("dry-run").NoOptDefVal = "true"

	err := deboardCmd.MarkPersistentFlagRequired("type")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	rootCmd.AddCommand(deboardCmd)
}
