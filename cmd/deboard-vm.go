package cmd

import (
	"github.com/spf13/cobra"
)

var disableVMScan bool

// deboardVMCmd represents the deboard command
var deboardVMCmd = &cobra.Command{
	Use:   "vm",
	Short: "Deboard your VM cluster from SaaS",
	Long:  "Deboard your VM cluster from SaaS",
}

func init() {
	deboardCmd.PersistentFlags().StringVar((*string)(&vmMode), "vm-mode", "", "Mode of installation (systemd/docker)")
	deboardCmd.PersistentFlags().BoolVar(&disableVMScan, "disbale-vmscan", true, "Remove rat installation")
	deboardCmd.AddCommand(deboardVMCmd)
}
