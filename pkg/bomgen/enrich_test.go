// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Authors of KubeArmor

package bomgen

import (
	"encoding/json"
	"testing"
)

const baseCDX = `{"bomFormat":"CycloneDX","specVersion":"1.6",
  "metadata":{"timestamp":"2020-01-01T00:00:00Z","component":{"type":"application","name":"app"}},
  "components":[{"type":"library","name":"left-pad","version":"1.0.0"}]}`

func parse(t *testing.T, b []byte) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	return m
}

func TestEnrichNoOpWhenEmpty(t *testing.T) {
	out := EnrichCycloneDX([]byte(baseCDX), EnrichParams{})
	if string(out) != baseCDX {
		t.Error("empty params should return the document byte-for-byte unchanged")
	}
}

func TestEnrichNoOpForNonCycloneDX(t *testing.T) {
	spdx := `{"spdxVersion":"SPDX-2.3","name":"x"}`
	out := EnrichCycloneDX([]byte(spdx), EnrichParams{Criticality: "High"})
	if string(out) != spdx {
		t.Error("non-CycloneDX document should be left untouched")
	}
}

func TestEnrichInjectsMetadataAndProperties(t *testing.T) {
	p := EnrichParams{
		AuthorName:   "Security Team",
		AuthorEmail:  "sec@example.com",
		SupplierName: "ACME Corp",
		SupplierURL:  "https://acme.example.com",
		Criticality:  "High",
		EOLDate:      "2027-01-10",
		Executable:   "Yes",
	}
	m := parse(t, EnrichCycloneDX([]byte(baseCDX), p))
	meta := m["metadata"].(map[string]interface{})

	authors := meta["authors"].([]interface{})
	if len(authors) != 1 || authors[0].(map[string]interface{})["name"] != "Security Team" {
		t.Errorf("author not injected: %v", authors)
	}
	sup := meta["supplier"].(map[string]interface{})
	if sup["name"] != "ACME Corp" {
		t.Errorf("supplier name not set: %v", sup)
	}
	if urls, ok := sup["url"].([]interface{}); !ok || urls[0] != "https://acme.example.com" {
		t.Errorf("supplier url not set as array: %v", sup["url"])
	}

	props := meta["component"].(map[string]interface{})["properties"].([]interface{})
	got := map[string]string{}
	for _, pr := range props {
		pm := pr.(map[string]interface{})
		got[pm["name"].(string)] = pm["value"].(string)
	}
	for k, want := range map[string]string{
		"certin:criticality": "High",
		"certin:eol_date":    "2027-01-10",
		"certin:executable":  "Yes",
	} {
		if got[k] != want {
			t.Errorf("property %s = %q, want %q", k, got[k], want)
		}
	}
	// Blank fields must not be injected.
	if _, exists := got["certin:release_date"]; exists {
		t.Error("blank release_date should not be injected")
	}
}

func TestEnrichMergeOverwritesExisting(t *testing.T) {
	withProp := `{"bomFormat":"CycloneDX","specVersion":"1.6","metadata":{"component":{"type":"application","name":"app","properties":[{"name":"certin:criticality","value":"Low"},{"name":"other:keep","value":"x"}]}}}`
	m := parse(t, EnrichCycloneDX([]byte(withProp), EnrichParams{Criticality: "Critical"}))
	props := m["metadata"].(map[string]interface{})["component"].(map[string]interface{})["properties"].([]interface{})
	var crit []string
	keep := false
	for _, pr := range props {
		pm := pr.(map[string]interface{})
		switch pm["name"] {
		case "certin:criticality":
			crit = append(crit, pm["value"].(string))
		case "other:keep":
			keep = true
		}
	}
	if len(crit) != 1 || crit[0] != "Critical" {
		t.Errorf("criticality should be overwritten to single Critical, got %v", crit)
	}
	if !keep {
		t.Error("non-certin property should be preserved")
	}
}

func TestEnrichTimestampNow(t *testing.T) {
	m := parse(t, EnrichCycloneDX([]byte(baseCDX), EnrichParams{TimestampNow: true}))
	ts := m["metadata"].(map[string]interface{})["timestamp"].(string)
	if ts == "2020-01-01T00:00:00Z" || ts == "" {
		t.Errorf("timestampNow should overwrite with current time, got %q", ts)
	}
}

func TestEnrichNativeComponentFields(t *testing.T) {
	p := EnrichParams{
		ComponentName:        "lobe-chat-main",
		ComponentVersion:     "1.4.0",
		ComponentDescription: "Chat UI",
		ComponentLicense:     "Apache-2.0",
		ComponentOrigin:      "Lobe Hub",
		UniqueIdentifier:     "pkg:supplier/LobeHub/lobe-chat-main@1.4.0?os=linux#web",
	}
	comp := parse(t, EnrichCycloneDX([]byte(baseCDX), p))["metadata"].(map[string]interface{})["component"].(map[string]interface{})
	if comp["name"] != "lobe-chat-main" || comp["version"] != "1.4.0" {
		t.Errorf("name/version not set: %v", comp)
	}
	if comp["description"] != "Chat UI" {
		t.Errorf("description not set: %v", comp["description"])
	}
	if comp["author"] != "Lobe Hub" { // CERT-In Origin → component.author
		t.Errorf("origin should map to author: %v", comp["author"])
	}
	if comp["purl"] != "pkg:supplier/LobeHub/lobe-chat-main@1.4.0?os=linux#web" {
		t.Errorf("purl not set: %v", comp["purl"])
	}
	lic := comp["licenses"].([]interface{})[0].(map[string]interface{})["license"].(map[string]interface{})
	if lic["name"] != "Apache-2.0" {
		t.Errorf("license not set: %v", lic)
	}
}

