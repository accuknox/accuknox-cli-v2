package aspm

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
)

const (
	binaryURL  = "https://github.com/accuknox/aspm-scanner-cli/releases/download/v0.7.14/accuknox-aspm-scanner_linux_x86_64"
	binaryName = "accuknox-aspm-scanner"
)

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
	return filepath.Join(usr.HomeDir, ".local", "bin", binaryName), nil
}

// fileExists checks if a file exists at the given path
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// installBinary downloads and installs the binary to the target path
func installBinary(destPath string) error {
	fmt.Println("ASPM Scanner not found. Downloading and installing...")

	// Create directory
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Download binary
	resp, err := http.Get(binaryURL)
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
	if err := os.Chmod(destPath, 0755); err != nil {
		return fmt.Errorf("chmod failed: %w", err)
	}

	fmt.Println("Installation complete.")
	return nil
}

// runBinary executes the binary with all command-line args passed to this process
func runBinary(binPath string) error {
	// Find the index of "aspm" in os.Args
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
