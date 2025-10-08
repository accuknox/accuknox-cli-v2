package cmd

// This package will provide the kubearmor and its agents compatibility with the host vm.

import (
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/accuknox/accuknox-cli-v2/pkg/vm"
	"github.com/spf13/cobra"
)

var options vm.Options

// vmInspectCmd represents the vm command for inspect
var vmInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect VM for compatibility",
	Long:  "Inspect VM for compatibility with KubeArmor and its agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := vm.InspectVM(&options)
		if err != nil {
			logger.Error(err.Error())
			return err
		}
		return nil
	},
}

// ========== //
// == Init == //
// ========== //

func init() {
	vmCmd.PersistentFlags().StringVar(&options.CPNodeAddr, "cp-node-addr", "", "address of control plane")                                           // check control plane connectivity
	vmCmd.PersistentFlags().StringVar(&options.SpireMetricsURL, "spire-metrics-url", "http://spire.accuknox.com:8081", "address of control plane")   // spire metricsurl
	vmCmd.PersistentFlags().StringVar(&options.SpireReadyURL, "spire-ready-url", "http://spire.accuknox.com:9090/ready", "address of control plane") // spire ready url
	vmCmd.PersistentFlags().StringVar(&options.PPSURL, "pps-url", "https://pps.accuknox.com:443", "address of control plane")                        // pps url
	vmCmd.PersistentFlags().StringVar(&options.KnoxGwURL, "knox-gw-url", "http://knox-gw.accuknox.com:3000", "address of control plane")             // knox-gw-url

	vmCmd.AddCommand(vmInspectCmd)
}
