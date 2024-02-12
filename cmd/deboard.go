/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"log"

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
		log.Fatalln(err)
	}

	rootCmd.AddCommand(deboardCmd)
}
