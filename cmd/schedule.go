// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Authors of KubeArmor

package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/accuknox/accuknox-cli-v2/pkg/ui"
	"github.com/spf13/cobra"
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Manage scheduled BOM generation jobs",
	Long: `Manage scheduled Bill of Materials jobs. Jobs are created and managed from
the knoxctl web UI (Schedules screen) and executed on a recurring basis by the
host scheduler (cron on Linux, Task Scheduler on Windows), which invokes
'knoxctl schedule run <id>'.`,
	// Scheduled jobs run headless and need no k8s client.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
}

var scheduleRunCmd = &cobra.Command{
	Use:   "run <job-id>",
	Short: "Generate and publish a scheduled BOM job (invoked by the OS scheduler)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ui.RunJobByID(context.Background(), args[0], os.Stdout)
	},
}

var scheduleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List scheduled BOM jobs",
	RunE: func(cmd *cobra.Command, args []string) error {
		jobs := ui.ListJobs()
		if len(jobs) == 0 {
			fmt.Println("No scheduled jobs.")
			return nil
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tNAME\tFORMAT\tFREQUENCY\tPAUSED\tLAST RUN\tLAST STATUS")
		for _, j := range jobs {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%t\t%s\t%s\n",
				j.ID, j.Name, j.SBOM.Format, j.Schedule.Frequency, j.Paused, j.LastRun, j.LastStatus)
		}
		return tw.Flush()
	},
}

func init() {
	rootCmd.AddCommand(scheduleCmd)
	scheduleCmd.AddCommand(scheduleRunCmd)
	scheduleCmd.AddCommand(scheduleListCmd)
}
