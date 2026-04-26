package tools

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

var embeddedConfig []byte

// Load parses the embedded tools.yaml.
func Load() (*Config, error) {
	return parse(embeddedConfig)
}

// LoadFromFile parses tools.yaml from the given path (used by build scripts).
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	return parse(data)
}

func parse(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse tools config: %w", err)
	}
	return &cfg, nil
}

// ResolveForPlatform returns the PlatformConfig and resolved install filename
// for the given OS/arch, or an error if the tool is not available.
func (t *Tool) ResolveForPlatform(goos, goarch string) (*PlatformConfig, string, error) {
	platform, ok := t.Platforms[goos]
	if !ok {
		return nil, "", fmt.Errorf("tool %q is not available on %s", t.Name, goos)
	}
	cfg, ok := platform[goarch]
	if !ok {
		return nil, "", fmt.Errorf("tool %q is not available on %s/%s", t.Name, goos, goarch)
	}
	installAs := t.InstallAs
	if cfg.InstallAs != "" {
		installAs = cfg.InstallAs
	}
	return &cfg, installAs, nil
}

// EnsureInstalled returns a path to the tool binary, extracting or downloading
// it as needed.
//
// For builtin tools (builtin: true in tools.yaml), the binary is built from
// source (e.g. a git submodule) and embedded at build time. Only the embedded
// extraction and side-by-side paths are attempted; no network download is done.
//
// For downloaded tools, the lookup order is:
//  1. Embedded in the knoxctl binary (via //go:embed bins) → extract to versioned cache
//  2. Next to the knoxctl executable (manual side-by-side deployment)
//  3. ~/.accuknox-config/tools/ (previously downloaded at runtime)
//  4. Download from source URL to ~/.accuknox-config/tools/
func (t *Tool) EnsureInstalled() (string, error) {
	installAs := t.installAsForPlatform()

	// 1. Extract from the binary embedded at build time.
	if path, err := t.extractEmbedded(installAs); err == nil {
		return path, nil
	}

	// 2. Check next to the running binary.
	if execPath, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(execPath), installAs)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	if t.Builtin {
		return "", fmt.Errorf("%s binary is not embedded — run 'make prebuild' to build it from the submodule", t.Name)
	}

	// Resolve platform-specific download config for steps 3 and 4.
	cfg, _, err := t.ResolveForPlatform(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}

	// 3. Check the user's runtime download cache.
	cacheDir := installDir()
	cached := filepath.Join(cacheDir, installAs)
	if _, err := os.Stat(cached); err == nil {
		return cached, nil
	}

	// 4. Download.
	fmt.Printf("Downloading %s %s...\n", t.Name, t.Version)
	if err := os.MkdirAll(cacheDir, 0o750); err != nil { // #nosec G301
		return "", fmt.Errorf("failed to create install dir: %w", err)
	}
	if err := downloadAndInstall(cfg, installAs, cached); err != nil {
		return "", fmt.Errorf("failed to install %s: %w", t.Name, err)
	}
	fmt.Printf("Installed %s to %s\n", t.Name, cached)
	return cached, nil
}

// installAsForPlatform returns the install filename for the current platform,
// falling back to the tool-level InstallAs when no per-platform override exists.
func (t *Tool) installAsForPlatform() string {
	if platform, ok := t.Platforms[runtime.GOOS]; ok {
		if cfg, ok := platform[runtime.GOARCH]; ok && cfg.InstallAs != "" {
			return cfg.InstallAs
		}
	}
	if runtime.GOOS == "windows" && !strings.HasSuffix(t.InstallAs, ".exe") {
		return t.InstallAs + ".exe"
	}
	return t.InstallAs
}

// extractEmbedded reads the tool binary from binsFS (embedded at build time),
// writes it to a versioned cache directory, and returns its path.
// The cache is keyed by tool version so a new knoxctl release always extracts
// a fresh copy instead of reusing a stale one.
// Returns an error if the binary is not embedded (dev builds, unavailable platform).
func (t *Tool) extractEmbedded(installAs string) (string, error) {
	data, err := binsFS.ReadFile("bins/" + installAs)
	if err != nil || len(data) == 0 {
		return "", fmt.Errorf("binary not embedded")
	}

	extractDir := filepath.Join(embeddedExtractDir(), t.Version)
	if err := os.MkdirAll(extractDir, 0o750); err != nil { // #nosec G301
		return "", fmt.Errorf("failed to create extract dir: %w", err)
	}

	dest := filepath.Join(extractDir, installAs)
	if _, err := os.Stat(dest); err == nil {
		return dest, nil // already extracted from this version
	}

	if err := os.WriteFile(dest, data, 0o755); err != nil { // #nosec G306
		return "", fmt.Errorf("failed to extract embedded %s: %w", installAs, err)
	}
	return dest, nil
}

// embeddedExtractDir returns the base directory for extracted embedded binaries.
func embeddedExtractDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), ".accuknox-config", "tools", "embedded")
	}
	return filepath.Join(home, ".accuknox-config", "tools", "embedded")
}

// buildCacheDir returns the persistent cache directory for build-time tool downloads,
// keyed by goos/goarch so cross-platform builds don't collide.
func buildCacheDir(goos, goarch string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), ".accuknox-config", "tools", "build-cache", goos+"_"+goarch)
	}
	return filepath.Join(home, ".accuknox-config", "tools", "build-cache", goos+"_"+goarch)
}

