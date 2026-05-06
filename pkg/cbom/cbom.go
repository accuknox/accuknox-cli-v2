// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Authors of KubeArmor

// Package cbom generates Cryptography Bill of Materials (CBOM) documents
// compliant with CycloneDX 1.6. It supports two modes:
//
//   - Source scanning: walks Go source files and detects usage of known
//     cryptographic packages (stdlib crypto/* and golang.org/x/crypto/*).
//
//   - Image scanning: scans container images for certificates, keys, TLS
//     configuration, and secrets.
package cbom

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/google/uuid"

	"github.com/accuknox/accuknox-cli-v2/pkg/sign"
)

// GenerateFromSource scans a local directory and returns a CycloneDX BOM.
// It runs language-specific import detection (Go, Python, …) and the embedded
// cbomkit-theia binary scanner, then merges the results.
func GenerateFromSource(opts *Options) (*cdx.BOM, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("--path is required for source scanning")
	}

	// Language-specific import scanning (Go, Python, …).
	langComponents, err := ScanSource(opts.Path)
	if err != nil {
		return nil, err
	}

	// Embedded binary scanner (certificates, secrets, OpenSSL config, …).
	binBOM, binErr := ScanDir(opts)

	name := opts.Name
	if name == "" {
		name = opts.Path
	}

	// Merge: start from the binary scanner BOM (if available) and append
	// language-detected components that are not already present.
	if binErr == nil && binBOM != nil {
		existing := map[string]bool{}
		if binBOM.Components != nil {
			for _, c := range *binBOM.Components {
				existing[c.BOMRef] = true
			}
		}
		merged := make([]cdx.Component, 0)
		if binBOM.Components != nil {
			merged = append(merged, *binBOM.Components...)
		}
		for _, c := range langComponents {
			if !existing[c.BOMRef] {
				merged = append(merged, c)
			}
		}
		binBOM.Components = &merged
		sanitizeBOM(binBOM, opts.Path, "", opts)
		return binBOM, nil
	}

	// Binary scanner unavailable — build BOM from language scan only.
	return newBOM(langComponents, name, "", opts), nil
}

// GenerateFromImage scans a container image and returns a CycloneDX BOM.
func GenerateFromImage(opts *Options) (*cdx.BOM, error) {
	if opts.Image == "" {
		return nil, fmt.Errorf("--image is required for image scanning")
	}
	bom, err := ScanImage(opts)
	if err != nil {
		return nil, err
	}
	name := opts.Name
	if name == "" {
		name = opts.Image
	}
	sanitizeBOM(bom, "", name, opts)
	return bom, nil
}

// Output writes the BOM to stdout or to opts.OutputTo, in the format
// specified by opts.Format ("json" or "table").
func Output(bom *cdx.BOM, opts *Options) error {
	switch strings.ToLower(opts.Format) {
	case "table":
		return printTable(bom, opts)
	default:
		return printJSON(bom, opts)
	}
}

// toolComponent is the CycloneDX tool entry stamped into every CBOM produced
// by knoxctl, regardless of the underlying scanner used.
var toolComponent = cdx.Component{
	Type:      cdx.ComponentTypeApplication,
	Publisher: "AccuKnox",
	Name:      "knoxctl-cbom",
}

// newBOM constructs a CycloneDX 1.6 BOM envelope around components.
// source and image are mutually exclusive; pass one and leave the other empty.
func newBOM(components []cdx.Component, source, image string, opts *Options) *cdx.BOM {
	bom := cdx.NewBOM()
	bom.SerialNumber = "urn:uuid:" + uuid.New().String()
	bom.Metadata = &cdx.Metadata{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Lifecycles: &[]cdx.Lifecycle{{Phase: cdx.LifecyclePhaseBuild}},
		Tools: &cdx.ToolsChoice{
			Components: &[]cdx.Component{toolComponent},
		},
		Component: buildMetadataComponent(source, image, opts),
	}
	bom.Components = &components
	return bom
}

