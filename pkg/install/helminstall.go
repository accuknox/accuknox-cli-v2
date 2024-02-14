package install

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed helm/*
var helmChart embed.FS

// extractHelmChart extracts the embedded Helm chart to a temporary directory and returns the path.
func extractHelmChart() (string, error) {
	tempDir, err := os.MkdirTemp("", "helm-chart-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir for helm chart: %w", err)
	}

	err = fs.WalkDir(helmChart, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == "." || path == "helm" {
			return nil
		}

		relPath := strings.TrimPrefix(path, "helm/")

		targetPath := filepath.Join(tempDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, os.ModePerm)
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), os.ModePerm); err != nil {
			return err
		}

		data, err := helmChart.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file from embedded fs: %w", err)
		}
		return os.WriteFile(targetPath, data, os.ModePerm)

	})

	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to extract helm chart: %w", err)
	}

	fmt.Printf("Created a temp directory: %s\n", tempDir)
	return tempDir, nil
}

// installViaHelm will install Discovery Engine using the charts embedded within the binary of knoxctl
func installViaHelm(o Options) error {
	if !checkHelmInstalled() {
		fmt.Println("Helm is not installed please, visit https://helm.sh/docs/intro/install/ for installation instructions.")
		return errors.New("Helm is not installed")
	}

	chartPath, err := extractHelmChart()
	if err != nil {
		return fmt.Errorf("failed to read helm chart: %v", err)
	}

	var stdout, stderr bytes.Buffer
	args := []string{
		"install", "accuknox-agents", chartPath, "--namespace", "accuknox-agents", "--create-namespace",
	}
	if o.Debug {
		fmt.Printf("Executing helm command: %+v\n", args)
		args = append(args, "--debug")
	}
	cmd := exec.Command("helm", args...) // #nosec G204

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run `helm install` command: %v, stderr: %v", err, stderr.String())
	}

	fmt.Printf("Helm install output: %v\n", stderr.String())
	return nil
}

// checkHelmInstalled checks if Helm is installed :)
func checkHelmInstalled() bool {
	_, err := exec.LookPath("helm")
	return err == nil
}
