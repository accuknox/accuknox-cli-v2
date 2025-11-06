package cmd

import (
	"fmt"
	"os"

	"github.com/accuknox/accuknox-cli-v2/pkg/deboard"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// deboardNodeCmd represents the deboardNode command
var deboardNodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Deboard a worker node",
	Long:  "Deboard a worker node",
	RunE: func(cmd *cobra.Command, args []string) error {

		if vmMode == "" {
			// look for systemd and docker mode
			installedSystemdServices, err := onboard.CheckInstalledSystemdServices()
			if err != nil {
				logger.Error("error checking systemd files: %s", err.Error())
				return err
			}

			if len(installedSystemdServices) > 0 {
				vmMode = onboard.VMMode_Systemd
			} else {
				vmMode = onboard.VMMode_Docker
			}
		}

		switch vmMode {
		case onboard.VMMode_Systemd:
			_, err := deboard.Deboard(onboard.NodeType_WorkerNode, vmMode, dryRun)
			if err != nil {
				logger.Error("Failed to deboard worker node: %s", err.Error())
				return err
			}
		case onboard.VMMode_Docker:
			configPath, err := deboard.Deboard(onboard.NodeType_WorkerNode, vmMode, dryRun)
			if err != nil && os.IsPermission(err) {
				logger.Warn("Please remove any remaining resources at %s", configPath)
			} else if err != nil {
				return fmt.Errorf("%s", color.RedString("Failed to deboard worker node: %s", err.Error()))
			}
		default:
			return fmt.Errorf("%s", color.RedString("vm mode: %s invalid, accepted values (docker/systemd)", vmMode))
		}
		if disableVMScan {
			logger.Info1("Removing RRA installation if it exists")
			err := deboard.UninstallRRA()
			if err != nil {
				if os.IsNotExist(err) {
					logger.Info1("RRA Installation not found")
				} else {
					logger.Warn("error removing RRA installation:%s", err.Error())
				}
			} else {
				logger.PrintSuccess("RRA uninstalled successfully.")
			}
		}
		logger.PrintSuccess("Worker node deboarded successfully.")
		return nil
	},
}

func init() {
	deboardVMCmd.AddCommand(deboardNodeCmd)
}
