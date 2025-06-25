// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package cmd

import (
	"github.com/accuknox/accuknox-cli-v2/pkg/update"
	"github.com/spf13/cobra"
)

var options update.Option

// selfUpdateCmd represents the get command
var selfUpdateCmd = &cobra.Command{
	Use:   "selfupdate",
	Short: "update knoxctl",
	Long:  `update knoxctl to sync with latest and greatest updates`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := update.SelfUpdate(client, &options); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(selfUpdateCmd)
	selfUpdateCmd.Flags().BoolVarP(&options.DoUpdate, "yes", "y", false, "Force update to latest version")
}
