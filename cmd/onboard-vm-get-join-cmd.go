package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// getJoinCmd represents the getJoinCmd command
var getJoinCmd = &cobra.Command{
	Use:   "get-join-cmd",
	Short: "Get join command for joining a worker node with the control plane node at the given adddress",
	Long:  "Get join command for joining a worker node with the control plane node at the given adddress",
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: update as more platforms added
		command := fmt.Sprintf("knoxctl onboard vm node --cp-node-addr=%s", nodeAddr)
		fmt.Println(command)

		return nil
	},
}

func init() {
	onboardVMCmd.DisableFlagParsing = true
	onboardVMCmd.AddCommand(getJoinCmd)
}
