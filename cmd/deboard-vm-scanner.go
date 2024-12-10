package cmd

import (
	"fmt"
	"os"

	"github.com/accuknox/accuknox-cli-v2/pkg/deboard"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// deboardNodeCmd represents the deboardNode command
var deboardRATScanner = &cobra.Command{
	Use:   "scanner",
	Short: "Deboard RAT scanner",
	RunE: func(cmd *cobra.Command, args []string) error {

		err := deboard.UninstallRAT()
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println(color.BlueString("RAT Installation not found"))
			} else {
				return fmt.Errorf("error removing RAT installation:%s", err.Error())
			}
		} else {
			fmt.Println(color.GreenString("RAT uninstalled successfully."))
		}
		return nil
	},
}

func init() {
	deboardVMCmd.AddCommand(deboardRATScanner)
}
