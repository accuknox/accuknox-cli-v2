package cmd

import (
	"github.com/accuknox/accuknox-cli-v2/pkg/top"
	"github.com/spf13/cobra"
)

var topOptions top.Options

var topCmd = &cobra.Command{
	Use:   "top",
	Short: "Show resource usage for accuknox-agents",
	Long:  `Show resource usage for accuknox-agents`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := top.Top(client, topOptions); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(topCmd)
	topCmd.Flags().IntVar(&topOptions.RealTimeUpdateInterval, "real-time", 5, "Real-time update interval (seconds), set time greater than 5")
}
