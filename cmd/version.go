// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"github.com/accuknox/accuknox-cli-v2/pkg/version"
	"github.com/spf13/cobra"
)

var opts version.Option

// versionCmd represents the get command
var versionCmd = &cobra.Command{
	Use:               "version",
	PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
	Short:             "Display version information",
	Long:              `Display Knoxctl and agent versions. Agent output shows name, version, and status by default. Use --long for full image references and diagnostic details.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := version.PrintVersion(client, opts); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().BoolVar(&opts.LatestRelease, "latest-release", false, "Gets the latest release tag for knoxctl")
	versionCmd.Flags().BoolVar(&opts.Long, "long", false, "show full image references, container image IDs, and checksum details")
	versionCmd.Flags().StringVar(&opts.VMMode, "vm-mode", "auto", "agent installation mode: auto, docker, or systemd")
}
