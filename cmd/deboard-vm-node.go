package cmd

import (
	"fmt"
	"os"

	"github.com/accuknox/accuknox-cli-v2/pkg/deboard"
	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/spf13/cobra"
)

// deboardNodeCmd represents the deboardNode command
var deboardNodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Deboard a worker node",
	Long:  "Deboard a worker node",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, err := deboard.Deboard(onboard.NodeType_WorkerNode, dryRun)
		if err != nil && os.IsPermission(err) {
			fmt.Println("Please remove any remaining resources at", configPath)
		} else if err != nil {
			return fmt.Errorf("Failed to deboard worker node: %s", err.Error())
		}

		fmt.Println("Worker node deboarded successfully.")
		return nil
	},
}

func init() {
	deboardVMCmd.AddCommand(deboardNodeCmd)
}
