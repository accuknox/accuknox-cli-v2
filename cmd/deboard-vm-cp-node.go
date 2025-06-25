package cmd

import (
	"os"

	"github.com/accuknox/accuknox-cli-v2/pkg/deboard"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/spf13/cobra"
)

// cpNodeCmd represents the cpNode command
var deboardCpNodeCmd = &cobra.Command{
	Use:   "cp-node",
	Short: "Deboard control plane node",
	Long:  "Deboard control plane node",
	RunE: func(cmd *cobra.Command, args []string) error {

		if vmMode == "" {
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
			_, err := deboard.Deboard(onboard.NodeType_ControlPlane, vmMode, dryRun)
			if err != nil {
				logger.Error("Failed to deboard control plane node: %s", err.Error())
				return err
			}

		case onboard.VMMode_Docker:
			configPath, err := deboard.Deboard(onboard.NodeType_ControlPlane, vmMode, dryRun)
			if err != nil && os.IsPermission(err) {
				logger.Warn("Please remove any remaining resources at %s", configPath)
			} else if err != nil {
				logger.Error("Failed to deboard control plane node: %s", err.Error())
				return err
			}

		default:
			logger.Error("vm mode: %s invalid, accepted values (docker/systemd)", vmMode)
		}
		if disableVMScan {
			logger.Info1("Removing RRA installation if it exists")
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
		}
		logger.PrintSuccess("Control plane node deboarded successfully.")
		return nil
	},
}

func init() {
	deboardVMCmd.AddCommand(deboardCpNodeCmd)
}
