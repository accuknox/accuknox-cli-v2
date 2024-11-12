package cmd

import (
	"fmt"

	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/fatih/color"
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
		_, err := cc.ValidateEnv()
		if vmMode == "" {
			if err == nil {
				vmMode = onboard.VMMode_Docker
			} else {
				fmt.Print(color.YellowString("Warning: Docker requirements did not match:\n%s.\nFalling back to systemd mode for installation.\n", err.Error()))
				vmMode = onboard.VMMode_Systemd
			}
		} else if vmMode == onboard.VMMode_Docker && err != nil {
			// docker mode specified explicitly but requirements didn't match
			return fmt.Errorf(color.RedString("failed to validate environment: %s", err.Error()))
		}

		cc.EnableVMScan = true
		// create RAT config
		cc.Mode = vmMode
		cc.InitRATConfig(authToken, url, tenantID, clusterID, clusterName, label, schedule, profile, benchmark, registry, registryConfigPath, insecure, plainHTTP, ratImage, ratTag, releaseVersion, preserveUpstream)
		err = cc.InstallRAT()
		if err != nil {
			return err
		}

		return nil
	},
}

func init() {

	// all flags are optional
	// add a mode flag here for systemd or docker

	onboardVMCmd.AddCommand(vmScanCmd)
	// TODO: hide global flags from here as they are not useful here
}
