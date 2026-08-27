package aspm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
)

const binaryName = "accuknox-aspm-scanner"

// getDownloadURL returns the appropriate binary URL based on OS and architecture
func getDownloadURL() (string, error) {
	// Fetch latest release tag
	resp, err := http.Get("https://api.github.com/repos/accuknox/aspm-scanner-cli/releases/latest")
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status: %s", resp.Status)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse release response: %w", err)
	}

	baseURL := fmt.Sprintf("https://github.com/accuknox/aspm-scanner-cli/releases/download/%s", release.TagName)

	switch runtime.GOOS {
	case "linux":
		switch runtime.GOARCH {
			case "amd64":
				return baseURL + "/accuknox-aspm-scanner", nil
		}
	case "windows":
		switch runtime.GOARCH {
			case "amd64":
				return baseURL + "/accuknox-aspm-scanner.exe", nil
		}
	}
	return "", fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
}

// ExecuteASPM is the main function that ensures the binary exists and runs it
func ExecuteASPM() error {
	binPath, err := getBinaryPath()
	if err != nil {
		return err
	}

	if !fileExists(binPath) {
		if err := installBinary(binPath); err != nil {
			return fmt.Errorf("installation failed: %w", err)
		}
	}

	if err := runBinary(binPath); err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}

	return nil
}

// getBinaryPath returns the full path to where the ASPM binary should live
func getBinaryPath() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("unable to determine current user: %w", err)
	}

	binName := binaryName

	// --- Step 1: Check if binary is already in PATH ---
	foundPath, err := exec.LookPath(binName)
	if err == nil && foundPath != "" {
		// Already installed somewhere in PATH
		return foundPath, nil
	}

	// --- Step 2: Decide default install path ---
	var installDir string
	if runtime.GOOS == "windows" {
		// Equivalent to USERPROFILE\AppData\Local\Programs\AccuKnox
		userProfile := os.Getenv("USERPROFILE")
		if userProfile == "" {
			return "", fmt.Errorf("USERPROFILE not set")
		}
		installDir = filepath.Join(userProfile, "AppData", "Local", "Programs", "AccuKnox")
		binName += ".exe"
	} else {
		// Default for Linux ~/.local/bin/accuknox
		installDir = filepath.Join(usr.HomeDir, ".local", "bin")
	}

	// Make sure directory exists
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create install directory: %w", err)
	}

	return filepath.Join(installDir, binName), nil
}


// fileExists checks if a file exists at the given path
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// installBinary downloads and installs the binary to the target path
func installBinary(destPath string) error {

	url, err := getDownloadURL()
	fmt.Println("Downloading ASPM helper...", url)
	if err != nil {
		return err
	}

	// Create directory
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Download binary
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status while downloading: %s", resp.Status)
	}

	// Save to file
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("failed to save binary: %w", err)
	}

	// Make executable
	if runtime.GOOS != "windows" {
		if err := os.Chmod(destPath, 0755); err != nil {
			return fmt.Errorf("chmod failed: %w", err)
		}
	}

	fmt.Println("Installation complete.")
	return nil
}

// runBinary executes the binary with all command-line args passed to this process
func runBinary(binPath string) error {
	var args []string
	for i, arg := range os.Args {
		if arg == "aspm" && i+1 < len(os.Args) {
			args = os.Args[i+1:]
			break
		}
	}

	// If no args found after "aspm", just show help
	if len(args) == 0 {
		args = []string{"--help"}
	}

	cmd := exec.Command(binPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}
