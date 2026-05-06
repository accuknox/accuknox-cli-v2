// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Authors of KubeArmor

package cbom

import (
	"bufio"
	"os"
	"regexp"
	"sort"
	"strings"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

// pythonScanner detects crypto imports in Python source files.
type pythonScanner struct{}

func (p *pythonScanner) Extensions() []string { return []string{".py"} }

// importRe matches "import foo" and "from foo.bar import baz" statements.
var (
	pyImportRe     = regexp.MustCompile(`^\s*import\s+([A-Za-z_][\w.]*)`)
	pyFromImportRe = regexp.MustCompile(`^\s*from\s+([A-Za-z_][\w.]*)\s+import`)
)

func (p *pythonScanner) ScanFile(path string) (map[string][]occurrence, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	found := map[string][]occurrence{}
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		var modulePath string
		if m := pyFromImportRe.FindStringSubmatch(line); m != nil {
			modulePath = m[1]
		} else if m := pyImportRe.FindStringSubmatch(line); m != nil {
			modulePath = m[1]
		} else {
			continue
		}

		if key := matchPythonPackage(modulePath); key != "" {
			found[key] = append(found[key], occurrence{
				file: path,
				line: lineNum,
			})
		}
	}
	return found, scanner.Err()
}

// sortedPythonPrefixes holds known package keys sorted longest-first so that
// prefix matching picks the most specific entry.
var sortedPythonPrefixes []string

func init() {
	sortedPythonPrefixes = make([]string, 0, len(knownPythonPackages))
	for k := range knownPythonPackages {
		sortedPythonPrefixes = append(sortedPythonPrefixes, k)
	}
	sort.Slice(sortedPythonPrefixes, func(i, j int) bool {
		return len(sortedPythonPrefixes[i]) > len(sortedPythonPrefixes[j])
	})
}

// matchPythonPackage returns the known-package key that best matches the
// given Python module path, or "" if there is no match.
// A module matches a key if the module equals the key or is a sub-package of
// it (e.g. "cryptography.hazmat.primitives.kdf.pbkdf2" matches
// "cryptography.hazmat.primitives.kdf.pbkdf2"), or if the key is a prefix
// of the module at a dot boundary.
func matchPythonPackage(module string) string {
	for _, prefix := range sortedPythonPrefixes {
		if module == prefix {
			return prefix
		}
		if strings.HasPrefix(module, prefix+".") {
			return prefix
		}
	}
	return ""
}

