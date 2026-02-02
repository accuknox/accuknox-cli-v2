package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/imagescan"
	"github.com/accuknox/kmux"
	"github.com/accuknox/kmux/config"
	"github.com/accuknox/kmux/security"
	kubesheildScanner "github.com/accuknox/kubeshield/pkg/scanner/scan"
	"github.com/spf13/cobra"
)

var (
	HOST_NAME                   string
	RUN_TIME                    string
	artifactEndpointPath        string
	vulnerabilityDB             string
	javaDB                      string
	allContainers               bool
	imagesOnly                  bool
	cfg                         = kubesheildScanner.ScanConfig{}
	defaultArtifactEndpointPath = "/api/v1/artifact/"

	// systemd config
	kmuxConfigPath        string
	defaultKmuxconfigPath = "/opt/kubeshield-service/kmux-config.yaml"
	spireSockPath         = "unix:///var/run/spire/agent.sock"
)

var imageScanCmd = &cobra.Command{
	Use:   "image-scan",
	Short: "scans vm container images",
	Long: `Scans VM container images 
and sends back the result to saas
		`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {

		if strings.HasPrefix(cfg.ArtifactConfig.ArtifactAPI, "http://") {
			return fmt.Errorf("http scheme not supported: %s", cfg.ArtifactConfig.ArtifactAPI)
		}

		// Adds Scheme if not present
		if !strings.HasPrefix(cfg.ArtifactConfig.ArtifactAPI, "https://") {
			cfg.ArtifactConfig.ArtifactAPI = "https://" + cfg.ArtifactConfig.ArtifactAPI
		}

		// Checks whether the domain is in vaild regex pattern
		if !imagescan.IsValidDomain(cfg.ArtifactConfig.ArtifactAPI) {
			return fmt.Errorf("invalid domain name: %s", cfg.ArtifactConfig.ArtifactAPI)
		}

		// if artifact endpoint(after domain) is empty then use default value
		if artifactEndpointPath == "" {
			artifactEndpointPath = defaultArtifactEndpointPath
		}

		if !strings.HasPrefix(artifactEndpointPath, "/") {
			artifactEndpointPath = "/" + artifactEndpointPath
		}

		// trivy can make use of this variable to download the trivyDB from the
		// specified source. If it is empty, trivy will download from one of its public
		// registries.
		_ = os.Setenv("TRIVY_DB_REPOSITORY", vulnerabilityDB)
		_ = os.Setenv("TRIVY_JAVA_DB_REPOSITORY", javaDB)

		// Init kmux for both cp and worker node
		if imagescan.NodeType != "" {
			if err := kmux.Init(&config.Options{
				LocalConfigFile: kmuxConfigPath,
			}); err != nil {
				return err
			}
		}

		// Only for control plane, we are connecting to spire for pushing
		// the messages to knoxgateway
		if imagescan.NodeType == "cp" {
			if err := initSpire(spireSockPath); err != nil {
				return err
			}

		}

		cfg.ArtifactConfig.ArtifactAPI += artifactEndpointPath
		return imagescan.DiscoverAndScan(cfg, HOST_NAME, RUN_TIME, !allContainers, imagesOnly)

	},
}

func init() {

	// Artifact API Configurations
	imageScanCmd.Flags().StringVarP(&cfg.ArtifactConfig.ArtifactAPI, "artifactEndpoint", "", "",
		"Specify the domain name of the artifact endpoint")
	imageScanCmd.Flags().StringVarP(&artifactEndpointPath, "artifactEndpointPath", "", "",
		"Optional: specify the URL path segment after the domain name")
	imageScanCmd.Flags().StringVarP(&cfg.ArtifactConfig.Label, "label", "l", "", "used to filter the finding based on the label")
	imageScanCmd.Flags().StringVarP(&cfg.ArtifactConfig.ArtifactToken, "token", "t", "", "token required for authentication")

	// Scan Configurations
	imageScanCmd.Flags().StringVarP(&HOST_NAME, "hostname", "", "", "name of the host")
	imageScanCmd.Flags().StringVarP(&RUN_TIME, "runtime", "r", "", "container runtime used in the host machine")
	imageScanCmd.Flags().BoolVar(&allContainers, "all-containers", false, "If set, discover containers in all states. By default, only running containers are discovered.")
	imageScanCmd.Flags().BoolVar(&imagesOnly, "images-only", false, "If set, discovers and scans all images. By default, only images from running containers are scanned.")

	// Trivy Configurations
	imageScanCmd.Flags().StringVarP(&vulnerabilityDB, "db-repository", "", "", "OCI repository to retrieve vulnerability db")
	imageScanCmd.Flags().StringVarP(&javaDB, "java-db-repository", "", "", "OCI repository to retrieve java db")

	// ImageScanning configurations for systemd mode
	// TODO: Add validation for either knoxgateway or rmq
	imageScanCmd.Flags().StringVar(&imagescan.FlushTo, "flush-to", "", "flushes the data to the specified service")
	imageScanCmd.Flags().StringVar(&kmuxConfigPath, "kmux-config", defaultKmuxconfigPath, "kmux config path")
	imageScanCmd.Flags().StringVar(&imagescan.NodeType, "node-type", "", "specify the type of node (CP/Worker)")
	imageScanCmd.Flags().StringVar(&spireSockPath, "spire-sock", spireSockPath, "spire socket path")
	imageScanCmd.Flags().StringVar(&imagescan.CreateRegistryURL, "create-registry-url", "", "create registry url")

	// Required Flags Validation
	imageScanCmd.MarkFlagsOneRequired("artifactEndpoint", "token", "label")
	imageScanCmd.MarkFlagsRequiredTogether("artifactEndpoint", "token", "label")

	// For intenral purpose hide the flags
	_ = imageScanCmd.Flags().MarkHidden("flush-to")
	_ = imageScanCmd.Flags().MarkHidden("kmux-config")
	_ = imageScanCmd.Flags().MarkHidden("node-type")
	_ = imageScanCmd.Flags().MarkHidden("create-registry-url")

	rootCmd.AddCommand(imageScanCmd)
}

func initSpire(sockPath string) error {
	spireSecurity, err := security.NewSecurity("")
	if err != nil {
		return err
	}

	return spireSecurity.Connect(sockPath)
}
