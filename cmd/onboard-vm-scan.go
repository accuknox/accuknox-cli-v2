package cmd

import (
	"fmt"

	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/spf13/cobra"
)

// onboardVM scan represents the sub-command to onboard VM clusters
var vmScanCmd = &cobra.Command{
	Use:   "scanner",
	Short: "sub-command for onboarding RAT(risk assessment tool)",
	Long:  "sub-command for onboarding RAT(risk assessment tool)",
	RunE: func(cmd *cobra.Command, args []string) error {

		// create cluster config
		var cc onboard.ClusterConfig
		cc.EnableVMScan = true
		// create RAT config
		cc.InitRATConfig(authToken, url, tenantID, clusterID, clusterName, label, schedule, profile, benchmark, registry, registryConfigPath, insecure, plainHTTP, ratImage, ratTag, releaseVersion, preserveUpstream)
		err := cc.InstallRAT()
		if err != nil {
			fmt.Println("error", err)
		}
		//err := onboard.InstallRAT()
		// if err != nil {
		// 	fmt.Println("error")
		// }
		// create and template service files

		// install RAT

		return nil
	},
}

func init() {

	// all flags are optional
	// add a mode flag here for systemd or docker
	vmScanCmd.PersistentFlags().StringVar((*string)(&profile), "profile", "", "ubuntu - rhel")
	vmScanCmd.PersistentFlags().StringVar((*string)(&benchmark), "benchmark", "", "stig,soc2")
	vmScanCmd.PersistentFlags().StringVar((*string)(&schedule), "schedule", "*-*-* 00:00:00", "schedule for RAT to run (default value once a day)")
	vmScanCmd.PersistentFlags().StringVar((*string)(&authToken), "auth-token", "", "authentication token")
	vmScanCmd.PersistentFlags().StringVar((*string)(&tenantID), "tenant-id", "", "tenant id of the cluster")
	vmScanCmd.PersistentFlags().StringVar((*string)(&clusterName), "cluster-name", "", "cluster name")
	vmScanCmd.PersistentFlags().StringVar((*string)(&clusterID), "cluster-id", "", "cluster id")
	vmScanCmd.PersistentFlags().StringVar((*string)(&url), "url", "", "url")
	vmScanCmd.PersistentFlags().StringVar((*string)(&label), "label", "", "label")

	onboardVMCmd.AddCommand(vmScanCmd)
	// TODO: hide global flags from here as they are not useful here
}
