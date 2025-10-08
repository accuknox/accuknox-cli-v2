package imagescan

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var defaultTrivyVersion = "0.64.1"

// Returns the tar.gz url for the provided trivy version
func GetTrivyDownloadURL(version string) string {
	osName := mapOS(runtime.GOOS)
	archName := mapArch(runtime.GOARCH)

	if osName == "" || archName == "" {
		fmt.Printf("unsupported platform: %s-%s\n", runtime.GOOS, runtime.GOARCH)
	}

	// Construct the URL
	return fmt.Sprintf(
		"https://github.com/aquasecurity/trivy/releases/download/v%s/trivy_%s_%s-%s.tar.gz",
		version, version, osName, archName,
	)
}

func mapOS(goos string) string {
	switch strings.ToLower(goos) {
	case "linux":
		return "Linux"
	case "darwin":
		return "macOS"
	case "windows":
		return "Windows"
	default:
		return ""
	}
}

func mapArch(goarch string) string {
	switch goarch {
	case "amd64":
		return "64bit"
	case "386":
		return "32bit"
	case "arm64":
		return "ARM64"
	case "arm":
		return "ARM"
	default:
		return ""
	}
}

// Checks if trivy is present in the $PATH variable
func IsTrivyInstalled() bool {
	if _, err := exec.LookPath("trivy"); err != nil {
		fmt.Println("Container image scanner is not installed or not found in $PATH. Installing It...")
		return false
	}
	return true
}

func IsValidDomain(domain string) bool {
	// regex for domain name validation
	re := regexp.MustCompile(`^https:\/\/[a-zA-Z0-9-]+\.[a-zA-Z0-9-.]+$`)
	return re.MatchString(domain)
}

// Installs trivy at $HOME/.accuknox-config/container-scanner
func installTrivy() error {
	binaryPath, err := getBinaryPath()
	if err != nil {
		return fmt.Errorf("error while creating binary path: %v", err)
	}
	if err := DownloadAndInstallBinary(GetTrivyDownloadURL(defaultTrivyVersion),
		filepath.Join(binaryPath, "trivy")); err != nil {
		return err
	}
	return nil
}
