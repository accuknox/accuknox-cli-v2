package cmd

import (
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/spf13/cobra"
)

// onboardVM scan represents the sub-command to onboard VM clusters
var onboardVmScanCmd = &cobra.Command{
	Use:   "scanner",
	Short: "sub-command for onboarding RRA(risk assessment tool)",
	Long:  "sub-command for onboarding RRA(risk assessment tool)",
	RunE: func(cmd *cobra.Command, args []string) error {

		// create cluster config
		var cc onboard.ClusterConfig
		_, err := cc.ValidateEnv()
		if vmMode == "" {
			if err == nil {
				vmMode = onboard.VMMode_Docker
			} else {
				logger.Warn("Warning: Docker requirements did not match:\n%s.\nFalling back to systemd mode for installation.\n", err.Error())
				vmMode = onboard.VMMode_Systemd
			}
		} else if vmMode == onboard.VMMode_Docker && err != nil {
			// docker mode specified explicitly but requirements didn't match
			logger.Error("failed to validate environment: %s", err.Error())
			return err

		}
		cc.EnableVMScan = true
		// create RRA config
		cc.Mode = vmMode
		err = cc.InitRRAConfig(authToken, url, tenantID, clusterID, clusterName, label, schedule, profile, benchmark, registry, registryConfigPath, insecure, plainHTTP, rraImage, rraTag, releaseVersion, preserveUpstream)
		if err != nil {
			logger.Error(" failed to initialize RRA config:%s", err.Error())
			return err
		}
		err = cc.InstallRRA()
		if err != nil {
			logger.Error("failed to install RRA: %s", err.Error())
			return err
		}

		logger.PrintSuccess("RRA installed successfully!!")
		return nil
	},
}

func init() {
	onboardVmScanCmd.PersistentFlags().StringVarP(&releaseVersion, "version", "v", "", "agents release version to use")

	// all flags are optional
	onboardVMCmd.AddCommand(onboardVmScanCmd)
}
