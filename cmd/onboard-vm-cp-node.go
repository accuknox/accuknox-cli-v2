package cmd

import (
	"fmt"
	"os"

	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/spf13/cobra"
)

var (
	// essential flags
	joinToken   string
	spireHost   string
	ppsHost     string
	knoxGateway string

	// non-essential
	spireTrustBundle string
	enableLogs       bool

	// cp-node only images
	kubeArmorRelayServerImage string
	siaImage                  string
	peaImage                  string
	feederImage               string
	spireAgentImage           string

	// cp-node systemd tags
	kubeArmorRelayServerTag string
	siaVersionTag           string
	peaVersionTag           string
	feederVersionTag        string
)

// cpNodeCmd represents the init command
var cpNodeCmd = &cobra.Command{
	Use:   "cp-node",
	Short: "Initialize a control plane node for onboarding onto SaaS",
	Long:  "Initialize a control plane node for onboarding onto SaaS",
	RunE: func(cmd *cobra.Command, args []string) error {
		// validate environment for pre-requisites
		var (
			cc               onboard.ClusterConfig
			secureContainers = true
		)
		_, err := cc.ValidateEnv()
		if vmMode == "" {
			if err == nil {
				vmMode = onboard.VMMode_Docker
			} else {
				fmt.Printf("Warning: Docker requirements did not match: %s. Falling back to systemd mode for installation.\n", err.Error())
				vmMode = onboard.VMMode_Systemd
				secureContainers = false
			}
		} else if vmMode == onboard.VMMode_Systemd {
			// systemd mode explicitly specified
			if err != nil {
				// docker requirements didn't meet - containers won't be protected
				secureContainers = false
			}
		} else if vmMode == onboard.VMMode_Docker && err != nil {
			// docker mode specified explicitly but requirements didn't match
			return fmt.Errorf("failed to validate environment: %s", err.Error())
		}
		vmConfig, err := onboard.CreateClusterConfig(onboard.ClusterType_VM, userConfigPath, vmMode,
			vmAdapterTag, kubeArmorRelayServerTag, peaVersionTag, siaVersionTag,
			feederVersionTag, kubearmorVersion, releaseVersion, kubeArmorImage,
			kubeArmorInitImage, kubeArmorVMAdapterImage, kubeArmorRelayServerImage, siaImage,
			peaImage, feederImage, spireAgentImage, nodeAddr, dryRun,
			false, imagePullPolicy, visibility, hostVisibility,
			audit, block, cidr, secureContainers)
		if err != nil {
			return fmt.Errorf("failed to create cluster config: %s", err.Error())
		}
		onboardConfig := onboard.InitCPNodeConfig(*vmConfig, joinToken, spireHost, ppsHost, knoxGateway, spireTrustBundle, enableLogs)

		err = onboardConfig.CreateBaseTemplateConfig()
		if err != nil {
			return fmt.Errorf("failed to create base template config: %s", err.Error())
		}

		switch vmMode {

		case onboard.VMMode_Systemd:
			err = onboardConfig.InitializeControlPlaneSD()
			if err != nil {
				return fmt.Errorf("failed to onboard control plane node: %s", err.Error())
			}
			fmt.Println(cm.Green +
				`VM successfully onboarded!

Now run the below command to onboard any worker nodes.
Please assign appropriate IP address to --cp-node-addr to make sure
that worker nodes can connect to this node` + cm.Reset)
			onboardConfig.PrintJoinCommand(vmMode)

		case onboard.VMMode_Docker:
			err = onboardConfig.InitializeControlPlane()
			if err != nil {
				return fmt.Errorf("failed to onboard control plane node: %s", err.Error())
			}
			fmt.Println(cm.Green +
				`VM successfully onboarded!

Now run the below command to onboard any worker nodes.
Please assign appropriate IP address to --cp-node-addr to make sure
that worker nodes can connect to this node` + cm.Reset)
			onboardConfig.PrintJoinCommand(vmMode)

		default:
			return fmt.Errorf("vm mode: %s invalid, accepted values (docker/systemd)", vmMode)
		}
		return nil
	},
}

func init() {
	// configuration for connecting with accuKnox SaaS
	cpNodeCmd.PersistentFlags().StringVarP(&releaseVersion, "version", "v", "", "agents release version to use")

	cpNodeCmd.PersistentFlags().StringVar(&joinToken, "join-token", "", "join-token to use")
	cpNodeCmd.PersistentFlags().StringVar(&spireHost, "spire-host", "", "address of spire-host to connect for authenticating with accuknox SaaS")
	cpNodeCmd.PersistentFlags().StringVar(&ppsHost, "pps-host", "", "address of policy-provider-service to connect with for receiving policies")
	cpNodeCmd.PersistentFlags().StringVar(&knoxGateway, "knox-gateway", "", "address of knox-gateway to connect with for pushing telemetry data")

	cpNodeCmd.PersistentFlags().StringVar(&spireTrustBundle, "spire-trust-bundle-addr", "", "address of spire trust bundle (CA cert for accuknox spire-server)")

	cpNodeCmd.PersistentFlags().BoolVar(&enableLogs, "enable-logs", false, "enable pushing logs from feeder service")

	cpNodeCmd.PersistentFlags().StringVar(&nodeAddr, "cp-node-addr", "", "address of control plane node for generating join command")

	cpNodeCmd.PersistentFlags().StringVar(&kubeArmorRelayServerImage, "kubearmor-relay-server", "", "KubeArmor relay-server image to use")
	cpNodeCmd.PersistentFlags().StringVar(&siaImage, "sia-image", "", "sia image to use")
	cpNodeCmd.PersistentFlags().StringVar(&peaImage, "pea-image", "", "pea image to use")
	cpNodeCmd.PersistentFlags().StringVar(&feederImage, "feeder-image", "", "feeder-service image to use")
	cpNodeCmd.PersistentFlags().StringVar(&kubeArmorRelayServerTag, "relayserver-version", "", "relay server version to use")
	cpNodeCmd.PersistentFlags().StringVar(&siaVersionTag, "sia-version", "", "sia version to use")
	cpNodeCmd.PersistentFlags().StringVar(&peaVersionTag, "pea-version", "", "pea version to use")
	cpNodeCmd.PersistentFlags().StringVar(&feederVersionTag, "feeder-version", "", "feeder version to use")
	cpNodeCmd.PersistentFlags().StringVar(&spireAgentImage, "spire-agent-image", "", "spire-agent image to use")

	err := cpNodeCmd.MarkPersistentFlagRequired("join-token")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = cpNodeCmd.MarkPersistentFlagRequired("spire-host")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = cpNodeCmd.MarkPersistentFlagRequired("pps-host")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = cpNodeCmd.MarkPersistentFlagRequired("knox-gateway")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = cpNodeCmd.MarkPersistentFlagRequired("version")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	onboardVMCmd.AddCommand(cpNodeCmd)
}
