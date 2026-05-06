// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Authors of KubeArmor

package cbom

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

// cryptoEntry describes a known cryptographic package and its CycloneDX properties.
type cryptoEntry struct {
	name        string
	description string
	primitive   cdx.CryptoPrimitive
	params      string // parameter set identifier (e.g. key size)
	curve       string
	mode        cdx.CryptoAlgorithmMode
	functions   []cdx.CryptoFunction
	assetType   cdx.CryptoAssetType
	protocol    cdx.CryptoProtocolType
	refs        []cdx.ExternalReference
	license     string // SPDX license identifier
}

// occurrence tracks a source location where a crypto import was found.
type occurrence struct {
	file string
	line int
}

// ref is a convenience helper for building an ExternalReference.
func ref(refType cdx.ExternalReferenceType, url string) cdx.ExternalReference {
	return cdx.ExternalReference{Type: refType, URL: url}
}

// languageScanner detects crypto imports in files of a particular language.
type languageScanner interface {
	// Extensions returns the file extensions this scanner handles (e.g. ".go", ".py").
	Extensions() []string
	// ScanFile scans a single file and returns any crypto import occurrences found.
	ScanFile(path string) (map[string][]occurrence, error)
}

// ScanSource walks the directory at path, detects the languages present, and
// returns CycloneDX components for every distinct crypto package imported.
func ScanSource(path string) ([]cdx.Component, error) {
	scanners := []languageScanner{
		&goScanner{},
		&pythonScanner{},
	}

	// Build extension → scanner lookup.
	extMap := map[string]languageScanner{}
	for _, s := range scanners {
		for _, ext := range s.Extensions() {
			extMap[ext] = s
		}
	}

	seen := map[string][]occurrence{}

	err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p == path {
				return nil
			}
			name := d.Name()
			if name == "vendor" || name == "testdata" || name == "node_modules" ||
				name == "__pycache__" || name == ".tox" || name == ".venv" || name == "venv" ||
				strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(p)
		scanner, ok := extMap[ext]
		if !ok {
			return nil
		}

		found, scanErr := scanner.ScanFile(p)
		if scanErr != nil {
			return nil // skip files that cannot be parsed
		}
		for pkg, occs := range found {
			seen[pkg] = append(seen[pkg], occs...)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", path, err)
	}

	return buildComponents(seen), nil
}

// buildComponents converts the collected import occurrences into CycloneDX components.
func buildComponents(seen map[string][]occurrence) []cdx.Component {
	components := make([]cdx.Component, 0, len(seen))

	for importPath, occs := range seen {
		entry, ok := lookupEntry(importPath)
		if !ok {
			continue
		}
		comp := cdx.Component{
			BOMRef:      fmt.Sprintf("crypto/%s/%s", entry.name, importPath),
			Type:        cdx.ComponentTypeCryptographicAsset,
			Name:        entry.name,
			Description: entry.description,
			Scope:       cdx.ScopeRequired,
		}

		if len(entry.refs) > 0 {
			refs := entry.refs
			comp.ExternalReferences = &refs
		}

		if entry.license != "" {
			lc := cdx.LicenseChoice{License: &cdx.License{ID: entry.license}}
			comp.Licenses = &cdx.Licenses{lc}
		}

		switch entry.assetType {
		case cdx.CryptoAssetTypeAlgorithm:
			funcs := entry.functions
			comp.CryptoProperties = &cdx.CryptoProperties{
				AssetType: cdx.CryptoAssetTypeAlgorithm,
				AlgorithmProperties: &cdx.CryptoAlgorithmProperties{
					Primitive:              entry.primitive,
					ParameterSetIdentifier: entry.params,
					Curve:                  entry.curve,
					Mode:                   entry.mode,
					CryptoFunctions:        &funcs,
				},
			}
		case cdx.CryptoAssetTypeProtocol:
			comp.CryptoProperties = &cdx.CryptoProperties{
				AssetType: cdx.CryptoAssetTypeProtocol,
				ProtocolProperties: &cdx.CryptoProtocolProperties{
					Type: entry.protocol,
				},
			}
		case cdx.CryptoAssetTypeCertificate:
			comp.CryptoProperties = &cdx.CryptoProperties{
				AssetType:             cdx.CryptoAssetTypeCertificate,
				CertificateProperties: &cdx.CertificateProperties{},
			}
		}

		evOccs := make([]cdx.EvidenceOccurrence, 0, len(occs))
		for _, o := range occs {
			line := o.line
			evOccs = append(evOccs, cdx.EvidenceOccurrence{
				Location: o.file,
				Line:     &line,
			})
		}
		comp.Evidence = &cdx.Evidence{Occurrences: &evOccs}

		components = append(components, comp)
	}

	return components
}

// lookupEntry finds the cryptoEntry for a given import path by searching
// all registered language-specific known-package maps.
func lookupEntry(importPath string) (cryptoEntry, bool) {
	if e, ok := knownGoPackages[importPath]; ok {
		return e, true
	}
	if e, ok := knownPythonPackages[importPath]; ok {
		return e, true
	}
	return cryptoEntry{}, false
}
