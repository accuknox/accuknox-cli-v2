/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// deboardCmd represents the deboard command
var deboardCmd = &cobra.Command{
	Use:   "deboard",
	Short: "Deboard your cluster from SaaS",
	Long:  "Deboard your cluster from SaaS",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("deboard called")
	},
}

func init() {
	deboardCmd.PersistentFlags().StringVarP(&clusterType, "type", "t", "", "type of cluster to onboard. possible values VM")

	deboardCmd.MarkPersistentFlagRequired("type")

	rootCmd.AddCommand(deboardCmd)
}
