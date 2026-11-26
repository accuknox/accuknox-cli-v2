package cmd

import (
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/spf13/cobra"
)

func newRestartCountsCmd() *cobra.Command {
	var exclude []string
	var reset bool
	command := &cobra.Command{
		Use:   "restart-counts",
		Short: "Show or reset systemd restart counters for agents",
		Long: `Show each agent's systemd NRestarts counter and current state.
The counter tracks automatic restarts, not starts within the StartLimitBurst window.
Unloaded services have no available counter.

By default, only read counters. With --reset, clear failure states and restart
counters and start agents that are not running, matching the existing reset behavior.
`,
		Example:           "  knoxctl vm restart-counts\n  knoxctl vm restart-counts --reset\n  knoxctl vm restart-counts --reset --exclude spire-agent.service",
		Args:              cobra.NoArgs,
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		RunE: func(cmd *cobra.Command, _ []string) error {
			excluded := make(map[string]bool, len(exclude))
			for _, service := range exclude {
				excluded[strings.TrimSuffix(service, ".service")+".service"] = true
			}
			if reset {
				return onboard.ResetRestartCount(excluded)
			}
			return onboard.PrintAgentRestartCounts(cmd.Context(), cmd.OutOrStdout(), excluded)
		},
	}
	command.Flags().StringSliceVar(&exclude, "exclude", nil, "Service names to omit (with or without .service)")
	command.Flags().BoolVar(&reset, "reset", false, "Reset failure states and restart counters, and start agents that are not running")
	return command
}

func init() {
	vmCmd.AddCommand(newRestartCountsCmd())
}