func TestEnrichPullHashes(t *testing.T) {
	// Primary component "app" also appears in components[] carrying a hash.
	doc := `{"bomFormat":"CycloneDX","specVersion":"1.6",
	  "metadata":{"component":{"type":"application","name":"app","version":"1.0.0"}},
	  "components":[{"type":"application","name":"app","version":"1.0.0",
	    "hashes":[{"alg":"SHA-256","content":"abc123"}]}]}`
	comp := parse(t, EnrichCycloneDX([]byte(doc), EnrichParams{ComponentVersion: "1.0.0", PullHashes: true}))["metadata"].(map[string]interface{})["component"].(map[string]interface{})
	h, ok := comp["hashes"].([]interface{})
	if !ok || len(h) != 1 || h[0].(map[string]interface{})["content"] != "abc123" {
		t.Errorf("hash not pulled up from components[]: %v", comp["hashes"])
	}
}

func TestEnrichPullHashesNoMatch(t *testing.T) {
	// No components[] match → hashes stays absent (a real gap, not invented).
	comp := parse(t, EnrichCycloneDX([]byte(baseCDX), EnrichParams{ComponentName: "other", PullHashes: true}))["metadata"].(map[string]interface{})["component"].(map[string]interface{})
	if _, exists := comp["hashes"]; exists {
		t.Error("hashes should be absent when no components[] entry matches")
	}
}

func TestEnrichUserHashWins(t *testing.T) {
	// A components[] entry has a hash, but the user supplies one → user value wins.
	doc := `{"bomFormat":"CycloneDX","specVersion":"1.6",
	  "metadata":{"component":{"type":"application","name":"app","version":"1.0.0"}},
	  "components":[{"name":"app","version":"1.0.0","hashes":[{"alg":"SHA-256","content":"frompkg"}]}]}`
	comp := parse(t, EnrichCycloneDX([]byte(doc), EnrichParams{
		ComponentVersion: "1.0.0", HashAlg: "SHA-512", HashValue: "deadbeef", PullHashes: true,
	}))["metadata"].(map[string]interface{})["component"].(map[string]interface{})
	h := comp["hashes"].([]interface{})[0].(map[string]interface{})
	if h["alg"] != "SHA-512" || h["content"] != "deadbeef" {
		t.Errorf("user-provided hash should win, got %v", h)
	}
}

func TestEnrichHashDefaultAlg(t *testing.T) {
	comp := parse(t, EnrichCycloneDX([]byte(baseCDX), EnrichParams{HashValue: "abc"}))["metadata"].(map[string]interface{})["component"].(map[string]interface{})
	h := comp["hashes"].([]interface{})[0].(map[string]interface{})
	if h["alg"] != "SHA-256" {
		t.Errorf("default alg should be SHA-256, got %v", h["alg"])
	}
}

func TestEnrichEolNotApplicable(t *testing.T) {
	got := propMap(t, EnrichCycloneDX([]byte(baseCDX), EnrichParams{EOLNotApplicable: true}))
	if got["certin:eol_date"] != "N/A" {
		t.Errorf("EOL N/A should record certin:eol_date=N/A, got %q", got["certin:eol_date"])
	}
	// Explicit toggle wins over a supplied date.
	got = propMap(t, EnrichCycloneDX([]byte(baseCDX), EnrichParams{EOLDate: "2027-01-01", EOLNotApplicable: true}))
	if got["certin:eol_date"] != "N/A" {
		t.Errorf("N/A toggle should override the date, got %q", got["certin:eol_date"])
	}
}

func TestEnrichExecutableFiles(t *testing.T) {
	got := propMap(t, EnrichCycloneDX([]byte(baseCDX), EnrichParams{
		Executable: "Yes", ExecutableFiles: "catalina.sh, startup.bat",
	}))
	if got["certin:executable"] != "Yes" || got["certin:executable_files"] != "catalina.sh, startup.bat" {
		t.Errorf("executable + files not injected: %v", got)
	}
}

// propMap collects metadata.component.properties[] into a name→value map.
func propMap(t *testing.T, b []byte) map[string]string {
	t.Helper()
	out := map[string]string{}
	comp, _ := parse(t, b)["metadata"].(map[string]interface{})["component"].(map[string]interface{})
	props, _ := comp["properties"].([]interface{})
	for _, pr := range props {
		pm := pr.(map[string]interface{})
		out[pm["name"].(string)] = pm["value"].(string)
	}
	return out
}

func TestCycloneDXValidateArgs(t *testing.T) {
	got := CycloneDXValidateArgs("", "/tmp/out.json")
	want := []string{"validate", "--input-file", "/tmp/out.json", "--input-version", "v1_6", "--fail-on-errors"}
	if len(got) != len(want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	// Explicit version must be converted to the underscore flag form.
	if v := CycloneDXValidateArgs("1.6", "x")[4]; v != "v1_6" {
		t.Errorf("input-version flag = %q, want v1_6", v)
	}
}
