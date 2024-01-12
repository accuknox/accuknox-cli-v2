// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	dev2install "github.com/accuknox/accuknox-cli-v2/pkg/install"
	"github.com/kubearmor/kubearmor-client/install"
	"github.com/spf13/cobra"
)

var uninstallOptions install.Options
var uninstallKubearmor bool
var uninstallDev2 bool

// uninstallCmd represents the get command
var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall KubeArmor from a Kubernetes Cluster",
	Long:  `Uninstall KubeArmor from a Kubernetes Clusters`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if uninstallKubearmor {
			if err := install.K8sUninstaller(client, uninstallOptions); err != nil {
				return err
			}
		}

		if uninstallDev2 {
			if err := dev2install.Remove(client); err != nil {
				return err
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
	uninstallCmd.Flags().StringVarP(&uninstallOptions.Namespace, "namespace", "n", "kube-system", "Namespace for kubearmor resources")
	uninstallCmd.Flags().BoolVar(&uninstallOptions.Force, "force", false, "Force remove KubeArmor annotations from deployments. (Deployments might be restarted)")
	uninstallCmd.Flags().BoolVar(&uninstallKubearmor, "kubearmor", true, "uninstall KubeArmor")
	uninstallCmd.Flags().BoolVar(&uninstallDev2, "discovery-engine", true, "uninstall Discovery Engine")
}
