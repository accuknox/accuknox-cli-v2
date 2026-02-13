package cmd

import (
	"fmt"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/accuknox/accuknox-cli-v2/pkg/vm"
	"github.com/spf13/cobra"
)

var (
	downloadOpts vm.DownloadOptions
	archs        string
	debug        bool
)

// vmCmd represents the vm command
var vmDownload = &cobra.Command{
	Use:   "pre-download",
	Short: "Commands for pre-download of agent binaries/images",
	Long:  "Commands for pre-download of agent binaries/images",
	RunE: func(cmd *cobra.Command, args []string) error {

		switch vmMode {
		case "":
			downloadOpts.VMMode = []onboard.VMMode{onboard.VMMode_Docker, onboard.VMMode_Systemd}
		case onboard.VMMode_Docker:
			downloadOpts.VMMode = []onboard.VMMode{onboard.VMMode_Docker}
		case onboard.VMMode_Systemd:
			downloadOpts.VMMode = []onboard.VMMode{onboard.VMMode_Systemd}
		default:
			logger.Error("vm mode: %s invalid, accepted values (docker/systemd)", vmMode)
			return fmt.Errorf("vm mode: %s invalid, accepted values (docker/systemd)", vmMode)
		}

		switch strings.ToLower(archs) {
		case "", "amd64,arm64":
			downloadOpts.Arch = []string{"amd64", "arm64"}
		case "amd64", "amd":
			downloadOpts.Arch = []string{"amd64"}
		case "arm64", "arm":
			downloadOpts.Arch = []string{"arm64"}

		default:
			logger.Error("arch: %s invalid, accepted values (amd64/arm64)", archs)
			return fmt.Errorf("arch: %s invalid, accepted values (amd64/arm64)", archs)
		}

		downloadOpts.Version = releaseVersion
		downloadOpts.Registry = registry
		downloadOpts.RegistryConfigPath = registryConfigPath
		downloadOpts.InsecureRegistryConnection = insecure
		downloadOpts.HttpRegistryConnection = plainHTTP
		downloadOpts.PreserveUpstream = preserveUpstream

		downloadOpts.Debug = debug

		return downloadOpts.Download()
	},
}

// ========== //
// == Init == //
// ========== //

func init() {
	vmCmd.AddCommand(vmDownload)

	// options for vm download commands related to saving binaries/images
	vmDownload.PersistentFlags().StringVarP(&releaseVersion, "version", "v", "", "agents release version to use")
	vmDownload.PersistentFlags().StringVar((*string)(&vmMode), "vm-mode", "", "Mode of installation (systemd/docker)")

	vmDownload.PersistentFlags().StringVar(&archs, "arch", "amd64,arm64", "comma separated list of architectures to download, Default: amd64,arm64")
	vmDownload.PersistentFlags().StringVar(&downloadOpts.SavePath, "save-path", "", "path to save downloaded binaries/images, Default: current directory")

	vmDownload.PersistentFlags().StringVarP(&registry, "registry", "r", "docker.io", "the registry to authenticate with (default - DockerHub)")
	vmDownload.PersistentFlags().StringVarP(&registryConfigPath, "registry-config-path", "", "", "path to pre-existing OCI registry config")

	vmDownload.PersistentFlags().BoolVarP(&plainHTTP, "plain-http", "", false, "use plain HTTP everywhere")
	vmDownload.PersistentFlags().BoolVarP(&insecure, "insecure", "", false, "skip verifying TLS certs")
	vmDownload.PersistentFlags().Lookup("plain-http").NoOptDefVal = "true"
	vmDownload.PersistentFlags().Lookup("insecure").NoOptDefVal = "true"

	vmDownload.PersistentFlags().BoolVarP(&preserveUpstream, "preserve-upstream-repo", "", true, "to keep upstream repo name e.g \"accuknox\" from accuknox/shared-informer-agent")

	vmDownload.PersistentFlags().BoolVar(&debug, "debug", false, "debug mode")
}
