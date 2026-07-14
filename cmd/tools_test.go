package cmd

import (
	"reflect"
	"testing"
)

func TestEnrichPkgscanCycloneDXArgs(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		args     []string
		want     []string
	}{
		{
			name:     "cyclonedx short output flag",
			toolName: "pkgscan",
			args:     []string{"scan", ".", "-o", "cyclonedx-json=result.json", "--quiet"},
			want:     []string{"scan", ".", "-o", "cyclonedx-json=result.json", "--quiet", "--enrich", "all"},
		},
		{
			name:     "cyclonedx long output flag",
			toolName: "pkgscan",
			args:     []string{"scan", ".", "--output=cyclonedx-json=result.json"},
			want:     []string{"scan", ".", "--output=cyclonedx-json=result.json", "--enrich", "all"},
		},
		{
			name:     "explicit enrichment is preserved",
			toolName: "pkgscan",
			args:     []string{"scan", ".", "-o", "cyclonedx-json=result.json", "--enrich", "golang"},
			want:     []string{"scan", ".", "-o", "cyclonedx-json=result.json", "--enrich", "golang"},
		},
		{
			name:     "explicit equals enrichment is preserved",
			toolName: "pkgscan",
			args:     []string{"scan", ".", "-o=cyclonedx-json=result.json", "--enrich=javascript"},
			want:     []string{"scan", ".", "-o=cyclonedx-json=result.json", "--enrich=javascript"},
		},
		{
			name:     "other pkgscan format is unchanged",
			toolName: "pkgscan",
			args:     []string{"scan", ".", "-o", "spdx-json=result.json"},
			want:     []string{"scan", ".", "-o", "spdx-json=result.json"},
		},
		{
			name:     "other pkgscan command is unchanged",
			toolName: "pkgscan",
			args:     []string{"version"},
			want:     []string{"version"},
		},
		{
			name:     "other tool is unchanged",
			toolName: "imgscan",
			args:     []string{"scan", ".", "-o", "cyclonedx-json=result.json"},
			want:     []string{"scan", ".", "-o", "cyclonedx-json=result.json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := append([]string(nil), tt.args...)
			got := enrichPkgscanCycloneDXArgs(tt.toolName, tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("enrichPkgscanCycloneDXArgs() = %#v, want %#v", got, tt.want)
			}
			if !reflect.DeepEqual(tt.args, original) {
				t.Fatalf("input args mutated: got %#v, want %#v", tt.args, original)
			}
		})
	}
}
