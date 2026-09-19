// SPDX-License-Identifier: Apache-2.0
// Copyright 2021 Authors of KubeArmor

package cmd

import (
	aspm "github.com/accuknox/accuknox-cli-v2/pkg/aspm"
	"github.com/spf13/cobra"
)

// aspmCmd represents the get command
var aspmCmd = &cobra.Command{
	Use:                "aspm",
	Short:              "Run AccuKnox ASPM scanner",
	Long:               "Run AccuKnox ASPM scanner",
	DisableFlagParsing: true, // disables Cobra's own flag parsing for this command
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := aspm.ExecuteASPM(); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(aspmCmd)
}
