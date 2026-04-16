package cmd

import (
	"github.com/accuknox/accuknox-cli-v2/pkg/vm"
	"github.com/spf13/cobra"
)

var exclude []string

var restCmd = &cobra.Command{
	Use:   "agent-reset",
	Short: "Command for resetting restart counter for all agents",
	Long:  "Command for resetting restart counter for all agents",
	RunE: func(cmd *cobra.Command, args []string) error {

		return vm.Reset(exclude)
	},
}

func init() {
	vmCmd.AddCommand(restCmd)
	restCmd.PersistentFlags().StringSliceVar(&exclude, "exclude", []string{}, "agents that doesn't need restart counter reset")

}
