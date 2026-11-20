// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Authors of KubeArmor

package ui

import (
	"strings"
	"testing"
)

func TestSanitizeFilenamePart(t *testing.T) {
	cases := []struct{ in, want string }{
		{"KubeArmor", "KubeArmor"},
		{"my app:v2/linux", "my-app-v2-linux"}, // spaces, colon, slash → '-'
		{"a < b > c | d", "a-b-c-d"},           // Windows-illegal chars
		{"  --trim--  ", "trim"},               // trimmed separators
		{"a***b", "a-b"},                        // collapsed dashes
		{"", ""},                                // empty stays empty
		{"CON", "_CON"},                         // Windows reserved device name
		{"nul", "_nul"},                         // reserved, case-insensitive
		{"com3", "_com3"},                       // reserved COMn
		{"report.cdx", "report.cdx"},            // dots preserved
	}
	for _, c := range cases {
		if got := sanitizeFilenamePart(c.in); got != c.want {
			t.Errorf("sanitizeFilenamePart(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSanitizeFilenamePartLengthCap(t *testing.T) {
	got := sanitizeFilenamePart(strings.Repeat("a", 200))
	if len(got) > 80 {
		t.Errorf("length cap not applied: got %d chars", len(got))
	}
}

func TestBomUploadFilenameFromComponent(t *testing.T) {
	doc := []byte(`{"metadata":{"component":{"name":"KubeArmor"}}}`)
	got := bomUploadFilename("sbom", doc, "")
	// component-type-<YYYYMMDD-HHMMSS>.json
	if !strings.HasPrefix(got, "KubeArmor-sbom-") || !strings.HasSuffix(got, ".json") {
		t.Errorf("unexpected filename: %q", got)
	}
	if strings.ContainsAny(got, `:/\ *?"<>|`) {
		t.Errorf("filename contains a cross-platform-unsafe character: %q", got)
	}
}

func TestBomUploadFilenameFallbackNoComponent(t *testing.T) {
	// No metadata.component.name → "<type>-<stamp>.json", no leading dash.
	got := bomUploadFilename("cbom", []byte(`{"bomFormat":"CycloneDX"}`), "")
	if !strings.HasPrefix(got, "cbom-") || !strings.HasSuffix(got, ".json") {
		t.Errorf("fallback filename wrong: %q", got)
	}
	if strings.HasPrefix(got, "-") {
		t.Errorf("fallback should not start with a dash: %q", got)
	}
}

func TestBomUploadFilenameClientNameWins(t *testing.T) {
	// A client-proposed name is used (re-sanitised), overriding the derived name.
	got := bomUploadFilename("sbom", []byte(`{"metadata":{"component":{"name":"Other"}}}`),
		"KubeArmor-sbom-20261119-101500.json")
	if got != "KubeArmor-sbom-20261119-101500.json" {
		t.Errorf("client name should be used, got %q", got)
	}
}

func TestSanitizeUploadName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"KubeArmor-sbom-20261119-101500.json", "KubeArmor-sbom-20261119-101500.json"},
		{"/etc/passwd", "passwd.json"},         // path stripped, extension forced
		{"../../evil", "evil.json"},            // traversal stripped
		{"report", "report.json"},              // extension added
		{"", ""},                                // empty → empty (caller derives)
	}
	for _, c := range cases {
		if got := sanitizeUploadName(c.in); got != c.want {
			t.Errorf("sanitizeUploadName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
