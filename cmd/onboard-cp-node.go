/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"log"

	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/spf13/cobra"
)

var (
	joinToken   string
	spireHost   string
	ppsHost     string
	knoxGateway string
)

// cpNodeCmd represents the init command
var cpNodeCmd = &cobra.Command{
	Use:   "cp-node",
	Short: "Initialize a control plane node for onboarding onto SaaS",
	Long:  "Initialize a control plane node for onboarding onto SaaS",
	Run: func(cmd *cobra.Command, args []string) {
		clusterTypeValue := onboard.ClusterTypeValues[clusterType]

		clusterConfig, err := onboard.CreateClusterConfig(clusterTypeValue, kubearmorVersion, agentVersion, kubeArmorImage, kubeArmorInitImage, kubeArmorVMAdapterImage, kubeArmorRelayServerImage, siaImage, peaImage, feederImage, false)

		onboardConfig := onboard.InitCPNodeConfig(*clusterConfig, joinToken, spireHost, ppsHost, knoxGateway)

		err = onboardConfig.InitializeControlPlane()
		if err != nil {
			log.Fatalln("Failed to onboard control plane node:", err.Error())
		}

		log.Println("VM successfully onboarded!")
	},
}

func init() {
	// configuration for connecting with accuKnox SaaS
	cpNodeCmd.PersistentFlags().StringVar(&joinToken, "join-token", "", "join-token to use")
	cpNodeCmd.PersistentFlags().StringVar(&spireHost, "spire-host", "", "address of spire-host to connect for authenticating with accuknox SaaS")
	cpNodeCmd.PersistentFlags().StringVar(&ppsHost, "pps-host", "", "address of policy-provider-service to connect with for receiving policies")
	cpNodeCmd.PersistentFlags().StringVar(&knoxGateway, "knox-gateway", "", "address of knox-gateway to connect with for pushing telemetry data")

	cpNodeCmd.MarkPersistentFlagRequired("join-token")
	cpNodeCmd.MarkPersistentFlagRequired("spire-host")
	cpNodeCmd.MarkPersistentFlagRequired("pps-host")
	cpNodeCmd.MarkPersistentFlagRequired("knox-gateway")

	onboardCmd.AddCommand(cpNodeCmd)
}