// sanitizeBOM overwrites the metadata of a BOM returned by an underlying
// scanner so that only "knoxctl-cbom" appears as the tool and the project
// metadata reflects the user-supplied options.
func sanitizeBOM(bom *cdx.BOM, source, image string, opts *Options) {
	if bom.Metadata == nil {
		bom.Metadata = &cdx.Metadata{}
	}
	bom.Metadata.Tools = &cdx.ToolsChoice{
		Components: &[]cdx.Component{toolComponent},
	}
	bom.Metadata.Lifecycles = &[]cdx.Lifecycle{{Phase: cdx.LifecyclePhaseBuild}}
	if bom.Metadata.Timestamp == "" {
		bom.Metadata.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	bom.Metadata.Component = buildMetadataComponent(source, image, opts)
	enforceLicenses(bom)
}

// enforceLicenses ensures every cryptographic-asset component in the BOM has a
// license entry. Components for which no license could be determined are marked
// "unknown" so the field is never silently absent from the output.
func enforceLicenses(bom *cdx.BOM) {
	if bom.Components == nil {
		return
	}
	unknown := cdx.Licenses{cdx.LicenseChoice{License: &cdx.License{ID: "unknown"}}}
	comps := *bom.Components
	for i := range comps {
		if comps[i].Type != cdx.ComponentTypeCryptographicAsset {
			continue
		}
		if comps[i].Licenses == nil || len(*comps[i].Licenses) == 0 {
			comps[i].Licenses = &unknown
		}
	}
}

// buildMetadataComponent constructs the metadata.component entry that
// identifies the project being scanned.
func buildMetadataComponent(source, image string, opts *Options) *cdx.Component {
	compType := cdx.ComponentTypeApplication
	name := source
	if image != "" {
		compType = cdx.ComponentTypeContainer
		name = image
	}

	c := &cdx.Component{
		Type:        compType,
		Name:        name,
		Group:       opts.Group,
		Version:     opts.Version,
		Description: opts.Description,
	}

	if opts.License != "" {
		licenseChoice := cdx.LicenseChoice{License: &cdx.License{ID: opts.License}}
		c.Licenses = &cdx.Licenses{licenseChoice}
	}

	if opts.Group != "" && opts.Version != "" {
		c.PackageURL = fmt.Sprintf("pkg:generic/%s/%s@%s", opts.Group, name, opts.Version)
	} else if opts.Version != "" {
		c.PackageURL = fmt.Sprintf("pkg:generic/%s@%s", name, opts.Version)
	}

	// bom-ref must always be present so other entries in the document can
	// reference this component. Prefer the purl (stable, globally unique);
	// fall back to name@version or just name.
	switch {
	case c.PackageURL != "":
		c.BOMRef = c.PackageURL
	case opts.Version != "":
		c.BOMRef = name + "@" + opts.Version
	default:
		c.BOMRef = name
	}

	return c
}

// printJSON marshals the BOM as indented JSON and writes it to stdout or file.
func printJSON(bom *cdx.BOM, opts *Options) error {
	data, err := json.MarshalIndent(bom, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling CBOM: %w", err)
	}
	if opts.OutputTo != "" {
		if err := os.WriteFile(opts.OutputTo, data, 0600); err != nil {
			return fmt.Errorf("writing CBOM to %s: %w", opts.OutputTo, err)
		}
		fmt.Printf("CBOM written to %s\n", opts.OutputTo)
		return sign.Artifact(opts.OutputTo, &opts.Sign)
	}
	fmt.Println(string(data))
	return nil
}

// printTable renders a human-readable summary of the cryptographic components.
func printTable(bom *cdx.BOM, opts *Options) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "NAME\tASSET TYPE\tPRIMITIVE\tPARAMETERS\tFUNCTIONS")
	fmt.Fprintln(w, "----\t----------\t---------\t----------\t---------")

	if bom.Components == nil {
		return nil
	}
	for _, c := range *bom.Components {
		if c.CryptoProperties == nil {
			continue
		}
		cp := c.CryptoProperties
		primitive := ""
		params := ""
		functions := ""

		if cp.AlgorithmProperties != nil {
			primitive = string(cp.AlgorithmProperties.Primitive)
			params = cp.AlgorithmProperties.ParameterSetIdentifier
			if cp.AlgorithmProperties.CryptoFunctions != nil {
				parts := make([]string, 0, len(*cp.AlgorithmProperties.CryptoFunctions))
				for _, f := range *cp.AlgorithmProperties.CryptoFunctions {
					parts = append(parts, string(f))
				}
				functions = strings.Join(parts, ",")
			}
		} else if cp.ProtocolProperties != nil {
			primitive = "protocol"
			params = string(cp.ProtocolProperties.Type)
			if cp.ProtocolProperties.Version != "" {
				params += " " + cp.ProtocolProperties.Version
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			c.Name, string(cp.AssetType), primitive, params, functions)
	}

	if opts.OutputTo != "" {
		fmt.Printf("\n(use --format json to save full CBOM to %s)\n", opts.OutputTo)
	}
	return nil
}

// ComponentCount returns the number of cryptographic-asset components in the BOM.
func ComponentCount(bom *cdx.BOM) int {
	if bom.Components == nil {
		return 0
	}
	count := 0
	for _, c := range *bom.Components {
		if c.Type == cdx.ComponentTypeCryptographicAsset {
			count++
		}
	}
	return count
}
