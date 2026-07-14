package cmd

import "strings"

// enrichPkgscanCycloneDXArgs enables Syft's package metadata enrichment for
// CycloneDX JSON scans. This adds licenses where ecosystem metadata can be
// resolved while preserving explicit user enrichment settings and all other
// pkgscan commands and output formats.
func enrichPkgscanCycloneDXArgs(toolName string, args []string) []string {
	if toolName != "pkgscan" || len(args) == 0 || args[0] != "scan" {
		return args
	}

	cycloneDXOutput := false
	for i, arg := range args {
		if arg == "--enrich" || strings.HasPrefix(arg, "--enrich=") {
			return args
		}

		var output string
		switch {
		case (arg == "-o" || arg == "--output") && i+1 < len(args):
			output = args[i+1]
		case strings.HasPrefix(arg, "-o="):
			output = strings.TrimPrefix(arg, "-o=")
		case strings.HasPrefix(arg, "--output="):
			output = strings.TrimPrefix(arg, "--output=")
		}

		if output == "cyclonedx-json" || strings.HasPrefix(output, "cyclonedx-json=") {
			cycloneDXOutput = true
		}
	}

	if !cycloneDXOutput {
		return args
	}

	enriched := append([]string(nil), args...)
	return append(enriched, "--enrich", "all")
}
