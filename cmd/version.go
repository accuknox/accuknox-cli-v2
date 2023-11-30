// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	"github.com/accuknox/accuknox-cli-v2/pkg/version"
	"github.com/spf13/cobra"
)

var option version.Option

// versionCmd represents the get command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display version information",
	Long:  `Display version information`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := version.PrintVersion(client, option.GitPATPath); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().StringVar(&option.GitPATPath, "git-pat", "", "Print client version")
}
