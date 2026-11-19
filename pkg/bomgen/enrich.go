// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Authors of KubeArmor

package bomgen

import (
	"encoding/json"
	"time"
)

// EnrichParams carries the CERT-In SBOM metadata values that a user supplies
// through the UI to fill fields the scanner engine does not emit natively.
//
// Values fall into three groups:
//   - Document-level metadata (metadata.authors / supplier / timestamp).
//   - Native primary-component fields the scanner leaves thin at
//     metadata.component (name, version, description, license, author/origin,
//     purl "unique identifier", and hashes pulled up from components[]).
//   - CERT-In "Technical Guidelines" elements with no native CycloneDX field,
//     written as metadata.component.properties[] (namespace certin:):
//     Release/EOL date, Criticality, Executable/Archive/Structured (+ the file
//     lists that back a "Yes"), Patch Status, Usage Restrictions and Notes.
type EnrichParams struct {
	// Document metadata.
	AuthorName   string `json:"authorName"`
	AuthorEmail  string `json:"authorEmail"`
	SupplierName string `json:"supplierName"`
	SupplierURL  string `json:"supplierUrl"`
	// Timestamp sets metadata.timestamp. The literal "now" (or TimestampNow)
	// records the current time in RFC 3339.
	Timestamp    string `json:"timestamp"`
	TimestampNow bool   `json:"timestampNow"`

	// Native primary-component fields (metadata.component.*). The scanner emits
	// a thin metadata.component for a source/product scan, so CERT-In's
	// product-level data fields are supplied here.
	ComponentName        string `json:"componentName"`        // metadata.component.name (path basename)
	ComponentVersion     string `json:"componentVersion"`     // metadata.component.version
	ComponentDescription string `json:"componentDescription"` // metadata.component.description
	ComponentLicense     string `json:"componentLicense"`     // metadata.component.licenses[]
	ComponentOrigin      string `json:"componentOrigin"`      // metadata.component.author (CERT-In "Origin")
	UniqueIdentifier     string `json:"uniqueIdentifier"`     // metadata.component.purl (pkg:supplier/Org/Name@Ver)
	// Checksums/Hashes (CERT-In). A supplied HashValue (with HashAlg, default
	// SHA-256) writes metadata.component.hashes[] directly. When no value is
	// given, PullHashes copies the primary artifact's hashes[] from components[]
	// if a matching entry exists.
	HashAlg    string `json:"hashAlg"`
	HashValue  string `json:"hashValue"`
	PullHashes bool   `json:"pullHashes"`

	// Primary-component CERT-In properties (metadata.component.properties[]).
	Criticality string `json:"criticality"`
	ReleaseDate string `json:"releaseDate"`
	EOLDate     string `json:"eolDate"`
	// EOLNotApplicable records certin:eol_date = "N/A" so a missing EOL is
	// scored as deliberately addressed rather than absent.
	EOLNotApplicable  bool   `json:"eolNotApplicable"`
	Executable        string `json:"executable"` // Yes / No
	ExecutableFiles   string `json:"executableFiles"`
	Archive           string `json:"archive"` // Yes / No
	ArchiveFiles      string `json:"archiveFiles"`
	Structured        string `json:"structured"` // Yes / No
	StructuredFiles   string `json:"structuredFiles"`
	PatchStatus       string `json:"patchStatus"`
	UsageRestrictions string `json:"usageRestrictions"`
	Notes             string `json:"notes"`
}

// propNamespace prefixes every injected property key so CERT-In enrichment is
// unambiguous and easy to strip or re-apply.
const propNamespace = "certin:"

