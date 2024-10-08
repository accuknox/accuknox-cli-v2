package cmd

import (
	"fmt"
	"os"

	"github.com/accuknox/accuknox-cli-v2/pkg/deboard"
	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/fatih/color"
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
				return fmt.Errorf(color.RedString("error checking systemd files"))
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
				return fmt.Errorf(color.RedString("Failed to deboard control plane node: %s", err.Error()))
			}

		case onboard.VMMode_Docker:
			configPath, err := deboard.Deboard(onboard.NodeType_ControlPlane, vmMode, dryRun)
			if err != nil && os.IsPermission(err) {
				fmt.Println(color.YellowString("Please remove any remaining resources at %s", configPath))
			} else if err != nil {
				return fmt.Errorf(color.RedString("Failed to deboard control plane node: %s", err.Error()))
			}

		default:
			fmt.Printf(color.RedString("vm mode: %s invalid, accepted values (docker/systemd)", vmMode))
		}

		fmt.Println(color.GreenString("Control plane node deboarded successfully."))
		return nil
	},
}

func init() {
	deboardVMCmd.AddCommand(deboardCpNodeCmd)
}
