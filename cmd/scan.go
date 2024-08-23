package cmd

import (
	"fmt"

	"github.com/accuknox/accuknox-cli-v2/pkg/scan"
	"github.com/spf13/cobra"
)

var scanOpts scan.ScanOptions

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Runtime scanning of CI/CD pipelines",
	Long:  "Scans the events taking place in CI/CD pipelines powered by KubeArmor",
	RunE: func(cmd *cobra.Command, args []string) error {
		scanner := scan.New(&scanOpts)

		if err := scanner.Start(); err != nil {
			fmt.Println(err)
			return err
		}

		return nil
	},
}

var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Manage policies for scanning",
	Long:  "Apply, generate, or manage policies for the scanning process",
	RunE: func(cmd *cobra.Command, args []string) error {
		if scanOpts.PolicyAction != "Block" && scanOpts.PolicyAction != "Audit" {
			return fmt.Errorf("invalid policy action: %s. Must be 'Block' or 'Audit'", scanOpts.PolicyAction)
		}

		if scanOpts.PolicyEvent != "ADDED" && scanOpts.PolicyEvent != "DELETED" {
			return fmt.Errorf("invalid policy event: %s. Must be 'ADDED' or 'DELETED'", scanOpts.PolicyEvent)
		}

		scanner := scan.New(&scanOpts)
		return scanner.HandlePolicies()
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
	scanCmd.AddCommand(policyCmd)

	scanCmd.PersistentFlags().BoolVar(&scanOpts.FilterEventType.All, "all", false, "collect 'all' events, may get verbose")
	scanCmd.PersistentFlags().BoolVar(&scanOpts.FilterEventType.System, "system", false, "collect 'system' only events")
	scanCmd.PersistentFlags().StringVar(&scanOpts.Output, "output", "", "output path for the files to be placed")

	policyCmd.Flags().StringVar(&scanOpts.PolicyAction, "action", "Audit", "Policy action: 'Block' or 'Audit'")
	policyCmd.Flags().StringVar(&scanOpts.PolicyEvent, "event", "ADDED", "Policy event: 'ADDED' or 'DELETED'")
	policyCmd.Flags().BoolVar(&scanOpts.PolicyDryRun, "dryrun", false, "generate and save the hardening policies but don't apply them")
	policyCmd.Flags().StringVar(&scanOpts.RepoBranch, "branch", "main", "Branch of the policy templates repository")
}

