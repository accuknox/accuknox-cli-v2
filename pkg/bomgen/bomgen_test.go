// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Authors of KubeArmor

package bomgen

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSBOMArgsDepth(t *testing.T) {
	// Depth on + cyclonedx -> filesystem (imgscan fs) scan.
	a := SBOMArgs(SBOMParams{Source: "/proj", Format: "cyclonedx-json", Depth: true}, "/out.json")
	got := strings.Join(a, " ")
	if !strings.HasPrefix(got, "imgscan fs --format cyclonedx") || !strings.Contains(got, "/proj") {
		t.Errorf("depth cyclonedx args = %v", a)
	}

	// Depth on + spdx-json -> spdx-json deep scan.
	a = SBOMArgs(SBOMParams{Source: "/proj", Format: "spdx-json", Depth: true}, "/out.json")
	if !strings.Contains(strings.Join(a, " "), "--format spdx-json") {
		t.Errorf("depth spdx args = %v", a)
	}

	// Depth on but format has no deep equivalent (syft-json) -> standard scan.
	a = SBOMArgs(SBOMParams{Source: "/proj", Format: "syft-json", Depth: true}, "/out.json")
	if a[0] != "pkgscan" {
		t.Errorf("syft-json with depth should fall back to package scan, got %v", a)
	}

	// Depth off -> standard package scan regardless of format.
	a = SBOMArgs(SBOMParams{Source: "/proj", Format: "cyclonedx-json", Depth: false}, "/out.json")
	if a[0] != "pkgscan" {
		t.Errorf("depth off should use package scan, got %v", a)
	}
}

func TestUsesDepth(t *testing.T) {
	if !(SBOMParams{Format: "cyclonedx-json", Depth: true}).UsesDepth() {
		t.Error("cyclonedx + depth should use depth")
	}
	if (SBOMParams{Format: "syft-json", Depth: true}).UsesDepth() {
		t.Error("syft-json has no depth equivalent")
	}
	if (SBOMParams{Format: "cyclonedx-json", Depth: false}).UsesDepth() {
		t.Error("depth off must not use depth")
	}
}

func TestStripDepthBrandingCycloneDX(t *testing.T) {
	// Tool metadata + a trivy-namespaced property + a component whose purl
	// legitimately contains the engine's org/name (must be preserved).
	in := []byte(`{
      "bomFormat":"CycloneDX","specVersion":"1.6",
      "metadata":{"tools":{"components":[{"type":"application","group":"aquasecurity","name":"trivy","version":"0.69.3"}]}},
      "components":[
        {"type":"library","name":"trivy","version":"0.69.3",
         "purl":"pkg:golang/github.com/aquasecurity/trivy@0.69.3",
         "properties":[{"name":"aquasecurity:trivy:PkgType","value":"gomod"}]}
      ]
    }`)
	outBytes := StripDepthBranding(in, "cyclonedx-json")
	out := string(outBytes)

	// Tool metadata must be rebranded (checked structurally so the assertion
	// isn't confused by the package that is legitimately named "trivy").
	var doc map[string]interface{}
	if err := json.Unmarshal(outBytes, &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	tool := doc["metadata"].(map[string]interface{})["tools"].(map[string]interface{})["components"].([]interface{})[0].(map[string]interface{})
	if tool["name"] != "knoxctl" {
		t.Errorf("tool name should be rebranded to knoxctl, got %v", tool["name"])
	}
	if tool["group"] != "accuknox" {
		t.Errorf("tool group should be rebranded to accuknox, got %v", tool["group"])
	}

	if strings.Contains(out, "aquasecurity:trivy:") {
		t.Error("trivy-namespaced property keys should be rebranded")
	}
	if !strings.Contains(out, "knoxctl:PkgType") {
		t.Error("property key should become knoxctl-namespaced")
	}
	// A real package's purl/name containing the engine name must be left intact.
	if !strings.Contains(out, "pkg:golang/github.com/aquasecurity/trivy@0.69.3") {
		t.Error("a real package purl containing the engine name must NOT be altered")
	}
}
