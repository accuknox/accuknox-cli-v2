package cmd

import (
	"context"
	"fmt"
	"io"
	neturl "net/url"
	"os"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/accuknox/accuknox-cli-v2/pkg/vmscan"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func newOmniVMScanCmd() *cobra.Command {
	return newOmniVMScanCmdWithScheduler(vmscan.InstallScheduled)
}

func newOmniVMScanCmdWithScheduler(schedule func(context.Context, vmscan.Config, io.Writer, io.Writer) error) *cobra.Command {
	return newOmniVMScanCmdWithLifecycle(schedule, onboard.UninstallVMScanner)
}

func newOmniVMScanCmdWithLifecycle(schedule func(context.Context, vmscan.Config, io.Writer, io.Writer) error, uninstall func(context.Context) error) *cobra.Command {
	var config vmscan.Config
	var useScript bool
	var runOnce bool
	var removeScanner bool
	var tokenFile string
	var scanClusterID int
	c := &cobra.Command{
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		Use:               "vm-scan",
		Short:             "Install Omni's scheduled VM vulnerability scanner",
		Long:              "Use Knoxctl's templates to install omni.service and a daily omni.timer, and trigger an initial scan through Omni's Go packages. Onboarding uses the same native installation. Use --use-script to run the external installer instead, --run-once for a single package scan, or --uninstall to remove the VM scanner.",
		Args:              cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if removeScanner {
				if err := uninstall(cmd.Context()); os.IsNotExist(err) {
					fmt.Fprintln(cmd.OutOrStdout(), "VM scanner (Omni) is not installed; skipping.")
					return nil
				} else if err != nil {
					return fmt.Errorf("removing VM scanner (Omni): %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "VM scanner (Omni) removed successfully.")
				return nil
			}
			endpoint, err := normalizeArtifactEndpoint(config.Endpoint)
			if err != nil {
				return err
			}
			config.Endpoint = endpoint
			if cmd.Flags().Changed("cluster-id") {
				config.ClusterID = &scanClusterID
			}
			config.Token, err = resolveArtifactToken(config.Token, tokenFile)
			if err != nil {
				return err
			}
			if config.UploadTimeout <= 0 {
				return fmt.Errorf("--upload-timeout must be positive")
			}
			if useScript {
				for _, name := range []string{"rootfs", "hostname", "output-dir", "upload-timeout"} {
					if cmd.Flags().Changed(name) {
						return fmt.Errorf("--%s cannot be combined with --use-script", name)
					}
				}
				return installVMScannerWithStatus(cmd, config, vmscan.Install)
			}
			for _, name := range []string{"script-url", "script-path", "omni-binary-url", "omni-install-dir"} {
				if cmd.Flags().Changed(name) {
					return fmt.Errorf("--%s requires --use-script", name)
				}
			}
			if runOnce {
				return vmscan.ScanWithProgress(cmd.Context(), config, cmd.OutOrStdout(), cmd.ErrOrStderr())
			}
			return installVMScannerWithStatus(cmd, config, schedule)
		},
	}
	c.SetGlobalNormalizationFunc(artifactEndpointFlagAlias)
	c.Flags().StringVar(&config.Endpoint, "url", "", "artifact API URL or domain (alias: --artifact-endpoint)")
	c.Flags().StringVarP(&config.Token, "auth-token", "t", "", "artifact API token (alias: --token; defaults to OMNI_ARTIFACT_API_TOKEN)")
	c.Flags().StringVar(&tokenFile, "token-file", "", "read artifact API token from a file (aliases: --auth-token-file, --artifact-token-file)")
	c.MarkFlagsMutuallyExclusive("auth-token", "token-file")
	c.Flags().UintVar(&config.TenantID, "tenant-id", 0, "tenant ID")
	c.Flags().IntVar(&scanClusterID, "cluster-id", 0, "cluster ID to include in findings (native scanning only)")
	c.Flags().StringVar(&config.ClusterName, "cluster-name", "", "cluster name to include in findings (native scanning only)")
	c.Flags().DurationVar(&config.UploadTimeout, "upload-timeout", vmscan.DefaultUploadTimeout, "maximum time for artifact upload including retries (native scanning only)")
	c.Flags().StringVarP(&config.Label, "artifact-label", "l", "", "artifact label ID")
	c.Flags().StringVar(&config.Label, "label", "", "alias for --artifact-label")
	c.Flags().BoolVar(&useScript, "use-script", false, "use the external Bash installer instead of Knoxctl's native service and timer templates")
	c.Flags().BoolVar(&runOnce, "run-once", false, "run a single scan through the Omni package without installing a service or timer")
	c.Flags().BoolVar(&removeScanner, "uninstall", false, "remove the Omni VM scanner, service, timer, credentials, and standard installation data")
	c.Flags().StringVar(&config.RootFS, "rootfs", "", "root filesystem to scan (defaults to this host)")
	c.Flags().StringVar(&config.DBDirectory, "vuln-db-directory", "/var/lib/accuknox-omni", "Omni vulnerability database directory")
	c.Flags().StringVar(&config.Hostname, "hostname", "", "override the host name in findings")
	c.Flags().BoolVar(&config.SkipDBUpdate, "skip-db-update", false, "use an existing vulnerability database without updating")
	c.Flags().BoolVar(&config.Offline, "offline", false, "disable scanner downloads; requires pre-provisioned Linux vulnerability databases")
	c.Flags().StringVar(&config.OutputDir, "output-dir", "", "write findings locally instead of uploading; artifact credentials are not required")
	c.Flags().StringVar(&config.ScriptURL, "script-url", vmscan.InstallerURL, "external installer URL (requires --use-script)")
	c.Flags().StringVar(&config.ScriptPath, "script-path", "", "local external installer file (requires --use-script)")
	c.Flags().StringVar(&config.BinaryURL, "omni-binary-url", "", "Omni binary download URL passed to the installer (for internal mirrors)")
	c.Flags().StringVar(&config.InstallDir, "omni-install-dir", "/usr/local/bin", "Omni installation directory; offline installation requires an executable omni here")
	c.Flags().BoolVar(&config.MalwareScan, "malware-scan", false, "enable malware scanning for direct scans and native or script installations; skips malware scanning with guidance if ClamAV tools or signatures are missing")
	c.Flags().StringVar(&config.MalwareDBDirectory, "malware-db-directory", "/var/lib/clamav", "ClamAV signature database directory")
	c.MarkFlagsMutuallyExclusive("script-url", "script-path")
	c.MarkFlagsMutuallyExclusive("use-script", "run-once", "uninstall")
	return c
}

func init() {
	rootCmd.AddCommand(newOmniVMScanCmd())
}

func resolveArtifactToken(token, tokenFile string) (string, error) {
	if token != "" && tokenFile != "" {
		return "", fmt.Errorf("use either --auth-token/--token or --token-file, not both")
	}
	if tokenFile != "" {
		data, err := os.ReadFile(tokenFile)
		if err != nil {
			return "", fmt.Errorf("reading artifact token file: %w", err)
		}
		token = strings.TrimSpace(string(data))
		if token == "" {
			return "", fmt.Errorf("artifact token file is empty")
		}
		return token, nil
	}
	if token != "" {
		return token, nil
	}
	return os.Getenv("OMNI_ARTIFACT_API_TOKEN"), nil
}

func artifactEndpointFlagAlias(_ *pflag.FlagSet, name string) pflag.NormalizedName {
	switch name {
	case "artifact-endpoint":
		name = "url"
	case "token":
		name = "auth-token"
	case "auth-token-file", "artifact-token-file":
		name = "token-file"
	}
	return pflag.NormalizedName(name)
}

func normalizeArtifactEndpoint(endpoint string) (string, error) {
	if endpoint == "" {
		return "", nil
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	u, err := neturl.ParseRequestURI(endpoint)
	if err != nil {
		return "", err
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = "/api/v1/artifact/"
	}
	return u.String(), nil
}

func installVMScannerWithStatus(cmd *cobra.Command, config vmscan.Config, install func(context.Context, vmscan.Config, io.Writer, io.Writer) error) error {
	fmt.Fprintln(cmd.OutOrStdout(), "Installing VM scanner...")
	if err := install(cmd.Context(), config, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "VM scanner installation failed.")
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "VM scanner installed successfully.")
	return nil
}
