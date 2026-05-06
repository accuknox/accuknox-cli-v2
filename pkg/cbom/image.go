// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Authors of KubeArmor

package cbom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	_ "embed"
	"os/exec"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

//go:embed cbomkit-theia-bin
var cbomkitBinary []byte

// scannerBin is the filename used when extracting the embedded image scanner.
const scannerBin = "knoxctl-cbom-scanner"

// ScanImage extracts the embedded image scanner, runs it against a container
// image reference, and returns the parsed CycloneDX BOM.
func ScanImage(opts *Options) (*cdx.BOM, error) {
	bin, cleanup, err := extractBinary()
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return runScanner(bin, "image", opts.Image, opts)
}

// ScanDir extracts the embedded image scanner, runs it against a local
// directory, and returns the parsed CycloneDX BOM.
func ScanDir(opts *Options) (*cdx.BOM, error) {
	bin, cleanup, err := extractBinary()
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return runScanner(bin, "dir", opts.Path, opts)
}

// extractBinary writes the embedded scanner binary to a temporary file and
// returns its path along with a cleanup function.
func extractBinary() (path string, cleanup func(), err error) {
	tmpDir, err := os.MkdirTemp("", "knoxctl-cbom-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp dir for image scanner: %w", err)
	}
	cleanup = func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to remove temp dir %s: %v\n", tmpDir, err)
		}
	}

	path = filepath.Join(tmpDir, scannerBin)
	if runtime.GOOS == "windows" {
		path += ".exe"
	}
	if err = os.WriteFile(path, cbomkitBinary, 0o700); err != nil { // #nosec G306 — needs execute permission
		cleanup()
		return "", nil, fmt.Errorf("writing image scanner binary: %w", err)
	}
	return path, cleanup, nil
}

// runScanner executes the image scanner with <subcmd> [flags] <target> and
// unmarshals the CycloneDX JSON written to stdout.
func runScanner(bin, subcmd, target string, opts *Options) (*cdx.BOM, error) {
	args := []string{subcmd}
	if opts.Plugins != "" {
		args = append(args, "--plugins", opts.Plugins)
	}
	if opts.BOMFile != "" {
		args = append(args, "--bom", opts.BOMFile)
	}
	if opts.Ignore != "" {
		args = append(args, "--ignore", opts.Ignore)
	}
	args = append(args, target)

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(bin, args...) // #nosec G204 — args are user-supplied CLI values, not shell
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := stderr.String()
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("image scanner: %s", msg)
	}

	var bom cdx.BOM
	if err := json.Unmarshal(stdout.Bytes(), &bom); err != nil {
		return nil, fmt.Errorf("parsing image scanner output: %w", err)
	}
	return &bom, nil
}
