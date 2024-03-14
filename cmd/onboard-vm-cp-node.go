package cmd

import (
	"fmt"
	"os"

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

	// cp-node only images
	kubeArmorRelayServerImage string
	siaImage                  string
	peaImage                  string
	feederImage               string
)

// cpNodeCmd represents the init command
var cpNodeCmd = &cobra.Command{
	Use:   "cp-node",
	Short: "Initialize a control plane node for onboarding onto SaaS",
	Long:  "Initialize a control plane node for onboarding onto SaaS",
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterConfig, err := onboard.CreateClusterConfig(onboard.ClusterType_VM, userConfigPath, kubearmorVersion, releaseVersion, kubeArmorImage, kubeArmorInitImage, kubeArmorVMAdapterImage, kubeArmorRelayServerImage, siaImage, peaImage, feederImage, nodeAddr, dryRun, false, imagePullPolicy, visibility, hostVisibility, audit, block, cidr)
		if err != nil {
			return fmt.Errorf("Failed to create cluster config: %s", err.Error())
		}

		onboardConfig := onboard.InitCPNodeConfig(*clusterConfig, joinToken, spireHost, ppsHost, knoxGateway, spireTrustBundle)

		err = onboardConfig.InitializeControlPlane()
		if err != nil {
			return fmt.Errorf("Failed to onboard control plane node: %s", err.Error())
		}

		fmt.Println(
			`VM successfully onboarded!

Now run the below command to onboard any worker nodes.
Please assign appropriate IP address to --cp-node-addr to make sure
that worker nodes can connect to this node`)
		onboardConfig.PrintJoinCommand()

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

	cpNodeCmd.PersistentFlags().StringVar(&nodeAddr, "cp-node-addr", "", "address of control plane node for generating join command")

	cpNodeCmd.PersistentFlags().StringVar(&kubeArmorRelayServerImage, "kubearmor-relay-server", "", "KubeArmor relay-server image to use")
	cpNodeCmd.PersistentFlags().StringVar(&siaImage, "sia-image", "", "sia image to use")
	cpNodeCmd.PersistentFlags().StringVar(&peaImage, "pea-image", "", "pea image to use")
	cpNodeCmd.PersistentFlags().StringVar(&feederImage, "feeder-image", "", "feeder-service image to use")

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
