// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Authors of KubeArmor

package bomgen

import "strings"

// CycloneDXSpecVersion is the CycloneDX specification version the SBOM pipeline
// targets and validates against, in human form ("1.6"). The scanner engine
// (trivy 0.69.x) emits 1.6; the AIBOM path (handled elsewhere) is the only
// producer of 1.7. Used for UI display and as the default validation target.
const CycloneDXSpecVersion = "1.6"

// cdxInputVersionFlag converts a human spec version ("1.6") to the value the
// `cyclonedx` CLI expects for --input-version ("v1_6"). The CLI only recognises
// the underscore form; the dotted form ("v1.6") is silently ignored and the BOM
// is validated against the auto-detected version instead — which would let a
// 1.7 document pass a "1.6" check. Always use the underscore form.
func cdxInputVersionFlag(specVersion string) string {
	if specVersion == "" {
		specVersion = CycloneDXSpecVersion
	}
	return "v" + strings.ReplaceAll(specVersion, ".", "_")
}

// CycloneDXValidateArgs returns the arguments for the external `cyclonedx` CLI
// that validate inputFile against the given spec version, i.e.:
//
//	cyclonedx validate --input-file <inputFile> --input-version v1_6 --fail-on-errors
//
// --fail-on-errors is required: without it the CLI prints validation problems
// but still exits 0, so the caller cannot tell valid from invalid. The binary
// name itself is resolved by the caller.
func CycloneDXValidateArgs(specVersion, inputFile string) []string {
	return []string{
		"validate",
		"--input-file", inputFile,
		"--input-version", cdxInputVersionFlag(specVersion),
		"--fail-on-errors",
	}
}
