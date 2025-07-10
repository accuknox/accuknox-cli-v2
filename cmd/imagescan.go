package cmd

import (
	"github.com/accuknox/accuknox-cli-v2/pkg/imagescan"
	kubesheildConfig "github.com/accuknox/kubeshield/pkg/scanner/config"
	"github.com/spf13/cobra"
)

var (
	HOST_NAME string
	RUN_TIME  string
	cfg       = kubesheildConfig.Config{}
)

var imageScanCmd = &cobra.Command{
	Use:   "image-scan",
	Short: "scans vm container images",
	Long: `Scans VM container images using trivy 
and sends back the result to saas through artifact API
		`,
	Args: cobra.NoArgs,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return imagescan.IsTrivyInstalled()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return imagescan.DiscoverAndScan(cfg, HOST_NAME, RUN_TIME)
	},
}

func init() {

	// Artifact API Configurations
	imageScanCmd.Flags().StringVarP(&cfg.ScanConfig.ArtifactAPI, "artifactEndpoint", "", "",
		"scanned results will sent back to saas through this api")
	imageScanCmd.Flags().StringVarP(&cfg.ScanConfig.Label, "label", "l", "", "used to filter the finding based on the label")
	imageScanCmd.Flags().StringVarP(&cfg.ScanConfig.ArtifactToken, "token", "t", "", "token required for authentication")
	imageScanCmd.Flags().StringVarP(&cfg.ScanConfig.TenantID, "tenantId", "", "", "tenant id")

	// Scan Configurations
	imageScanCmd.Flags().StringVarP(&cfg.ScanConfig.ScanTool, "tool", "", "trivy", "tool used for scanning")
	imageScanCmd.Flags().StringVarP(&HOST_NAME, "hostname", "", "", "name of the host")
	imageScanCmd.Flags().StringVarP(&RUN_TIME, "runtime", "r", "", "container runtime used in the host machine")

	// Required Flags Validation
	imageScanCmd.MarkFlagsOneRequired("artifactEndpoint", "token", "label", "tenantId")
	imageScanCmd.MarkFlagsRequiredTogether("artifactEndpoint", "token", "label", "tenantId")
	rootCmd.AddCommand(imageScanCmd)
}
