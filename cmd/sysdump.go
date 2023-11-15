// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"strings"
	"time"

	"github.com/accuknox/accuknox-cli-v2/pkg/sysdump"
	karmorSysdump "github.com/kubearmor/kubearmor-client/sysdump"
	"github.com/spf13/cobra"
)

var dumpOptions sysdump.Options
var karmorDumpOptions karmorSysdump.Options

// sysdumpCmd represents the get command
var sysdumpCmd = &cobra.Command{
	Use:   "sysdump",
	Short: "Collect system dump information for troubleshooting and error report",
	Long:  `Collect system dump information for troubleshooting and error reports`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := karmorSysdump.Collect(client, karmorDumpOptions); err != nil {
			return err
		}
		if err := sysdump.Collect(client, dumpOptions); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(sysdumpCmd)
	dev2File := "discovery-engine-sysdump-" + strings.Replace(time.Now().Format(time.UnixDate), ":", "_", -1) + ".zip"
	kubearmorFile := "kubearmor-sysdump-" + strings.Replace(time.Now().Format(time.UnixDate), ":", "_", -1) + ".zip"
	sysdumpCmd.Flags().StringVarP(&karmorDumpOptions.Filename, "kubearmor-sysdump", "k", kubearmorFile, "output file to use for Kubearmor dump")
	sysdumpCmd.Flags().StringVarP(&dumpOptions.Filename, "discovery-engine-sysdump", "f", dev2File, "output file to use for discovery engine dump")

}
