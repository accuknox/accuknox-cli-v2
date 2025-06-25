package cmd

import (
	"os"

	"github.com/accuknox/accuknox-cli-v2/pkg/deboard"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/spf13/cobra"
)

// deboardNodeCmd represents the deboardNode command
var deboardRRAScanner = &cobra.Command{
	Use:   "scanner",
	Short: "Deboard RRA scanner",
	RunE: func(cmd *cobra.Command, args []string) error {

		err := deboard.UninstallRRA()
		if err != nil {
			if os.IsNotExist(err) {
				logger.Info1("RRA Installation not found")
			} else {
				logger.Error("error removing RRA installation:%s", err.Error())
				return err
			}
		} else {
			logger.PrintSuccess("RRA uninstalled successfully.")
		}
		return nil
	},
}

func init() {
	deboardVMCmd.AddCommand(deboardRRAScanner)
}
