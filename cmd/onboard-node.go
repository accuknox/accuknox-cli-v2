package cmd

import (
	"fmt"
	"log"

	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/spf13/cobra"
)

var (
	kubeArmorAddr   string
	relayServerAddr string
	SIAAddr         string
	PEAAddr         string
)

// joinNodeCmd represents the join command
var joinNodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Join this worker node with the control plane node for onboarding onto SaaS",
	Long:  "Join this worker node with the control plane node for onboarding onto SaaS",
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterTypeValue := onboard.ClusterTypeValues[clusterType]

		clusterConfig, err := onboard.CreateClusterConfig(clusterTypeValue, kubearmorVersion, releaseVersion, kubeArmorImage, kubeArmorInitImage, kubeArmorVMAdapterImage, kubeArmorRelayServerImage, siaImage, peaImage, feederImage, nodeAddr, dryRun, true)
		if err != nil {
			return fmt.Errorf("Failed to create cluster config: %s", err.Error())
		}

		joinConfig := onboard.JoinClusterConfig(*clusterConfig, kubeArmorAddr, relayServerAddr, SIAAddr, PEAAddr)

		err = joinConfig.JoinWorkerNode()
		if err != nil {
			return fmt.Errorf("Failed to join worker node: %s", err.Error())
		}

		log.Println("VM successfully joined with control-plane!")

		return nil
	},
}

func init() {
	// configuration for connecting with KubeArmor and control plane
	joinNodeCmd.PersistentFlags().StringVarP(&clusterType, "type", "t", "", "type of cluster to onboard. possible values VM")
	joinNodeCmd.PersistentFlags().StringVar(&kubeArmorAddr, "kubearmor-addr", "", "address of kubearmor on this node")
	joinNodeCmd.PersistentFlags().StringVar(&relayServerAddr, "relay-server-addr", "", "address of relay-server on control plane to connect with for pushing telemetry events")
	joinNodeCmd.PersistentFlags().StringVar(&SIAAddr, "sia-addr", "", "address of shared-informer-agent on control plane to push state events")
	joinNodeCmd.PersistentFlags().StringVar(&PEAAddr, "pea-addr", "", "address of policy-enforcement-agent on control plane for receiving policy events")

	joinNodeCmd.PersistentFlags().StringVar(&nodeAddr, "cp-node-addr", "", "address of control plane")

	onboardCmd.AddCommand(joinNodeCmd)
}