// HasAny reports whether any enrichment value is set. When false the document is
// left byte-for-byte unchanged so an empty form never rewrites the SBOM.
// PullHashes is intentionally excluded: it only acts when other fields identify
// the primary component, so on its own it must not trigger a rewrite.
func (p EnrichParams) HasAny() bool {
	return p.AuthorName != "" || p.AuthorEmail != "" ||
		p.SupplierName != "" || p.SupplierURL != "" ||
		p.Timestamp != "" || p.TimestampNow ||
		p.ComponentName != "" || p.ComponentVersion != "" ||
		p.ComponentDescription != "" || p.ComponentLicense != "" ||
		p.ComponentOrigin != "" || p.UniqueIdentifier != "" ||
		p.HashValue != "" ||
		p.Criticality != "" || p.ReleaseDate != "" ||
		p.EOLDate != "" || p.EOLNotApplicable ||
		p.Executable != "" || p.ExecutableFiles != "" ||
		p.Archive != "" || p.ArchiveFiles != "" ||
		p.Structured != "" || p.StructuredFiles != "" ||
		p.PatchStatus != "" || p.UsageRestrictions != "" || p.Notes != ""
}

// effectiveEOL resolves the EOL property value: an explicit "No EOL" toggle wins
// and records "N/A"; otherwise the supplied date (or empty to skip).
func (p EnrichParams) effectiveEOL() string {
	if p.EOLNotApplicable {
		return "N/A"
	}
	return p.EOLDate
}

// componentProps returns the namespaced metadata.component properties to inject,
// in a stable CERT-In order, skipping empty values.
func (p EnrichParams) componentProps() []map[string]interface{} {
	fields := []struct{ key, val string }{
		{"release_date", p.ReleaseDate},
		{"eol_date", p.effectiveEOL()},
		{"criticality", p.Criticality},
		{"executable", p.Executable},
		{"executable_files", p.ExecutableFiles},
		{"archive", p.Archive},
		{"archive_files", p.ArchiveFiles},
		{"structured", p.Structured},
		{"structured_files", p.StructuredFiles},
		{"patch_status", p.PatchStatus},
		{"usage_restrictions", p.UsageRestrictions},
		{"notes", p.Notes},
	}
	var out []map[string]interface{}
	for _, f := range fields {
		if f.val == "" {
			continue
		}
		out = append(out, map[string]interface{}{
			"name":  propNamespace + f.key,
			"value": f.val,
		})
	}
	return out
}

// EnrichCycloneDX injects the supplied CERT-In metadata into a CycloneDX JSON
// SBOM and returns the re-serialised document. It is a no-op (returning the
// input unchanged) when no values are set, when the document is not CycloneDX
// JSON, or when it cannot be parsed, so it is always safe to call in the BOM
// pipeline. Injected properties are merged by name: an existing certin:* value
// is overwritten rather than duplicated.
func EnrichCycloneDX(doc []byte, p EnrichParams) []byte {
	if !p.HasAny() {
		return doc
	}
	var root map[string]interface{}
	if err := json.Unmarshal(doc, &root); err != nil {
		return doc
	}
	if bf, _ := root["bomFormat"].(string); bf != "CycloneDX" {
		return doc // SPDX / syft-json etc. — leave untouched.
	}

	meta, _ := root["metadata"].(map[string]interface{})
	if meta == nil {
		meta = map[string]interface{}{}
		root["metadata"] = meta
	}

	// metadata.timestamp
	if p.TimestampNow || p.Timestamp == "now" {
		meta["timestamp"] = time.Now().UTC().Format(time.RFC3339)
	} else if p.Timestamp != "" {
		meta["timestamp"] = p.Timestamp
	}

	// metadata.authors[] — append the SBOM author (organizationalContact).
	if p.AuthorName != "" || p.AuthorEmail != "" {
		author := map[string]interface{}{}
		if p.AuthorName != "" {
			author["name"] = p.AuthorName
		}
		if p.AuthorEmail != "" {
			author["email"] = p.AuthorEmail
		}
		authors, _ := meta["authors"].([]interface{})
		meta["authors"] = append(authors, author)
	}

	// metadata.supplier (organizationalEntity: name + url[]).
	if p.SupplierName != "" || p.SupplierURL != "" {
		supplier, _ := meta["supplier"].(map[string]interface{})
		if supplier == nil {
			supplier = map[string]interface{}{}
		}
		if p.SupplierName != "" {
			supplier["name"] = p.SupplierName
		}
		if p.SupplierURL != "" {
			supplier["url"] = []interface{}{p.SupplierURL}
		}
		meta["supplier"] = supplier
	}

	// metadata.component — native product-level CERT-In fields + certin:* props.
	comp, _ := meta["component"].(map[string]interface{})
	if comp == nil {
		comp = map[string]interface{}{"type": "application", "name": "root-component"}
		meta["component"] = comp
	}
	if p.ComponentName != "" {
		comp["name"] = p.ComponentName
	}
	if p.ComponentVersion != "" {
		comp["version"] = p.ComponentVersion
	}
	if p.ComponentDescription != "" {
		comp["description"] = p.ComponentDescription
	}
	if p.ComponentOrigin != "" {
		comp["author"] = p.ComponentOrigin // CERT-In "Origin" → component.author
	}
	if p.ComponentLicense != "" {
		comp["licenses"] = []interface{}{
			map[string]interface{}{"license": map[string]interface{}{"name": p.ComponentLicense}},
		}
	}
	if p.UniqueIdentifier != "" {
		comp["purl"] = p.UniqueIdentifier
	}
	// Checksums/Hashes: a supplied value wins; otherwise lift the primary
	// artifact's hashes from components[] when PullHashes is set.
	if p.HashValue != "" {
		alg := p.HashAlg
		if alg == "" {
			alg = "SHA-256"
		}
		comp["hashes"] = []interface{}{
			map[string]interface{}{"alg": alg, "content": p.HashValue},
		}
	} else if p.PullHashes {
		if h := primaryComponentHashes(root, comp); len(h) > 0 {
			comp["hashes"] = h
		}
	}
	if props := p.componentProps(); len(props) > 0 {
		existing, _ := comp["properties"].([]interface{})
		comp["properties"] = mergeProperties(existing, props)
	}

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return doc
	}
	return out
}

