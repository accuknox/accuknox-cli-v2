// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Authors of KubeArmor

package cbom

import (
	"testing"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

// ----- BOM construction tests -----

func TestNewBOM_Source(t *testing.T) {
	comps := []cdx.Component{
		{
			Type: cdx.ComponentTypeCryptographicAsset,
			Name: "AES",
			CryptoProperties: &cdx.CryptoProperties{
				AssetType: cdx.CryptoAssetTypeAlgorithm,
			},
		},
	}
	opts := &Options{
		Name:        "my-project",
		Group:       "com.example",
		Version:     "1.0.0",
		Description: "Test project",
		License:     "Apache-2.0",
	}
	bom := newBOM(comps, "my-project", "", opts)
	if bom.SerialNumber == "" {
		t.Error("SerialNumber should be set")
	}
	if bom.Metadata == nil || bom.Metadata.Timestamp == "" {
		t.Error("Metadata.Timestamp should be set")
	}
	if bom.Metadata.Lifecycles == nil || len(*bom.Metadata.Lifecycles) == 0 {
		t.Error("Metadata.Lifecycles should be set")
	} else if (*bom.Metadata.Lifecycles)[0].Phase != cdx.LifecyclePhaseBuild {
		t.Errorf("expected lifecycle phase 'build', got %s", (*bom.Metadata.Lifecycles)[0].Phase)
	}
	c := bom.Metadata.Component
	if c == nil {
		t.Fatal("Metadata.Component should be set")
	}
	if c.Name != "my-project" {
		t.Errorf("component name = %q, want %q", c.Name, "my-project")
	}
	if c.Group != "com.example" {
		t.Errorf("component group = %q, want %q", c.Group, "com.example")
	}
	if c.Version != "1.0.0" {
		t.Errorf("component version = %q, want %q", c.Version, "1.0.0")
	}
	if c.Description != "Test project" {
		t.Errorf("component description = %q, want %q", c.Description, "Test project")
	}
	if c.PackageURL == "" {
		t.Error("PackageURL should be set when group and version are provided")
	}
	if c.Licenses == nil {
		t.Error("Licenses should be set")
	}
	if bom.Components == nil || len(*bom.Components) != 1 {
		t.Error("expected 1 component")
	}
}

func TestNewBOM_Image(t *testing.T) {
	bom := newBOM(nil, "", "nginx:latest", &Options{})
	if bom.Metadata.Component == nil || bom.Metadata.Component.Name != "nginx:latest" {
		t.Error("Metadata.Component should reflect image name")
	}
	if bom.Metadata.Component.Type != cdx.ComponentTypeContainer {
		t.Errorf("expected container type, got %s", bom.Metadata.Component.Type)
	}
	if bom.Metadata.Lifecycles == nil {
		t.Error("Metadata.Lifecycles should be set")
	}
}

func TestBOMRef_MetadataComponent(t *testing.T) {
	cases := []struct {
		name    string
		opts    Options
		source  string
		wantRef string
	}{
		{
			name:    "purl when group and version present",
			opts:    Options{Group: "com.example", Version: "1.0.0"},
			source:  "myapp",
			wantRef: "pkg:generic/com.example/myapp@1.0.0",
		},
		{
			name:    "purl when only version present (no group)",
			opts:    Options{Version: "2.3.4"},
			source:  "myapp",
			wantRef: "pkg:generic/myapp@2.3.4",
		},
		{
			name:    "name only when no version",
			opts:    Options{},
			source:  "myapp",
			wantRef: "myapp",
		},
		{
			name:    "image name only",
			opts:    Options{},
			source:  "",
			wantRef: "nginx:latest",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			image := ""
			if tc.source == "" {
				image = "nginx:latest"
			}
			bom := newBOM(nil, tc.source, image, &tc.opts)
			if bom.Metadata == nil || bom.Metadata.Component == nil {
				t.Fatal("metadata.component must be set")
			}
			if bom.Metadata.Component.BOMRef == "" {
				t.Error("bom-ref must not be empty")
			}
			if bom.Metadata.Component.BOMRef != tc.wantRef {
				t.Errorf("bom-ref = %q, want %q", bom.Metadata.Component.BOMRef, tc.wantRef)
			}
		})
	}
}

func TestComponentCount(t *testing.T) {
	bom := &cdx.BOM{
		Components: &[]cdx.Component{
			{Type: cdx.ComponentTypeCryptographicAsset, Name: "AES"},
			{Type: cdx.ComponentTypeCryptographicAsset, Name: "RSA"},
			{Type: cdx.ComponentTypeLibrary, Name: "some-lib"},
		},
	}
	if got := ComponentCount(bom); got != 2 {
		t.Errorf("ComponentCount = %d, want 2", got)
	}
}

func TestComponentCount_NilComponents(t *testing.T) {
	bom := &cdx.BOM{}
	if got := ComponentCount(bom); got != 0 {
		t.Errorf("ComponentCount = %d, want 0", got)
	}
}

func TestEnforceLicenses(t *testing.T) {
	known := cdx.Licenses{cdx.LicenseChoice{License: &cdx.License{ID: "Apache-2.0"}}}
	bom := &cdx.BOM{
		Components: &[]cdx.Component{
			// crypto asset with no license → should get "unknown"
			{Type: cdx.ComponentTypeCryptographicAsset, Name: "AES"},
			// crypto asset with existing license → must not be overwritten
			{Type: cdx.ComponentTypeCryptographicAsset, Name: "RSA", Licenses: &known},
			// non-crypto component with no license → must not be touched
			{Type: cdx.ComponentTypeLibrary, Name: "some-lib"},
		},
	}
	enforceLicenses(bom)

	comps := *bom.Components
	// AES: should now have "unknown"
	if comps[0].Licenses == nil || len(*comps[0].Licenses) == 0 {
		t.Fatal("AES: expected Licenses to be set")
	}
	if id := (*comps[0].Licenses)[0].License.ID; id != "unknown" {
		t.Errorf("AES license = %q, want %q", id, "unknown")
	}
	// RSA: existing license must be preserved
	if id := (*comps[1].Licenses)[0].License.ID; id != "Apache-2.0" {
		t.Errorf("RSA license = %q, want %q", id, "Apache-2.0")
	}
	// some-lib: no license should have been added
	if comps[2].Licenses != nil {
		t.Error("some-lib: non-crypto component should not have a license added")
	}
}
