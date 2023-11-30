// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

// Package cmd is the collection of all the subcommands available in accuknox-cli while providing relevant options for the same
package cmd

import (
	//"github.com/accuknox/accuknox-cli/cmd/license"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var client *k8s.Client

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error

		//Initialise k8sClient for all child commands to inherit
		client, err = k8s.ConnectK8sClient()
		// fmt.Printf("%v", client.K8sClientset)
		if err != nil {
			log.Error().Msgf("unable to create Kubernetes clients: %s", err.Error())
			return err
		}
		return nil
	},
	Use:   "knoxctl",
	Short: "CLI tool to interact with KubeArmor and AccuKnox enterprise ecosystem.",
	Long: `
Interact with KubeArmor and AccuKnox enterprise feature including license management, system summary and policy management.
Harness the capabilites of AccuKnox eco-system with ease right from the command line. 
	`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// adding all the commands with sub commands
func addSubCommandPalettes() {
	//rootCmd.AddCommand(license.LicenseCmd)
}

func init() {
	rootCmd.PersistentFlags().StringVar(&k8s.KubeConfig, "kubeconfig", "", "Path to the kubeconfig file to use")
	rootCmd.PersistentFlags().StringVar(&k8s.ContextName, "context", "", "Name of the kubeconfig context to use")
	addSubCommandPalettes()
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}
