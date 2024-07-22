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

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().BoolVar(&scanOpts.FilterEventType.All, "all", false, "collect 'all' events, may get verbose")
	scanCmd.Flags().BoolVar(&scanOpts.FilterEventType.System, "system", false, "collect 'system' only events")
	scanCmd.Flags().StringVar(&scanOpts.Output, "output", "", "output path for the riles to be placed")
}
