// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package cmd

import (
	"fmt"
	"github.com/accuknox/accuknox-cli-v2/pkg/summary"
	"github.com/spf13/cobra"
)

var summaryOptions summary.Options

// summaryCmd represents the summary command
var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Observability from discovery engine",
	Long:  `Discovery engine keeps the telemetry information from the policy enforcement engines and the accuknox-cli connects to it to provide this as observability data`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := summary.Summary(client, summaryOptions); err != nil {
			fmt.Println(err)
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(summaryCmd)

	summaryCmd.Flags().StringVar(&summaryOptions.GRPC, "gRPC", "", "gRPC server information")
	summaryCmd.Flags().StringArrayVarP(&summaryOptions.Labels, "labels", "l", []string{}, "Labels")
	summaryCmd.Flags().StringArrayVarP(&summaryOptions.Namespace, "namespace", "n", []string{}, "Namespace")
	summaryCmd.Flags().StringVarP(&summaryOptions.Operation, "type", "t", "", "Summary filter type : process|file|network ")
	summaryCmd.Flags().StringArrayVarP(&summaryOptions.Clusters, "cluster", "c", []string{}, "Cluster name")
	summaryCmd.Flags().StringVarP(&summaryOptions.Output, "output", "o", "", "Export Summary Data in JSON (accuknox-cli summary -o json)")
	summaryCmd.Flags().BoolVar(&summaryOptions.RevDNSLookup, "rev-dns-lookup", false, "Reverse DNS Lookup")
	//summaryCmd.Flags().BoolVar(&summaryOptions.Aggregation, "agg", false, "Aggregate destination files/folder path")
}
