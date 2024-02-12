/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"log"
	"os"

	"github.com/accuknox/accuknox-cli-v2/pkg/deboard"
	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/spf13/cobra"
)

// cpNodeCmd represents the cpNode command
var deboardCpNodeCmd = &cobra.Command{
	Use:   "cp-node",
	Short: "Deboard control plane node",
	Long:  "Deboard control plane node",
	Run: func(cmd *cobra.Command, args []string) {
		configPath, err := deboard.Deboard(onboard.NodeType_ControlPlane, dryRun)
		if err != nil && os.IsPermission(err) {
			log.Println("Please remove any remaining resources at %s.", configPath)
		} else {
			log.Fatalln("Failed to deboard control plane node:", err.Error())
		}

		log.Println("Control plane node deboarded successfully.")
	},
}

func init() {
	deboardCmd.AddCommand(deboardCpNodeCmd)
}
