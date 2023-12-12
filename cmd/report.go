package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/report"
	"github.com/accuknox/accuknox-cli-v2/pkg/summary"
	"github.com/spf13/cobra"
)

var reportOptions summary.Options

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Report from discovery engine",
	Long:  `Discovery engine keeps the telemetry information from the policy enforcement engines and the knoxctl connects to it to provide this as observability data`,
	RunE: func(cmd *cobra.Command, args []string) error {
		rawArgs := strings.Join(os.Args[2:], " ")
		parseArgs, err := summary.ProcessArgs(rawArgs)
		if err != nil {
			return fmt.Errorf("errors processing args: %v", err)
		}

		if err := report.Report(client, parseArgs); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(reportCmd)

	reportCmd.Flags().StringVar(&reportOptions.GRPC, "gRPC", "", "gRPC server information")
	reportCmd.Flags().StringSliceVarP(&reportOptions.Namespace, "namespaces", "n", []string{}, "Namespaces names")
	reportCmd.Flags().StringSliceVarP(&reportOptions.Workloads, "workload", "", []string{}, "Workloads")
	reportCmd.Flags().StringSliceVar(&reportOptions.Labels, "labels", []string{}, "Labels")
	reportCmd.Flags().StringSliceVarP(&reportOptions.Source, "source", "s", []string{}, "Source path")
	reportCmd.Flags().StringSliceVarP(&reportOptions.Destination, "destination", "d", []string{}, "Destination path")
	reportCmd.Flags().StringVarP(&reportOptions.Operation, "operation", "", "", "Operation type")
	reportCmd.Flags().StringSliceVarP(&reportOptions.IgnorePath, "ignore-path", "", []string{}, "Destination path")
	reportCmd.Flags().StringVarP(&reportOptions.BaselineSummaryPath, "baseline", "", "baseline/report.json", "Baseline summary path")
	reportCmd.Flags().StringVarP(&reportOptions.View, "view", "v", "", "View type")
	reportCmd.Flags().BoolVarP(&reportOptions.Dump, "dump", "", false, "Dump")
	reportCmd.Flags().StringSliceVarP(&reportOptions.IgnoreCommand, "ignore-command", "", []string{}, "Ignore command")
	reportCmd.Flags().BoolVarP(&reportOptions.Debug, "debug", "", false, "Debug by printing the nodes")
}
