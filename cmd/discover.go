// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/discover"
	"github.com/spf13/cobra"
)

var parseArgs discover.Options

// discoverCmd represents the discover command
var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover applicable policies",
	Long:  `Discover applicable policies`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rawArgs := strings.Join(os.Args[2:], " ")

		parseArgs, err := discover.ProcessArgs(rawArgs)
		if err != nil {
			return fmt.Errorf("error processing args: %v", err)
		}

		if err := discover.Policy(client, parseArgs); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(discoverCmd)
	discoverCmd.Flags().StringVar(&parseArgs.GRPC, "gRPC", "", "gRPC server information")
	discoverCmd.Flags().StringVarP(&parseArgs.Format, "format", "f", "", "Format: json or yaml")
	discoverCmd.Flags().BoolVar(&parseArgs.Dump, "dump", false, "Dump policies to knoxctl_out directory and skip TUI")
	discoverCmd.Flags().StringSliceVarP(&parseArgs.Kind, "policy", "p", []string{"KubeArmorPolicy"}, "Type of policies to be discovered: NetworkPolicy|KubeArmorPolicy|KubeArmorHostPolicy")
	discoverCmd.Flags().StringSliceVarP(&parseArgs.Namespace, "namespace", "n", []string{}, "Filter by Namespace")
	discoverCmd.Flags().StringSliceVarP(&parseArgs.Labels, "labels", "l", []string{}, "Filter by policy Label")
	discoverCmd.Flags().StringSliceVarP(&parseArgs.Source, "source", "s", []string{}, "Filter by policy FromSource")
}
