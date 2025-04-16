package vm

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
)

//go:embed rra
var rraBinary []byte
var rra string = "rra"

var fileName = "accuknox-%s_%s_result.json"

func PrepareRRACommand(profile, benchmark, authToken, label, url, tenantID, clusterName, clusterID string) []string {
	args := []string{
		"analyze",
		"--profile", profile,
		"--benchmark", benchmark,
		"--auth-token", authToken,
		"--url", url,
		"--tenant-id", tenantID,
		"--cluster-name", clusterName,
		"--label", label,
	}

	if clusterID != "" {
		args = append(args, "--cluster-id", clusterID)
	}

	args = append(args, "--json")

	return args
}

func ExecCommand(commandArgs []string, path, benchmark string, save bool) error {
	tmpDir, _ := os.MkdirTemp("", rra)
	defer os.RemoveAll(tmpDir)

	rraPath := filepath.Join(tmpDir, rra)
	err := os.WriteFile(rraPath, rraBinary, 0o700) // #nosec G306 need perms for write and execute
	if err != nil {
		return fmt.Errorf("failed to write RRA binary: %v", err)
	}
	fullCmd := fmt.Sprintf("%s %s", rraPath, strings.Join(commandArgs, " "))

	cmd := exec.Command("/bin/sh", "-c", fullCmd) // #nosec G204 (CWE-78)

	filePath := ""

	if save {
		filePath = filepath.Join(path, fmt.Sprintf(fileName, rra, benchmark))
		file, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %v", err)
		}
		logger.Info1("Output file created at %s", filePath)
		defer file.Close()
		cmd.Stdout = file
	} else {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Unable to execute RRA: %s", err.Error())
	}
	return nil
}