// primaryComponentHashes finds the components[] entry that corresponds to the
// primary metadata.component (matched by purl, then bom-ref, then name+version)
// and returns its hashes[]. Returns nil when there is no match or no hashes, so
// the caller leaves metadata.component.hashes absent (a real CERT-In gap) rather
// than inventing one.
func primaryComponentHashes(root, comp map[string]interface{}) []interface{} {
	comps, _ := root["components"].([]interface{})
	if len(comps) == 0 {
		return nil
	}
	purl, _ := comp["purl"].(string)
	bomRef, _ := comp["bom-ref"].(string)
	name, _ := comp["name"].(string)
	version, _ := comp["version"].(string)

	for _, c := range comps {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		match := false
		if purl != "" {
			if cp, _ := cm["purl"].(string); cp == purl {
				match = true
			}
		}
		if !match && bomRef != "" {
			if cr, _ := cm["bom-ref"].(string); cr == bomRef {
				match = true
			}
		}
		if !match && name != "" {
			if cn, _ := cm["name"].(string); cn == name {
				cv, _ := cm["version"].(string)
				if version == "" || cv == version {
					match = true
				}
			}
		}
		if match {
			if h, ok := cm["hashes"].([]interface{}); ok && len(h) > 0 {
				return h
			}
		}
	}
	return nil
}

// mergeProperties appends newProps to existing, overwriting any existing entry
// whose "name" matches (so re-running enrichment updates rather than duplicates
// certin:* keys).
func mergeProperties(existing []interface{}, newProps []map[string]interface{}) []interface{} {
	overwrite := make(map[string]string, len(newProps))
	for _, np := range newProps {
		overwrite[np["name"].(string)] = np["value"].(string)
	}
	out := make([]interface{}, 0, len(existing)+len(newProps))
	for _, e := range existing {
		em, ok := e.(map[string]interface{})
		if ok {
			if name, _ := em["name"].(string); name != "" {
				if _, replacing := overwrite[name]; replacing {
					continue // drop; the new value is appended below
				}
			}
		}
		out = append(out, e)
	}
	for _, np := range newProps {
		out = append(out, np)
	}
	return out
}
