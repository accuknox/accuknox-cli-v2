// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Authors of KubeArmor

// Package bomgen holds shared Software Bill of Materials generation helpers used
// by both the web UI and scheduled (headless) BOM jobs: building the knoxctl
// sub-command arguments and stripping scanner-engine branding from the output.
package bomgen

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// SBOMParams describes a Software BOM generation request.
type SBOMParams struct {
	Source  string `json:"source"`
	Scheme  string `json:"scheme"`
	Format  string `json:"format"`
	Exclude string `json:"exclude"`
	// Depth enables a deep filesystem scan that includes the full dependency
	// graph (where the chosen output format supports it).
	Depth bool `json:"depth"`
}

// depthFormat maps a UI output-format value to the equivalent deep-scan format,
// returning ok=false when the format has no dependency-depth equivalent and must
// fall back to the standard package scanner.
func depthFormat(format string) (string, bool) {
	switch format {
	case "cyclonedx-json":
		return "cyclonedx", true
	case "spdx-json":
		return "spdx-json", true
	case "spdx-tag-value":
		return "spdx", true
	default:
		return "", false
	}
}

// UsesDepth reports whether these params will actually run a dependency-depth
// (filesystem) scan, i.e. the toggle is on and the format supports it.
func (p SBOMParams) UsesDepth() bool {
	if !p.Depth {
		return false
	}
	_, ok := depthFormat(p.Format)
	return ok
}

// SBOMArgs returns the knoxctl sub-command arguments that generate the SBOM into
// outFile. When dependency depth is enabled and the format supports it, a deep
// filesystem scan (full dependency graph) is used; otherwise the standard
// package scan is used. The underlying engine name is never surfaced to callers.
func SBOMArgs(p SBOMParams, outFile string) []string {
	if df, ok := depthFormat(p.Format); ok && p.Depth {
		// Deep filesystem scan — scheme/registry inputs do not apply.
		args := []string{"imgscan", "fs", "--format", df, "--output", outFile, p.Source}
		for _, e := range splitCSV(p.Exclude) {
			args = append(args, "--skip-dirs", e)
		}
		return args
	}

	source := p.Source
	if p.Scheme != "" {
		source = p.Scheme + ":" + source
	}
	args := []string{"pkgscan", "scan", source, "-o", p.Format + "=" + outFile}

	scheme := p.Scheme
	if scheme == "" {
		scheme = "dir"
	}
	if scheme == "dir" || scheme == "file" || scheme == "oci-dir" || scheme == "oci-archive" {
		if name := filepath.Base(p.Source); name != "" && name != "." {
			args = append(args, "--source-name", name)
		}
	}
	for _, e := range splitCSV(p.Exclude) {
		args = append(args, "--exclude", e)
	}
	return args
}

// CountComponents returns the number of packages/components in an SBOM document
// (CycloneDX, SPDX-JSON, or pkgscan-native JSON), or 0 if it cannot be parsed.
func CountComponents(doc []byte) int {
	var parsed struct {
		Components *[]json.RawMessage `json:"components"` // CycloneDX
		Artifacts  *[]json.RawMessage `json:"artifacts"`  // pkgscan native
		Packages   *[]json.RawMessage `json:"packages"`   // SPDX
	}
	if json.Unmarshal(doc, &parsed) != nil {
		return 0
	}
	switch {
	case parsed.Components != nil:
		return len(*parsed.Components)
	case parsed.Artifacts != nil:
		return len(*parsed.Artifacts)
	case parsed.Packages != nil:
		return len(*parsed.Packages)
	}
	return 0
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