// knownPythonPackages maps Python import paths to their CBOM descriptor.
var knownPythonPackages = map[string]cryptoEntry{
	// ── Python standard library ─────────────────────────────────────────────
	"hashlib": {
		name:        "hashlib",
		description: "Python hashlib module providing MD5, SHA-1, SHA-2, SHA-3, and BLAKE2 hash algorithms.",
		primitive:   cdx.CryptoPrimitiveHash,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionDigest},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "PSF-2.0",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeDocumentation, "https://docs.python.org/3/library/hashlib.html")},
	},
	"hmac": {
		name:        "HMAC",
		description: "Python hmac module implementing keyed-hashing for message authentication (RFC 2104).",
		primitive:   cdx.CryptoPrimitiveMAC,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionTag},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "PSF-2.0",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeDocumentation, "https://docs.python.org/3/library/hmac.html")},
	},
	"ssl": {
		name:        "TLS",
		description: "Python ssl module providing TLS/SSL wrapper for socket objects.",
		assetType:   cdx.CryptoAssetTypeProtocol,
		protocol:    cdx.CryptoProtocolTypeTLS,
		license:     "PSF-2.0",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeDocumentation, "https://docs.python.org/3/library/ssl.html")},
	},
	"secrets": {
		name:        "CSPRNG",
		description: "Python secrets module for generating cryptographically strong random numbers.",
		primitive:   cdx.CryptoPrimitiveDRBG,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionGenerate},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "PSF-2.0",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeDocumentation, "https://docs.python.org/3/library/secrets.html")},
	},

	// ── pyca/cryptography — symmetric ───────────────────────────────────────
	"cryptography.fernet": {
		name:        "Fernet",
		description: "Fernet symmetric encryption (AES-128-CBC + HMAC-SHA256) from the cryptography package.",
		primitive:   cdx.CryptoPrimitiveAE,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionEncrypt, cdx.CryptoFunctionDecrypt},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "Apache-2.0",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeDocumentation, "https://cryptography.io/en/latest/fernet/")},
	},
	"cryptography.hazmat.primitives.ciphers": {
		name:        "AES",
		description: "Symmetric ciphers (AES, Camellia, ChaCha20, etc.) from pyca/cryptography.",
		primitive:   cdx.CryptoPrimitiveBlockCipher,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionEncrypt, cdx.CryptoFunctionDecrypt},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "Apache-2.0",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeDocumentation, "https://cryptography.io/en/latest/hazmat/primitives/symmetric-encryption/")},
	},

	// ── pyca/cryptography — hash ────────────────────────────────────────────
	"cryptography.hazmat.primitives.hashes": {
		name:        "cryptography-hashes",
		description: "Cryptographic hash algorithms (SHA-2, SHA-3, BLAKE2, etc.) from pyca/cryptography.",
		primitive:   cdx.CryptoPrimitiveHash,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionDigest},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "Apache-2.0",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeDocumentation, "https://cryptography.io/en/latest/hazmat/primitives/cryptographic-hashes/")},
	},
	"cryptography.hazmat.primitives.hmac": {
		name:        "cryptography-HMAC",
		description: "HMAC implementation from pyca/cryptography.",
		primitive:   cdx.CryptoPrimitiveMAC,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionTag},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "Apache-2.0",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeDocumentation, "https://cryptography.io/en/latest/hazmat/primitives/mac/hmac/")},
	},
	"cryptography.hazmat.primitives.cmac": {
		name:        "CMAC",
		description: "Cipher-based Message Authentication Code (CMAC) from pyca/cryptography.",
		primitive:   cdx.CryptoPrimitiveMAC,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionTag},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "Apache-2.0",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeDocumentation, "https://cryptography.io/en/latest/hazmat/primitives/mac/cmac/")},
	},

	// ── pyca/cryptography — KDF ─────────────────────────────────────────────
	"cryptography.hazmat.primitives.kdf.pbkdf2": {
		name:        "PBKDF2",
		description: "PBKDF2 key derivation from pyca/cryptography (RFC 8018).",
		primitive:   cdx.CryptoPrimitiveKDF,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionKeyderive},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "Apache-2.0",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc8018")},
	},
	"cryptography.hazmat.primitives.kdf.scrypt": {
		name:        "scrypt",
		description: "scrypt key derivation from pyca/cryptography (RFC 7914).",
		primitive:   cdx.CryptoPrimitiveKDF,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionKeyderive},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "Apache-2.0",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc7914")},
	},
	"cryptography.hazmat.primitives.kdf.hkdf": {
		name:        "HKDF",
		description: "HKDF key derivation from pyca/cryptography (RFC 5869).",
		primitive:   cdx.CryptoPrimitiveKDF,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionKeyderive},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "Apache-2.0",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc5869")},
	},
	"cryptography.hazmat.primitives.kdf.concatkdf": {
		name:        "ConcatKDF",
		description: "Concat KDF key derivation from pyca/cryptography (NIST SP 800-56Ar2).",
		primitive:   cdx.CryptoPrimitiveKDF,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionKeyderive},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "Apache-2.0",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeDocumentation, "https://cryptography.io/en/latest/hazmat/primitives/key-derivation-functions/#cryptography.hazmat.primitives.kdf.concatkdf.ConcatKDFHash")},
	},

	// ── pyca/cryptography — asymmetric ──────────────────────────────────────
	"cryptography.hazmat.primitives.asymmetric.rsa": {
		name:        "RSA",
		description: "RSA public-key cryptography from pyca/cryptography.",
		primitive:   cdx.CryptoPrimitivePKE,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionEncrypt, cdx.CryptoFunctionDecrypt, cdx.CryptoFunctionSign, cdx.CryptoFunctionVerify},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "Apache-2.0",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc8017")},
	},
	"cryptography.hazmat.primitives.asymmetric.ec": {
		name:        "ECDSA",
		description: "Elliptic curve cryptography (ECDSA, ECDH) from pyca/cryptography.",
		primitive:   cdx.CryptoPrimitiveSignature,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionSign, cdx.CryptoFunctionVerify, cdx.CryptoFunctionKeygen},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "Apache-2.0",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://csrc.nist.gov/publications/detail/fips/186/5/final")},
	},
	"cryptography.hazmat.primitives.asymmetric.ed25519": {
		name:        "Ed25519",
		description: "Ed25519 digital signatures from pyca/cryptography.",
		primitive:   cdx.CryptoPrimitiveSignature,
		curve:       "Ed25519",
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionSign, cdx.CryptoFunctionVerify},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "Apache-2.0",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc8032")},
	},
	"cryptography.hazmat.primitives.asymmetric.ed448": {
		name:        "Ed448",
		description: "Ed448 digital signatures from pyca/cryptography.",
		primitive:   cdx.CryptoPrimitiveSignature,
		curve:       "Ed448",
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionSign, cdx.CryptoFunctionVerify},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "Apache-2.0",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc8032")},
	},
	"cryptography.hazmat.primitives.asymmetric.dh": {
		name:        "DH",
		description: "Diffie-Hellman key exchange from pyca/cryptography.",
		primitive:   cdx.CryptoPrimitiveKeyAgree,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionKeygen},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "Apache-2.0",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc7919")},
	},
	"cryptography.hazmat.primitives.asymmetric.dsa": {
		name:        "DSA",
		description: "DSA digital signatures from pyca/cryptography (deprecated).",
		primitive:   cdx.CryptoPrimitiveSignature,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionSign, cdx.CryptoFunctionVerify},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "Apache-2.0",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://csrc.nist.gov/publications/detail/fips/186/5/final")},
	},

	// ── pyca/cryptography — X.509 ───────────────────────────────────────────
	"cryptography.x509": {
		name:        "X.509",
		description: "X.509 certificate handling from pyca/cryptography.",
		assetType:   cdx.CryptoAssetTypeCertificate,
		license:     "Apache-2.0",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc5280")},
	},

	// ── Third-party packages ────────────────────────────────────────────────
	"bcrypt": {
		name:        "bcrypt",
		description: "bcrypt adaptive password hashing (Python bcrypt package).",
		primitive:   cdx.CryptoPrimitiveKDF,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionKeyderive},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "Apache-2.0",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.usenix.org/legacy/events/usenix99/provos/provos.pdf")},
	},
	"argon2": {
		name:        "Argon2",
		description: "Argon2 memory-hard password hashing (argon2-cffi package).",
		primitive:   cdx.CryptoPrimitiveKDF,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionKeyderive},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "MIT",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc9106")},
	},
	"jwt": {
		name:        "JWT",
		description: "JSON Web Token encoding/decoding (PyJWT) using HMAC, RSA, or ECDSA signatures.",
		primitive:   cdx.CryptoPrimitiveOther,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionSign, cdx.CryptoFunctionVerify},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "MIT",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc7519")},
	},
	"paramiko": {
		name:        "SSH",
		description: "Paramiko SSHv2 protocol implementation for Python.",
		assetType:   cdx.CryptoAssetTypeProtocol,
		protocol:    cdx.CryptoProtocolTypeSSH,
		license:     "LGPL-2.1-only",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc4251")},
	},
	"nacl": {
		name:        "NaCl",
		description: "PyNaCl bindings for libsodium (Curve25519, XSalsa20, Poly1305).",
		primitive:   cdx.CryptoPrimitiveAE,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionEncrypt, cdx.CryptoFunctionDecrypt},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "Apache-2.0",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeWebsite, "https://nacl.cr.yp.to/")},
	},

	// ── PyCryptodome (Crypto / Cryptodome) ──────────────────────────────────
	"Crypto.Cipher": {
		name:        "PyCryptodome-Cipher",
		description: "Symmetric ciphers (AES, DES, ChaCha20, etc.) from PyCryptodome.",
		primitive:   cdx.CryptoPrimitiveBlockCipher,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionEncrypt, cdx.CryptoFunctionDecrypt},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-2-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeDocumentation, "https://pycryptodome.readthedocs.io/en/latest/src/cipher/cipher.html")},
	},
	"Crypto.Hash": {
		name:        "PyCryptodome-Hash",
		description: "Cryptographic hash functions from PyCryptodome.",
		primitive:   cdx.CryptoPrimitiveHash,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionDigest},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-2-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeDocumentation, "https://pycryptodome.readthedocs.io/en/latest/src/hash/hash.html")},
	},
	"Crypto.PublicKey": {
		name:        "PyCryptodome-PublicKey",
		description: "Public-key algorithms (RSA, DSA, ECC) from PyCryptodome.",
		primitive:   cdx.CryptoPrimitivePKE,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionEncrypt, cdx.CryptoFunctionDecrypt, cdx.CryptoFunctionSign, cdx.CryptoFunctionVerify},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-2-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeDocumentation, "https://pycryptodome.readthedocs.io/en/latest/src/public_key/public_key.html")},
	},
	"Crypto.Signature": {
		name:        "PyCryptodome-Signature",
		description: "Digital signature schemes from PyCryptodome.",
		primitive:   cdx.CryptoPrimitiveSignature,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionSign, cdx.CryptoFunctionVerify},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-2-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeDocumentation, "https://pycryptodome.readthedocs.io/en/latest/src/signature/signature.html")},
	},
	"Crypto.Random": {
		name:        "PyCryptodome-Random",
		description: "Cryptographic random number generation from PyCryptodome.",
		primitive:   cdx.CryptoPrimitiveDRBG,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionGenerate},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-2-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeDocumentation, "https://pycryptodome.readthedocs.io/en/latest/src/random/random.html")},
	},
	"Cryptodome.Cipher": {
		name:        "PyCryptodome-Cipher",
		description: "Symmetric ciphers from PyCryptodome (Cryptodome namespace).",
		primitive:   cdx.CryptoPrimitiveBlockCipher,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionEncrypt, cdx.CryptoFunctionDecrypt},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-2-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeDocumentation, "https://pycryptodome.readthedocs.io/en/latest/src/cipher/cipher.html")},
	},
	"Cryptodome.Hash": {
		name:        "PyCryptodome-Hash",
		description: "Cryptographic hash functions from PyCryptodome (Cryptodome namespace).",
		primitive:   cdx.CryptoPrimitiveHash,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionDigest},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-2-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeDocumentation, "https://pycryptodome.readthedocs.io/en/latest/src/hash/hash.html")},
	},
	"Cryptodome.PublicKey": {
		name:        "PyCryptodome-PublicKey",
		description: "Public-key algorithms from PyCryptodome (Cryptodome namespace).",
		primitive:   cdx.CryptoPrimitivePKE,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionEncrypt, cdx.CryptoFunctionDecrypt, cdx.CryptoFunctionSign, cdx.CryptoFunctionVerify},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-2-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeDocumentation, "https://pycryptodome.readthedocs.io/en/latest/src/public_key/public_key.html")},
	},

	// ── Django ──────────────────────────────────────────────────────────────
	"django.utils.crypto": {
		name:        "Django-crypto",
		description: "Django cryptographic utilities (constant-time compare, PBKDF2, random string generation).",
		primitive:   cdx.CryptoPrimitiveKDF,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionKeyderive},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeDocumentation, "https://docs.djangoproject.com/en/stable/topics/signing/")},
	},

	// ── simplejwt (Django REST framework JWT) ───────────────────────────────
	"rest_framework_simplejwt": {
		name:        "SimpleJWT",
		description: "Django REST framework Simple JWT authentication using HMAC/RSA/ECDSA token signing.",
		primitive:   cdx.CryptoPrimitiveOther,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionSign, cdx.CryptoFunctionVerify},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "MIT",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc7519")},
	},

	// ── pyOpenSSL ───────────────────────────────────────────────────────────
	"OpenSSL": {
		name:        "pyOpenSSL",
		description: "pyOpenSSL wrapper around OpenSSL for TLS, X.509, and cryptographic operations.",
		assetType:   cdx.CryptoAssetTypeProtocol,
		protocol:    cdx.CryptoProtocolTypeTLS,
		license:     "Apache-2.0",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeDocumentation, "https://www.pyopenssl.org/en/latest/")},
	},
}
