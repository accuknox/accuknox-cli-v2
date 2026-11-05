package cmd

import (
	"fmt"
	"net"
	"slices"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/deboard"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/spf13/cobra"
)

var onboardRRAScan bool

// onboardVM scan represents the sub-command to onboard VM clusters
var onboardVmScanCmd = &cobra.Command{
	Use:   "scanner",
	Short: "Install RRA, VM vulnerability, and container image scanners",
	Long:  "Install RRA by default. Use --rra-scan=false to disable RRA, --vm-scanner=true to install the scheduled VM vulnerability scanner, --malware-scan=true to include malware scanning, and --image-scan=true to install the container image scanner.",
	Example: `  # Install only the VM vulnerability scanner with malware scanning
  sudo knoxctl onboard vm scanner --rra-scan=false --vm-scanner=true --malware-scan=true --schedule="0 */6 * * *" \
    --url=https://cspm.accuknox.com/api/v1/artifact/ --tenant-id=123 --label=LABEL --token-file=/path/to/token

  # Add VM and container image scanning to the default RRA installation
  sudo knoxctl onboard vm scanner --vm-scanner=true --image-scan=true \
    --url=https://cspm.accuknox.com/api/v1/artifact/ --tenant-id=123 --label=LABEL \
    --token-file=/path/to/token --cluster-name=CLUSTER --profile=ubuntu --benchmark=stig --schedule="0 0 * * *"`,
	Args:              cobra.NoArgs,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateScannerSelection(cmd); err != nil {
			return err
		}

		if dryRun {
			if onboardRRAScan {
				fmt.Fprintln(cmd.OutOrStdout(), "Would install the RRA scanner")
			}
			return installOnboardScanners(cmd, &onboard.ClusterConfig{DryRun: true})
		}
		// A standalone VM scanner does not need RRA's Docker/SPIRE setup.
		if !onboardRRAScan && !imageScan {
			return installOnboardScanners(cmd, &onboard.ClusterConfig{})
		}

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

		agentsDeployed := isDeployed(vmMode)

		configPath, err := common.GetDefaultConfigPath()
		if err != nil {
			logger.Error("failed to get default config path:%s", err.Error())
			return err
		}

		cc.DefaultConfigPath = configPath
		if cidr != "" {
			_, network, err := net.ParseCIDR(cidr)
			if err != nil {
				return err
			}

			cc.CIDR = network.String()
		}

		if !agentsDeployed {
			cc.AdditionalArgs = make(map[string]any)
			cc.AdditionalArgs["JoinToken"] = joinToken
			cc.AdditionalArgs["SpireSecretDir"] = spireDir
			cc.AdditionalArgs["ConfigPath"] = configPath
			cc.AdditionalArgs["NetworkCIDR"] = cc.CIDR
		}

		cc.EnableVMScan = onboardRRAScan
		// create RRA config
		cc.Mode = vmMode
		cc.LogRotateMaxFile = logRotateMaxFile
		cc.LogRotateMaxSize = logRotateMaxSize

		cc.LogRotateMaxSize = strings.ToLower(cc.LogRotateMaxSize)
		if vmMode == onboard.VMMode_Systemd {
			cc.LogRotateMaxSize = strings.ToUpper(cc.LogRotateMaxSize)
		}

		artifactToken, tokenErr := resolveArtifactToken(authToken, authTokenFile)
		if tokenErr != nil {
			return tokenErr
		}
		if onboardRRAScan {
			err = cc.InitRRAConfig(artifactToken, url, tenantID, clusterID, clusterName, label, schedule, profile, benchmark, registry, registryConfigPath, insecure, plainHTTP, imageVersions.RRAImage, imageVersions.RRAImageTag, releaseVersion, preserveUpstream, agentsDeployed, imageVersions.SpireImage, spireHost, spireDir, knoxGateway)
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
		}

		cc.AgentsResource = agentsResource
		return installOnboardScanners(cmd, &cc)
	},
}

func validateScannerSelection(cmd *cobra.Command) error {
	if !onboardRRAScan && !enableOmniVMScan && !imageScan {
		return fmt.Errorf("enable at least one scanner: --rra-scan, --vm-scanner, or --image-scan")
	}
	if onboardVMScannerOptions.MalwareScan && !enableOmniVMScan {
		return fmt.Errorf("--malware-scan requires --vm-scanner=true")
	}
	if onboardRRAScan {
		if err := validateLegacyRRAFlags(cmd); err != nil {
			return err
		}
	}
	if (onboardRRAScan || imageScan) && clusterName == "" {
		return fmt.Errorf("cluster name is required for RRA or image scanning")
	}
	return validateOnboardScanners(cmd)
}

func init() {
	onboardVmScanCmd.PersistentFlags().StringVarP(&releaseVersion, "version", "v", "", "agents release version to use")
	onboardVmScanCmd.Flags().BoolVar(&onboardRRAScan, "rra-scan", true, "install the RRA risk assessment scanner")
	onboardVmScanCmd.Flags().BoolVar(&enableOmniVMScan, "vm-scanner", false, "install the daily Omni VM vulnerability scanner (alias: --enable-vm-scan)")
	onboardVmScanCmd.Flags().BoolVar(&onboardVMScannerOptions.MalwareScan, "malware-scan", false, "include malware scanning with --vm-scanner; requires ClamAV and signatures")
	onboardVmScanCmd.Flags().StringVar(&onboardVMScannerOptions.MalwareDBDirectory, "malware-db-directory", "/var/lib/clamav", "ClamAV signature database directory")

	// all flags are optional
	onboardVMCmd.AddCommand(onboardVmScanCmd)
}

func isDeployed(vmMode onboard.VMMode) bool {
	switch vmMode {
	case onboard.VMMode_Docker:
		containers, _, err := deboard.GetInstalledObjects()
		if err != nil {
			return false
		}

		if _, ok := containers["spire-agent"]; ok {
			return true
		}
	case onboard.VMMode_Systemd:
		services, err := onboard.CheckInstalledSystemdServices()
		if err != nil {
			return false
		}

		if slices.ContainsFunc(services, func(s string) bool {
			return s == "spire-agent"
		}) {
			return true
		}

	}
	return false
}
