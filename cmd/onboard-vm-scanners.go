package cmd

import (
	"fmt"
	"maps"
	neturl "net/url"
	"strconv"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/accuknox/accuknox-cli-v2/pkg/vmscan"
	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
)

var (
	enableOmniVMScan        bool
	onboardVMScannerOptions vmscan.Config
	onboardVMScanUseScript  bool
)

func onboardScannerConfig() (vmscan.Config, error) {
	endpoint, err := normalizeArtifactEndpoint(url)
	if err != nil {
		return vmscan.Config{}, err
	}
	token, err := resolveArtifactToken(authToken, authTokenFile)
	if err != nil {
		return vmscan.Config{}, err
	}
	id, err := strconv.ParseUint(tenantID, 10, strconv.IntSize)
	if enableOmniVMScan && err != nil {
		return vmscan.Config{}, fmt.Errorf("VM scanning requires a positive numeric --tenant-id")
	}
	c := onboardVMScannerOptions
	c.Endpoint, c.Token, c.TenantID, c.Label = endpoint, token, uint(id), label
	if enableOmniVMScan && !onboardVMScanUseScript {
		c.ClusterName = clusterName
		if clusterID != "" {
			id, err := strconv.Atoi(clusterID)
			if err != nil {
				return c, fmt.Errorf("native VM scanning requires a numeric --cluster-id: %w", err)
			}
			c.ClusterID = &id
		}
	}
	c.Offline = c.Offline || skipDownload
	if enableOmniVMScan {
		var validation error
		if onboardVMScanUseScript {
			validation = c.ValidateInstall()
		} else {
			validation = c.ValidateScheduled()
		}
		if validation != nil {
			return c, validation
		}
	}
	if imageScan && (endpoint == "" || token == "" || label == "") {
		return c, fmt.Errorf("image scanning requires --artifact-endpoint (or --url), --auth-token/--token or --token-file (or OMNI_ARTIFACT_API_TOKEN), and --label")
	}
	if imageScan {
		u, err := neturl.ParseRequestURI(endpoint)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "/api/v1/artifact/" || u.RawQuery != "" || u.Fragment != "" {
			return c, fmt.Errorf("image scanner requires an HTTPS /api/v1/artifact/ endpoint")
		}
	}
	return c, nil
}

func validateLegacyRRAFlags(cmd *cobra.Command) error {
	token, err := resolveArtifactToken(authToken, authTokenFile)
	if err != nil {
		return err
	}
	for _, group := range [][]string{{"benchmark", "profile", "url", "tenant-id"}, {"auth-token", "label", "schedule"}} {
		var changed, missing []string
		for _, name := range group {
			if flag := cmd.Flag(name); flag != nil && (flag.Changed || (name == "auth-token" && token != "")) {
				changed = append(changed, name)
			} else {
				missing = append(missing, name)
			}
		}
		if len(changed) > 0 && len(missing) > 0 {
			return fmt.Errorf("RRA flags %s must be set together; missing %s", strings.Join(group, ", "), strings.Join(missing, ", "))
		}
	}
	return nil
}

func validateOnboardScanners(cmd *cobra.Command) error {
	if onboardVMScanUseScript && cmd != nil {
		if flag := cmd.Flag("vm-scan-upload-timeout"); flag != nil && flag.Changed {
			return fmt.Errorf("--vm-scan-upload-timeout requires native scanning; omit --use-script")
		}
	}
	if _, err := resolveArtifactToken(authToken, authTokenFile); err != nil {
		return err
	}
	if onboardVMScanUseScript && !enableOmniVMScan {
		return fmt.Errorf("--use-script requires --enable-vm-scan during VM onboarding")
	}
	if !enableOmniVMScan && !imageScan {
		return nil
	}
	if enableOmniVMScan && !onboardVMScanUseScript && cmd != nil {
		for _, name := range []string{"vm-scan-script-url", "vm-scan-script-path", "vm-scan-binary-url", "vm-scan-install-dir"} {
			if flag := cmd.Flag(name); flag != nil && flag.Changed {
				return fmt.Errorf("--%s requires --use-script", name)
			}
		}
	}
	if _, err := onboardScannerConfig(); err != nil {
		return err
	}
	if imageScan && schedule != "" {
		if _, err := cron.ParseStandard(schedule); err != nil {
			return fmt.Errorf("invalid image scan schedule: %w", err)
		}
	}
	return nil
}

func installOnboardScanners(cmd *cobra.Command, cc *onboard.ClusterConfig) error {
	if !enableOmniVMScan && !imageScan {
		return nil
	}
	c, err := onboardScannerConfig()
	if err != nil {
		return err
	}
	if cc.DryRun {
		if enableOmniVMScan {
			if onboardVMScanUseScript {
				fmt.Fprintln(cmd.OutOrStdout(), "Would install Omni's daily VM scanner using the external installer")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Would install Omni's daily VM scanner using Knoxctl's native service and timer templates")
			}
		}
		if imageScan {
			fmt.Fprintln(cmd.OutOrStdout(), "Would install the container image scanner")
		}
		return nil
	}
	if enableOmniVMScan {
		install := vmscan.InstallScheduled
		if onboardVMScanUseScript {
			install = vmscan.Install
		}
		if err := installVMScannerWithStatus(cmd, c, install); err != nil {
			return fmt.Errorf("installing Omni VM scanner: %w", err)
		}
	}
	if imageScan {
		u, err := neturl.ParseRequestURI(c.Endpoint)
		if err != nil {
			return err
		}
		if u.Path != "/api/v1/artifact/" || u.RawQuery != "" {
			return fmt.Errorf("image scanner requires the /api/v1/artifact/ endpoint")
		}
		scanSchedule := schedule
		if scanSchedule == "" {
			scanSchedule = "0 0 * * *"
		}

		scannerConfig := *cc
		scannerConfig.AdditionalArgs = maps.Clone(cc.AdditionalArgs)
		scannerConfig.SystemdServiceObjects = nil
		if err := scannerConfig.InitImageScan(c.Token, u.Scheme+"://"+u.Host, tenantID, clusterID, clusterName, c.Label, scanSchedule, allImages); err != nil {
			return fmt.Errorf("configuring image scanner: %w", err)
		}
		if err := scannerConfig.InstallImagescan(); err != nil {
			return fmt.Errorf("installing image scanner: %w", err)
		}
	}
	return nil
}
