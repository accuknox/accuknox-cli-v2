package deboard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
)

// UninstallScanners attempts every scanner even if another uninstall fails.
// Missing installations are expected, including on repeated deboarding.
func UninstallScanners(ctx context.Context) error {
	return UninstallScannersWithOutput(ctx, os.Stdout)
}

// UninstallScannersWithOutput reports which installed scanners were removed.
func UninstallScannersWithOutput(ctx context.Context, output io.Writer) error {
	return UninstallSelectedScannersWithOutput(ctx, output, ScannerSelection{VM: true, RRA: true, Image: true})
}

type ScannerSelection struct {
	VM    bool
	RRA   bool
	Image bool
}

func (s ScannerSelection) Names() []string {
	var names []string
	if s.VM {
		names = append(names, "VM scanner")
	}
	if s.RRA {
		names = append(names, "RRA")
	}
	if s.Image {
		names = append(names, "image scanner")
	}
	return names
}

func UninstallSelectedScannersWithOutput(ctx context.Context, output io.Writer, selected ScannerSelection) error {
	if len(selected.Names()) == 0 {
		return fmt.Errorf("select at least one scanner to remove")
	}
	return uninstallSelectedScanners([]scannerUninstaller{
		{
			"VM scanner",
			func() error {
				return onboard.UninstallVMScanner(ctx)
			},
		},
		{
			"RRA",
			UninstallRRA,
		},
		{
			"image scanner",
			UninstallImagescan,
		},
	}, selected, output)
}

func uninstallSelectedScanners(scanners []scannerUninstaller, selected ScannerSelection, output io.Writer) error {
	var targets []scannerUninstaller
	for _, scanner := range scanners {
		for _, name := range selected.Names() {
			if scanner.name == name {
				targets = append(targets, scanner)
				break
			}
		}
	}
	return uninstallScanners(targets, output)
}

type scannerUninstaller struct {
	name string
	run  func() error
}

func uninstallScanners(scanners []scannerUninstaller, output io.Writer) error {
	if output == nil {
		output = io.Discard
	}
	var failures []error
	for _, scanner := range scanners {
		err := scanner.run()
		switch {
		case os.IsNotExist(err):
			fmt.Fprintf(output, "%s is not installed; skipping.\n", scanner.name)
		case err != nil:
			fmt.Fprintf(output, "Failed to remove %s.\n", scanner.name)
			failures = append(failures, fmt.Errorf("removing %s: %w", scanner.name, err))
		default:
			fmt.Fprintf(output, "%s removed successfully.\n", scanner.name)
		}
	}
	return errors.Join(failures...)
}
