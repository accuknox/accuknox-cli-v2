package cmd

import (
	"os"

	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/accuknox/accuknox-cli-v2/pkg/vm"
	"github.com/spf13/cobra"
)

var path string
var save bool

// vmPolicyCmd represents the vm command for policy enforcement
var vmScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "RAT scan for vms",
	Long:  "RAT scan for vms",
	RunE: func(cmd *cobra.Command, args []string) error {

		var err error
		rracmdArgs := vm.PrepareRRACommand(profile, benchmark, authToken, label, url, tenantID, clusterName, clusterID)

		// set homedir as path to store result if no path has been provided by the user
		if path == "" {
			path, err = os.UserHomeDir()
			if err != nil {
				logger.Error("Error getting home directory:", err)
			}
		}
		logger.Info1("Running RRA tests")
		err = vm.ExecCommand(rracmdArgs, path, benchmark, save)
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
	vmCmd.AddCommand(vmScanCmd)

	// flags for RAT
	vmScanCmd.PersistentFlags().StringVar((*string)(&profile), "profile", "", "ubuntu,rhel")
	vmScanCmd.PersistentFlags().StringVar((*string)(&benchmark), "benchmark", "", "security benchmark (stig,soc2)")
	vmScanCmd.PersistentFlags().StringVar((*string)(&authToken), "auth-token", "", "authentication token")
	vmScanCmd.PersistentFlags().StringVar((*string)(&tenantID), "tenant-id", "", "tenant id of the cluster")
	vmScanCmd.PersistentFlags().StringVar((*string)(&clusterName), "cluster-name", "", "cluster name")
	vmScanCmd.PersistentFlags().StringVar((*string)(&clusterID), "cluster-id", "", "cluster id")
	vmScanCmd.PersistentFlags().StringVar((*string)(&url), "url", "", "url")
	vmScanCmd.PersistentFlags().StringVar((*string)(&label), "label", "", "label")
	vmScanCmd.PersistentFlags().BoolVar((*bool)(&save), "save", false, "save json output")
	vmScanCmd.PersistentFlags().StringVar((*string)(&path), "path", "", "path of output file")
	vmScanCmd.MarkFlagsRequiredTogether("benchmark", "profile", "auth-token", "url", "tenant-id", "cluster-name", "label")

}
