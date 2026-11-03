package cmd

import (
	"github.com/accuknox/accuknox-cli-v2/pkg/deboard"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/spf13/cobra"
)

// deboardNodeCmd represents the deboardNode command
var deboardRRAScanner = &cobra.Command{
	Use:   "scanner",
	Short: "Remove all VM scanners (Omni, RRA, and image scanner)",
	RunE: func(cmd *cobra.Command, args []string) error {

		if dryRun {
			logger.Info1("Would remove Omni, RRA, and image scanner installations")
			return nil
		}
		if err := deboard.UninstallScannersWithOutput(cmd.Context(), cmd.OutOrStdout()); err != nil {
			return err
		}
		logger.PrintSuccess("VM scanner cleanup completed.")
		return nil
	},
}

func init() {
	deboardVMCmd.AddCommand(deboardRRAScanner)
}
