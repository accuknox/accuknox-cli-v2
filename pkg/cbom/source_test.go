// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Authors of KubeArmor

package cbom

import (
	"os"
	"path/filepath"
	"testing"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

// ----- Go scanning tests -----

func TestScanSource_GoImports(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), `package main

import (
	"crypto/aes"
	"crypto/sha256"
	"fmt"
)

func main() { fmt.Println(aes.BlockSize, sha256.New()) }
`)

	comps, err := ScanSource(dir)
	if err != nil {
		t.Fatal(err)
	}

	names := compNames(comps)
	if !names["AES"] {
		t.Error("expected AES component")
	}
	if !names["SHA-256"] {
		t.Error("expected SHA-256 component")
	}
	if len(comps) != 2 {
		t.Errorf("expected 2 components, got %d", len(comps))
	}
}

func TestScanSource_GoSkipsVendor(t *testing.T) {
	dir := t.TempDir()
	vendor := filepath.Join(dir, "vendor")
	os.MkdirAll(vendor, 0o755)
	writeFile(t, filepath.Join(vendor, "crypto.go"), `package v
import "crypto/aes"
var _ = aes.BlockSize
`)
	comps, err := ScanSource(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(comps) != 0 {
		t.Errorf("expected 0 components from vendor, got %d", len(comps))
	}
}

// ----- Python scanning tests -----

func TestScanSource_PythonImports(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app.py"), `
import hashlib
import ssl
from cryptography.fernet import Fernet
from cryptography.hazmat.primitives.kdf.pbkdf2 import PBKDF2HMAC
import jwt
`)

	comps, err := ScanSource(dir)
	if err != nil {
		t.Fatal(err)
	}

	names := compNames(comps)
	if !names["hashlib"] {
		t.Error("expected hashlib component")
	}
	if !names["TLS"] {
		t.Error("expected TLS component (from ssl)")
	}
	if !names["Fernet"] {
		t.Error("expected Fernet component")
	}
	if !names["PBKDF2"] {
		t.Error("expected PBKDF2 component")
	}
	if !names["JWT"] {
		t.Error("expected JWT component")
	}
	if len(comps) != 5 {
		t.Errorf("expected 5 components, got %d: %v", len(comps), nameList(comps))
	}
}

func TestScanSource_PythonFromSubmodule(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "util.py"), `
from cryptography.hazmat.primitives.kdf.pbkdf2 import PBKDF2HMAC
from cryptography.hazmat.primitives.hashes import SHA256
from cryptography.hazmat.backends import default_backend
`)

	comps, err := ScanSource(dir)
	if err != nil {
		t.Fatal(err)
	}

	names := compNames(comps)
	if !names["PBKDF2"] {
		t.Error("expected PBKDF2 component")
	}
	if !names["cryptography-hashes"] {
		t.Error("expected cryptography-hashes component")
	}
}

func TestScanSource_PythonSkipsPycache(t *testing.T) {
	dir := t.TempDir()
	pycache := filepath.Join(dir, "__pycache__")
	os.MkdirAll(pycache, 0o755)
	writeFile(t, filepath.Join(pycache, "cached.py"), `import hashlib`)

	comps, err := ScanSource(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(comps) != 0 {
		t.Errorf("expected 0 components from __pycache__, got %d", len(comps))
	}
}

func TestScanSource_PythonDjangoCrypto(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "views.py"), `
from django.utils.crypto import constant_time_compare
from rest_framework_simplejwt.authentication import JWTAuthentication
`)

	comps, err := ScanSource(dir)
	if err != nil {
		t.Fatal(err)
	}

	names := compNames(comps)
	if !names["Django-crypto"] {
		t.Error("expected Django-crypto component")
	}
	if !names["SimpleJWT"] {
		t.Error("expected SimpleJWT component")
	}
}

func TestScanSource_PythonPyCryptodome(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "enc.py"), `
from Crypto.Cipher.AES import new as aes_new
from Crypto.PublicKey.RSA import generate
from Cryptodome.Hash.SHA256 import new as sha256_new
`)

	comps, err := ScanSource(dir)
	if err != nil {
		t.Fatal(err)
	}

	names := compNames(comps)
	if !names["PyCryptodome-Cipher"] {
		t.Error("expected PyCryptodome-Cipher component")
	}
	if !names["PyCryptodome-PublicKey"] {
		t.Error("expected PyCryptodome-PublicKey component")
	}
	if !names["PyCryptodome-Hash"] {
		t.Error("expected PyCryptodome-Hash component")
	}
}

func TestScanSource_PythonNonCrypto(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app.py"), `
import os
import json
from django.http import HttpResponse
`)

	comps, err := ScanSource(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(comps) != 0 {
		t.Errorf("expected 0 components for non-crypto imports, got %d", len(comps))
	}
}

// ----- Mixed language tests -----

func TestScanSource_MixedGoAndPython(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), `package main
import "crypto/rsa"
var _ = rsa.GenerateKey
`)
	writeFile(t, filepath.Join(dir, "script.py"), `
import hashlib
from cryptography.fernet import Fernet
`)

	comps, err := ScanSource(dir)
	if err != nil {
		t.Fatal(err)
	}

	names := compNames(comps)
	if !names["RSA"] {
		t.Error("expected RSA component from Go")
	}
	if !names["hashlib"] {
		t.Error("expected hashlib component from Python")
	}
	if !names["Fernet"] {
		t.Error("expected Fernet component from Python")
	}
}

func TestScanSource_EmptyDir(t *testing.T) {
	comps, err := ScanSource(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(comps) != 0 {
		t.Errorf("expected 0 components, got %d", len(comps))
	}
}

func TestScanSource_DeduplicatesAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.py"), `import hashlib`)
	writeFile(t, filepath.Join(dir, "b.py"), `import hashlib`)

	comps, err := ScanSource(dir)
	if err != nil {
		t.Fatal(err)
	}

	count := 0
	for _, c := range comps {
		if c.Name == "hashlib" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 hashlib component (deduplicated), got %d", count)
	}
	// But should have 2 evidence occurrences.
	for _, c := range comps {
		if c.Name == "hashlib" && c.Evidence != nil && c.Evidence.Occurrences != nil {
			if got := len(*c.Evidence.Occurrences); got != 2 {
				t.Errorf("expected 2 occurrences, got %d", got)
			}
		}
	}
}

func TestScanSource_EvidenceOccurrences(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app.py"), `import hashlib
import os
from cryptography.fernet import Fernet
`)

	comps, err := ScanSource(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range comps {
		if c.Evidence == nil || c.Evidence.Occurrences == nil {
			t.Errorf("component %q missing evidence occurrences", c.Name)
			continue
		}
		for _, occ := range *c.Evidence.Occurrences {
			if occ.Location == "" {
				t.Errorf("component %q has empty occurrence location", c.Name)
			}
			if occ.Line == nil || *occ.Line == 0 {
				t.Errorf("component %q has zero/nil occurrence line", c.Name)
			}
		}
	}
}

func TestScanSource_ComponentAttributes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "app.py"), `import hashlib`)

	comps, err := ScanSource(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(comps) != 1 {
		t.Fatalf("expected 1 component, got %d", len(comps))
	}

	c := comps[0]
	if c.Type != cdx.ComponentTypeCryptographicAsset {
		t.Errorf("type = %s, want cryptographic-asset", c.Type)
	}
	if c.Scope != cdx.ScopeRequired {
		t.Errorf("scope = %s, want required", c.Scope)
	}
	if c.CryptoProperties == nil {
		t.Fatal("CryptoProperties must be set")
	}
	if c.CryptoProperties.AssetType != cdx.CryptoAssetTypeAlgorithm {
		t.Errorf("asset type = %s, want algorithm", c.CryptoProperties.AssetType)
	}
	if c.Licenses == nil {
		t.Error("Licenses should be set for stdlib package")
	}
}

func TestMatchPythonPackage(t *testing.T) {
	cases := []struct {
		module string
		want   string
	}{
		{"hashlib", "hashlib"},
		{"ssl", "ssl"},
		{"cryptography.fernet", "cryptography.fernet"},
		{"cryptography.hazmat.primitives.kdf.pbkdf2", "cryptography.hazmat.primitives.kdf.pbkdf2"},
		{"cryptography.hazmat.primitives.hashes", "cryptography.hazmat.primitives.hashes"},
		{"cryptography.hazmat.backends", ""}, // not a known package
		{"rest_framework_simplejwt.authentication", "rest_framework_simplejwt"},
		{"rest_framework_simplejwt.tokens", "rest_framework_simplejwt"},
		{"django.utils.crypto", "django.utils.crypto"},
		{"Crypto.Cipher.AES", "Crypto.Cipher"},
		{"os", ""},
		{"json", ""},
	}

	for _, tc := range cases {
		t.Run(tc.module, func(t *testing.T) {
			got := matchPythonPackage(tc.module)
			if got != tc.want {
				t.Errorf("matchPythonPackage(%q) = %q, want %q", tc.module, got, tc.want)
			}
		})
	}
}

// ----- helpers -----

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func compNames(comps []cdx.Component) map[string]bool {
	m := make(map[string]bool, len(comps))
	for _, c := range comps {
		m[c.Name] = true
	}
	return m
}

func nameList(comps []cdx.Component) []string {
	names := make([]string, len(comps))
	for i, c := range comps {
		names[i] = c.Name
	}
	return names
}
