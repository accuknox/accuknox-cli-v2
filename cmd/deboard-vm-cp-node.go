package cmd

import (
	"fmt"
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
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, err := deboard.Deboard(onboard.NodeType_ControlPlane, dryRun)
		if err != nil && os.IsPermission(err) {
			fmt.Println("Please remove any remaining resources at", configPath)
		} else if err != nil {
			return fmt.Errorf("Failed to deboard control plane node: %s", err.Error())
		}

		fmt.Println("Control plane node deboarded successfully.")
		return nil
	},
}

func init() {
	deboardVMCmd.AddCommand(deboardCpNodeCmd)
}