// DownloadTo copies the tool binary for the specified goos/goarch into outputDir.
// It maintains a persistent cache under ~/.accuknox-config/tools/build-cache so
// that tools are not re-downloaded across goreleaser runs (even after dist/ is cleaned).
// Used by build scripts (e.g. scripts/download-tools) during release packaging.
func (t *Tool) DownloadTo(goos, goarch, outputDir string) error {
	cfg, installAs, err := t.ResolveForPlatform(goos, goarch)
	if err != nil {
		// Tool not available for this platform — skip silently.
		return nil
	}
	cacheDir := buildCacheDir(goos, goarch)
	cached := filepath.Join(cacheDir, installAs)

	// Download into the cache if not already there.
	if _, err := os.Stat(cached); os.IsNotExist(err) {
		fmt.Printf("  Downloading %s (%s/%s)...\n", t.Name, goos, goarch)
		if err := os.MkdirAll(cacheDir, 0o750); err != nil { // #nosec G301
			return fmt.Errorf("failed to create build cache dir: %w", err)
		}
		if err := downloadAndInstall(cfg, installAs, cached); err != nil {
			return fmt.Errorf("failed to download %s for %s/%s: %w", t.Name, goos, goarch, err)
		}
		fmt.Printf("  Cached at %s\n", cached)
	} else {
		fmt.Printf("  Using cached %s\n", cached)
	}

	// Copy from cache into the output directory.
	dest := filepath.Join(outputDir, installAs)
	if err := copyFile(cached, dest); err != nil {
		return fmt.Errorf("failed to copy %s to output: %w", installAs, err)
	}
	fmt.Printf("  -> %s\n", dest)
	return nil
}

// copyFile copies src to dst preserving executable permissions.
func copyFile(src, dst string) error {
	in, err := os.Open(src) // #nosec G304
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode()) // #nosec G304
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// installDir returns the user-level tool cache directory.
func installDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), ".accuknox-config", "tools")
	}
	return filepath.Join(home, ".accuknox-config", "tools")
}

// archiveBinary returns the name of the binary to look for inside an archive.
// Uses cfg.Binary if explicitly set; otherwise falls back to installAs.
func (cfg *PlatformConfig) archiveBinary(installAs string) string {
	if cfg.Binary != "" {
		return cfg.Binary
	}
	return installAs
}

// downloadAndInstall fetches the source URL, verifies the optional SHA256, then
// extracts the binary from .tar.gz / .zip archives or saves it directly.
func downloadAndInstall(cfg *PlatformConfig, installAs, destPath string) error {
	resp, err := http.Get(cfg.Source) // #nosec G107
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d for %s", resp.StatusCode, cfg.Source)
	}

	tmp, err := os.CreateTemp("", "knoxctl-tool-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hasher), resp.Body); err != nil {
		_ = tmp.Close() // #nosec G104
		return fmt.Errorf("failed to write download: %w", err)
	}
	_ = tmp.Close() // #nosec G104

	if cfg.SHA256 != "" {
		got := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(got, cfg.SHA256) {
			return fmt.Errorf("SHA256 mismatch: got %s, want %s", got, cfg.SHA256)
		}
	}

	switch {
	case strings.HasSuffix(cfg.Source, ".tar.gz") || strings.HasSuffix(cfg.Source, ".tgz"):
		return extractFromTarGz(tmpPath, cfg.archiveBinary(installAs), destPath)
	case strings.HasSuffix(cfg.Source, ".zip"):
		return extractFromZip(tmpPath, cfg.archiveBinary(installAs), destPath)
	default:
		return installBinary(tmpPath, destPath)
	}
}

// extractFromTarGz finds the entry whose base name matches binaryName and
// extracts it to destPath.
func extractFromTarGz(archivePath, binaryName, destPath string) error {
	f, err := os.Open(archivePath) // #nosec G304
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("not a valid gzip file: %w", err)
	}
	defer gz.Close()

	target := strings.ToLower(strings.TrimSuffix(binaryName, ".exe"))
	tr := tar.NewReader(gz)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		base := strings.ToLower(strings.TrimSuffix(filepath.Base(hdr.Name), ".exe"))
		if base == target {
			return writeExecutable(tr, destPath)
		}
	}
	return fmt.Errorf("binary %q not found in archive", binaryName)
}

// extractFromZip finds the matching entry and extracts it to destPath.
func extractFromZip(archivePath, binaryName, destPath string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("not a valid zip file: %w", err)
	}
	defer zr.Close()

	target := strings.ToLower(strings.TrimSuffix(binaryName, ".exe"))

	for _, f := range zr.File {
		base := strings.ToLower(strings.TrimSuffix(filepath.Base(f.Name), ".exe"))
		if base == target {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()
			return writeExecutable(rc, destPath)
		}
	}
	return fmt.Errorf("binary %q not found in zip archive", binaryName)
}

// installBinary copies src to dest with executable permissions.
func installBinary(src, dest string) error {
	data, err := os.ReadFile(src) // #nosec G304
	if err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o755) // #nosec G306 G703
}

// writeExecutable writes r to destPath with executable permissions.
func writeExecutable(r io.Reader, destPath string) error {
	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755) // #nosec G304 G302
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", destPath, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, r); err != nil {
		return fmt.Errorf("failed to write %s: %w", destPath, err)
	}
	return nil
}
