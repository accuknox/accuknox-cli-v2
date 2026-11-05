package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/deboard"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/spf13/cobra"
)

// deboardNodeCmd represents the deboardNode command
var deboardRRAScanner = newDeboardScannerCmd(deboard.UninstallSelectedScannersWithOutput)

func newDeboardScannerCmd(uninstall func(context.Context, io.Writer, deboard.ScannerSelection) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "scanner",
		Short:             "Remove all or selected VM scanners (Omni, RRA, and image scanner)",
		Long:              "Remove all scanners by default. Specify --vm-scanner, --rra-scan, or --image-scan to remove only the selected scanners. --all explicitly removes all scanners.",
		Args:              cobra.NoArgs,
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			selected, err := deboardScannerSelection(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Would remove: %s\n", strings.Join(selected.Names(), ", "))
				return nil
			}
			if err := uninstall(cmd.Context(), cmd.OutOrStdout(), selected); err != nil {
				return err
			}
			logger.PrintSuccess("VM scanner cleanup completed.")
			return nil
		},
	}
	cmd.Flags().Bool("vm-scanner", false, "remove the Omni VM scanner")
	cmd.Flags().Bool("rra-scan", false, "remove the RRA scanner")
	cmd.Flags().Bool("image-scan", false, "remove the container image scanner")
	cmd.Flags().Bool("all", false, "remove all scanners (default when no selection flags are supplied)")
	return cmd
}

func deboardScannerSelection(cmd *cobra.Command) (deboard.ScannerSelection, error) {
	changed := false
	for _, name := range []string{"vm-scanner", "rra-scan", "image-scan"} {
		if flag := cmd.Flag(name); flag != nil && flag.Changed {
			changed = true
		}
	}
	all := cmd.Flag("all")
	if all != nil && all.Changed {
		if !flagTrue(cmd, "all") {
			return deboard.ScannerSelection{}, fmt.Errorf("--all=false does not select scanners; choose --vm-scanner, --rra-scan, or --image-scan")
		}
		if changed {
			return deboard.ScannerSelection{}, fmt.Errorf("--all cannot be combined with scanner selection flags")
		}
	}
	if !changed {
		return deboard.ScannerSelection{VM: true, RRA: true, Image: true}, nil
	}
	selected := deboard.ScannerSelection{VM: flagTrue(cmd, "vm-scanner"), RRA: flagTrue(cmd, "rra-scan"), Image: flagTrue(cmd, "image-scan")}
	if len(selected.Names()) == 0 {
		return selected, fmt.Errorf("select at least one scanner to remove")
	}
	return selected, nil
}

func init() {
	deboardVMCmd.AddCommand(deboardRRAScanner)
}
