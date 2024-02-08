/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"log"

	"github.com/accuknox/accuknox-cli-v2/pkg/deboard"
	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/spf13/cobra"
)

// deboardNodeCmd represents the deboardNode command
var deboardNodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Deboard a worker node",
	Long:  "Deboard a worker node",
	Run: func(cmd *cobra.Command, args []string) {
		err := deboard.Deboard(onboard.NodeType_WorkerNode)
		if err != nil {
			log.Fatalln("Failed to deboard worker node:", err.Error())
		}

		log.Println("Worker node deboarded successfully.")
	},
}

func init() {
	deboardCmd.AddCommand(deboardNodeCmd)
}
