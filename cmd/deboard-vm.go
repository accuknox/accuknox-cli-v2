package cmd

import (
	"github.com/spf13/cobra"
)

// deboardVMCmd represents the deboard command
var deboardVMCmd = &cobra.Command{
	Use:   "vm",
	Short: "Deboard your VM cluster from SaaS",
	Long:  "Deboard your VM cluster from SaaS",
}

func init() {
	deboardCmd.AddCommand(deboardVMCmd)
}
