// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Authors of KubeArmor

package cbom

import (
	"go/parser"
	"go/token"
	"strings"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

// goScanner detects crypto imports in Go source files.
type goScanner struct{}

func (g *goScanner) Extensions() []string { return []string{".go"} }

func (g *goScanner) ScanFile(path string) (map[string][]occurrence, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}

	found := map[string][]occurrence{}
	for _, imp := range f.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		if _, ok := knownGoPackages[importPath]; ok {
			pos := fset.Position(imp.Path.Pos())
			found[importPath] = append(found[importPath], occurrence{
				file: path,
				line: pos.Line,
			})
		}
	}
	return found, nil
}

// knownGoPackages maps Go import paths to their CBOM descriptor.
var knownGoPackages = map[string]cryptoEntry{
	// ── Standard library — symmetric ────────────────────────────────────────
	"crypto/aes": {
		name:        "AES",
		description: "Advanced Encryption Standard (AES) block cipher as defined in FIPS PUB 197.",
		primitive:   cdx.CryptoPrimitiveBlockCipher,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionEncrypt, cdx.CryptoFunctionDecrypt},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://csrc.nist.gov/publications/detail/fips/197/final")},
	},
	"crypto/des": {
		name:        "DES",
		description: "Data Encryption Standard (DES) and Triple-DES (3DES) block cipher.",
		primitive:   cdx.CryptoPrimitiveBlockCipher,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionEncrypt, cdx.CryptoFunctionDecrypt},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://csrc.nist.gov/publications/detail/fips/46/3/final")},
	},
	"crypto/rc4": {
		name:        "RC4",
		description: "RC4 stream cipher (deprecated; avoid in new designs).",
		primitive:   cdx.CryptoPrimitiveStreamCipher,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionEncrypt, cdx.CryptoFunctionDecrypt},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc7465")},
	},

	// ── Standard library — hash ──────────────────────────────────────────────
	"crypto/md5": {
		name:        "MD5",
		description: "MD5 message-digest algorithm producing a 128-bit hash (deprecated for security use).",
		primitive:   cdx.CryptoPrimitiveHash,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionDigest},
		params:      "128",
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc1321")},
	},
	"crypto/sha1": {
		name:        "SHA-1",
		description: "SHA-1 hash function producing a 160-bit digest (deprecated for security use).",
		primitive:   cdx.CryptoPrimitiveHash,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionDigest},
		params:      "160",
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://csrc.nist.gov/publications/detail/fips/180/4/final")},
	},
	"crypto/sha256": {
		name:        "SHA-256",
		description: "SHA-2 hash function producing a 256-bit digest as defined in FIPS PUB 180-4.",
		primitive:   cdx.CryptoPrimitiveHash,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionDigest},
		params:      "256",
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://csrc.nist.gov/publications/detail/fips/180/4/final")},
	},
	"crypto/sha512": {
		name:        "SHA-512",
		description: "SHA-2 hash function producing a 512-bit digest as defined in FIPS PUB 180-4.",
		primitive:   cdx.CryptoPrimitiveHash,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionDigest},
		params:      "512",
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://csrc.nist.gov/publications/detail/fips/180/4/final")},
	},

	// ── Standard library — MAC ───────────────────────────────────────────────
	"crypto/hmac": {
		name:        "HMAC",
		description: "Hash-based Message Authentication Code (HMAC) as defined in FIPS PUB 198-1.",
		primitive:   cdx.CryptoPrimitiveMAC,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionTag},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://csrc.nist.gov/publications/detail/fips/198/1/final")},
	},

	// ── Standard library — asymmetric ───────────────────────────────────────
	"crypto/rsa": {
		name:        "RSA",
		description: "RSA public-key cryptosystem used for encryption and digital signatures.",
		primitive:   cdx.CryptoPrimitivePKE,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionEncrypt, cdx.CryptoFunctionDecrypt, cdx.CryptoFunctionSign, cdx.CryptoFunctionVerify},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc8017")},
	},
	"crypto/ecdsa": {
		name:        "ECDSA",
		description: "Elliptic Curve Digital Signature Algorithm as defined in FIPS PUB 186-5.",
		primitive:   cdx.CryptoPrimitiveSignature,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionSign, cdx.CryptoFunctionVerify},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://csrc.nist.gov/publications/detail/fips/186/5/final")},
	},
	"crypto/ecdh": {
		name:        "ECDH",
		description: "Elliptic-Curve Diffie-Hellman key agreement.",
		primitive:   cdx.CryptoPrimitiveKeyAgree,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionKeygen},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc8037")},
	},
	"crypto/ed25519": {
		name:        "Ed25519",
		description: "Edwards-curve Digital Signature Algorithm (EdDSA) over Curve25519.",
		primitive:   cdx.CryptoPrimitiveSignature,
		curve:       "Ed25519",
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionSign, cdx.CryptoFunctionVerify},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc8032")},
	},
	"crypto/elliptic": {
		name:        "ECDSA",
		description: "NIST elliptic curve operations (P-224, P-256, P-384, P-521).",
		primitive:   cdx.CryptoPrimitiveSignature,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionKeygen},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://csrc.nist.gov/publications/detail/fips/186/5/final")},
	},

	// ── Standard library — random ────────────────────────────────────────────
	"crypto/rand": {
		name:        "DRBG",
		description: "Cryptographically secure pseudo-random number generator (CSPRNG).",
		primitive:   cdx.CryptoPrimitiveDRBG,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionGenerate},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://csrc.nist.gov/publications/detail/sp/800-90a/rev-1/final")},
	},

	// ── Standard library — DSA ──────────────────────────────────────────────
	"crypto/dsa": {
		name:        "DSA",
		description: "Digital Signature Algorithm (DSA) as defined in FIPS PUB 186-5 (deprecated).",
		primitive:   cdx.CryptoPrimitiveSignature,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionSign, cdx.CryptoFunctionVerify},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://csrc.nist.gov/publications/detail/fips/186/5/final")},
	},

	// ── Standard library — protocols / infrastructure ────────────────────────
	"crypto/tls": {
		name:        "TLS",
		description: "Transport Layer Security (TLS) protocol implementation.",
		assetType:   cdx.CryptoAssetTypeProtocol,
		protocol:    cdx.CryptoProtocolTypeTLS,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc8446")},
	},
	"crypto/x509": {
		name:        "X.509",
		description: "X.509 public key infrastructure (PKI) and certificate handling.",
		assetType:   cdx.CryptoAssetTypeCertificate,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc5280")},
	},
	"crypto/x509/pkix": {
		name:        "X.509/PKIX",
		description: "ASN.1 PKIX structures used in X.509 certificates and CRLs.",
		assetType:   cdx.CryptoAssetTypeCertificate,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc5280")},
	},

	// ── golang.org/x/crypto ──────────────────────────────────────────────────
	"golang.org/x/crypto/chacha20": {
		name:        "ChaCha20",
		description: "ChaCha20 stream cipher as defined in RFC 8439.",
		primitive:   cdx.CryptoPrimitiveStreamCipher,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionEncrypt, cdx.CryptoFunctionDecrypt},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc8439")},
	},
	"golang.org/x/crypto/chacha20poly1305": {
		name:        "ChaCha20-Poly1305",
		description: "ChaCha20-Poly1305 AEAD cipher as defined in RFC 8439.",
		primitive:   cdx.CryptoPrimitiveAE,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionEncrypt, cdx.CryptoFunctionDecrypt},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc8439")},
	},
	"golang.org/x/crypto/argon2": {
		name:        "Argon2",
		description: "Argon2 memory-hard password hashing function (winner of the Password Hashing Competition).",
		primitive:   cdx.CryptoPrimitiveKDF,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionKeyderive},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc9106")},
	},
	"golang.org/x/crypto/bcrypt": {
		name:        "bcrypt",
		description: "bcrypt adaptive password hashing function based on Blowfish.",
		primitive:   cdx.CryptoPrimitiveKDF,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionKeyderive},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.usenix.org/legacy/events/usenix99/provos/provos.pdf")},
	},
	"golang.org/x/crypto/pbkdf2": {
		name:        "PBKDF2",
		description: "Password-Based Key Derivation Function 2 as defined in RFC 8018.",
		primitive:   cdx.CryptoPrimitiveKDF,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionKeyderive},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc8018")},
	},
	"golang.org/x/crypto/scrypt": {
		name:        "scrypt",
		description: "scrypt memory-hard key derivation function as defined in RFC 7914.",
		primitive:   cdx.CryptoPrimitiveKDF,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionKeyderive},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc7914")},
	},
	"golang.org/x/crypto/hkdf": {
		name:        "HKDF",
		description: "HMAC-based Key Derivation Function (HKDF) as defined in RFC 5869.",
		primitive:   cdx.CryptoPrimitiveKDF,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionKeyderive},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc5869")},
	},
	"golang.org/x/crypto/blake2b": {
		name:        "BLAKE2b",
		description: "BLAKE2b cryptographic hash function optimised for 64-bit platforms.",
		primitive:   cdx.CryptoPrimitiveHash,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionDigest},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc7693")},
	},
	"golang.org/x/crypto/blake2s": {
		name:        "BLAKE2s",
		description: "BLAKE2s cryptographic hash function optimised for 8- to 32-bit platforms.",
		primitive:   cdx.CryptoPrimitiveHash,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionDigest},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc7693")},
	},
	"golang.org/x/crypto/nacl/box": {
		name:        "NaCl/box",
		description: "NaCl box: authenticated public-key encryption using Curve25519, XSalsa20, and Poly1305.",
		primitive:   cdx.CryptoPrimitiveAE,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionEncrypt, cdx.CryptoFunctionDecrypt},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeWebsite, "https://nacl.cr.yp.to/box.html")},
	},
	"golang.org/x/crypto/nacl/secretbox": {
		name:        "NaCl/secretbox",
		description: "NaCl secretbox: authenticated secret-key encryption using XSalsa20 and Poly1305.",
		primitive:   cdx.CryptoPrimitiveAE,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionEncrypt, cdx.CryptoFunctionDecrypt},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeWebsite, "https://nacl.cr.yp.to/secretbox.html")},
	},
	"golang.org/x/crypto/ssh": {
		name:        "SSH",
		description: "Secure Shell (SSH) protocol implementation.",
		assetType:   cdx.CryptoAssetTypeProtocol,
		protocol:    cdx.CryptoProtocolTypeSSH,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc4251")},
	},
	"golang.org/x/crypto/openpgp": {
		name:        "OpenPGP",
		description: "OpenPGP message format for encryption and signing (RFC 4880).",
		primitive:   cdx.CryptoPrimitiveOther,
		functions:   []cdx.CryptoFunction{cdx.CryptoFunctionEncrypt, cdx.CryptoFunctionSign},
		assetType:   cdx.CryptoAssetTypeAlgorithm,
		license:     "BSD-3-Clause",
		refs:        []cdx.ExternalReference{ref(cdx.ERTypeOther, "https://www.rfc-editor.org/rfc/rfc4880")},
	},
}
