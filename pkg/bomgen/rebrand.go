// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Authors of KubeArmor

package bomgen

import (
	"bytes"
	"encoding/json"
	"strings"
)

// StripDepthBranding removes the dependency-depth scanner engine's identity from
// generated SBOM output, replacing it with AccuKnox/knoxctl so the underlying
// tool is never surfaced to users.
//
// It edits only document/tool metadata — never the package list — so component
// names, purls, and bom-refs (which may legitimately contain the engine's name,
// e.g. a project that depends on it) are left untouched. On any parse error the
// input is returned unchanged.
func StripDepthBranding(data []byte, format string) []byte {
	switch format {
	case "cyclonedx-json":
		data = stripCycloneDX(data)
	case "spdx-json":
		data = stripSPDXJSON(data)
	case "spdx-tag-value":
		data = stripSPDXTagValue(data)
	}
	// Rebrand the engine's namespaced property/annotation keys. The colon form
	// "aquasecurity:trivy:" only appears in tool-generated metadata keys — never
	// in package URLs, which use "aquasecurity/trivy" with a slash — so this is
	// safe to apply across the whole document.
	data = bytes.ReplaceAll(data, []byte("aquasecurity:trivy:"), []byte("knoxctl:"))
	return data
}

// isEngineToken reports whether s names the scanner engine or its vendor.
func isEngineToken(s string) bool {
	l := strings.ToLower(s)
	return strings.Contains(l, "trivy") || strings.Contains(l, "aquasecurity") || strings.Contains(l, "aqua security")
}

func stripCycloneDX(data []byte) []byte {
	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		return data
	}
	meta, ok := doc["metadata"].(map[string]interface{})
	if !ok {
		return data
	}

	rebrandToolComponent := func(m map[string]interface{}) {
		if name, _ := m["name"].(string); isEngineToken(name) {
			m["name"] = "knoxctl"
		}
		if grp, _ := m["group"].(string); isEngineToken(grp) {
			m["group"] = "accuknox"
		}
		if vendor, _ := m["vendor"].(string); isEngineToken(vendor) {
			m["vendor"] = "AccuKnox"
		}
		if man, ok := m["manufacturer"].(map[string]interface{}); ok {
			if n, _ := man["name"].(string); isEngineToken(n) {
				man["name"] = "AccuKnox"
			}
		}
	}

	switch tools := meta["tools"].(type) {
	case map[string]interface{}: // CycloneDX 1.5+ : tools.components[]
		if comps, ok := tools["components"].([]interface{}); ok {
			for _, c := range comps {
				if m, ok := c.(map[string]interface{}); ok {
					rebrandToolComponent(m)
				}
			}
		}
	case []interface{}: // legacy CycloneDX : tools[]
		for _, t := range tools {
			if m, ok := t.(map[string]interface{}); ok {
				rebrandToolComponent(m)
			}
		}
	}

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return data
	}
	return out
}

func stripSPDXJSON(data []byte) []byte {
	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		return data
	}
	ci, ok := doc["creationInfo"].(map[string]interface{})
	if !ok {
		return data
	}
	creators, ok := ci["creators"].([]interface{})
	if !ok {
		return data
	}
	for i, c := range creators {
		s, ok := c.(string)
		if !ok || !isEngineToken(s) {
			continue
		}
		switch {
		case strings.HasPrefix(s, "Tool:"):
			creators[i] = "Tool: knoxctl"
		case strings.HasPrefix(s, "Organization:"):
			creators[i] = "Organization: AccuKnox"
		default:
			creators[i] = "Tool: knoxctl"
		}
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return data
	}
	return out
}

func stripSPDXTagValue(data []byte) []byte {
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "Creator:") || !isEngineToken(line) {
			continue
		}
		switch {
		case strings.HasPrefix(line, "Creator: Tool:"):
			lines[i] = "Creator: Tool: knoxctl"
		case strings.HasPrefix(line, "Creator: Organization:"):
			lines[i] = "Creator: Organization: AccuKnox"
		default:
			lines[i] = "Creator: Tool: knoxctl"
		}
	}
	return []byte(strings.Join(lines, "\n"))
}
