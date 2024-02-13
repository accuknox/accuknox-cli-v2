package onboard

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	"golang.org/x/mod/semver"
)

// path for writing configuration files
func createDefaultConfigPath() (string, error) {
	configPath, err := common.GetDefaultConfigPath()
	if err != nil {
		return "", err
	}

	_, err = os.Stat(configPath)
	// return all errors expect if given path does not exist
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	err = os.MkdirAll(configPath, os.ModeDir|os.ModePerm)
	if err != nil {
		return "", err
	}

	return configPath, nil
}

// parseURL with/without scheme and return host, port or error
func parseURL(address string) (string, string, error) {
	var host string
	port := "80"

	addr, err := url.Parse(address)
	if err != nil || addr.Host == "" {
		// URL without scheme
		u, repErr := url.ParseRequestURI("http://" + address)
		if repErr != nil {
			return "", "", fmt.Errorf("Error while parsing URL: %s", err)
		}

		addr = u
	}

	host = addr.Hostname()
	if addr.Port() != "" {
		port = addr.Port()
	}

	return host, port, nil
}

// copyOrGenerateFile copies a a config file from userConfigDir to the given path or writes file with the given template at the given path
func copyOrGenerateFile(userConfigDir, dirPath, filePath string, tempFuncs template.FuncMap, templateString string, templateArgs interface{}) (string, error) {
	dataFile := &bytes.Buffer{}

	// if user specified a config path - read if the given file
	// exists in it and skip template generation
	if userConfigDir != "" {
		userConfigFilePath := filepath.Join(userConfigDir, filePath)
		if _, err := os.Stat(userConfigFilePath); err != nil {
			return "", fmt.Errorf("error while opening user specified file: %s", err.Error())
		}

		userFileBytes, err := os.ReadFile(userConfigFilePath) // #nosec G304
		if err != nil {
			return "", err
		} else if len(userFileBytes) == 0 {
			return "", fmt.Errorf("empty config file given at %s", userConfigFilePath)
		}

		dataFile = bytes.NewBuffer(userFileBytes)

	} else {
		// generate the file with the template
		templateFile, err := template.New(filePath).Funcs(tempFuncs).Parse(templateString)
		if err != nil {
			return "", err
		}

		err = templateFile.Execute(dataFile, templateArgs)
		if err != nil {
			return "", err
		}
	}

	if dataFile == nil {
		return "", fmt.Errorf("Failed to read config file for %s: Empty file", filePath)
	}

	fullFilePath := filepath.Join(dirPath, filePath)
	fullFileDir := filepath.Dir(fullFilePath)

	// create needed directories at the path to write
	err := os.MkdirAll(fullFileDir, os.ModeDir|os.ModePerm)
	if err != nil && !os.IsExist(err) {
		return "", err
	}

	// fullFilePath contains the path to configDir - hard coding paths won't be efficient
	// overwrite files if need
	resultFile, err := os.OpenFile(fullFilePath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600) // #nosec G304
	if err != nil {
		return "", err
	}
	defer resultFile.Close()

	_, err = dataFile.WriteTo(resultFile)
	if err != nil {
		return "", err
	}

	return fullFilePath, nil
}

func compareVersionsAndGetComposeCommand(composeCLIVersion, returnCmd string) string {
	composeCLIVersionStr := strings.TrimSpace(string(composeCLIVersion))
	if composeCLIVersion != "" {
		if composeCLIVersionStr[0] != 'v' {
			composeCLIVersionStr = "v" + composeCLIVersionStr
		}

		if semver.Compare(composeCLIVersionStr, common.MinDockerComposeVersion) >= 0 {
			return returnCmd
		}
	}

	return ""
}

// GetComposeCommand gets the compose command with perfect version
// caller must check for empty
func GetComposeCommand() string {
	var err error

	_, err = exec.LookPath("docker-compose")
	if err != nil {
		// docker-compose doesn't exist
		// we'll use "docker compose"
		return "docker compose"
	}

	// docker-compose exists, compare versions
	composeCLIVersion, err := ExecComposeCommand(false, false, "docker-compose", "version", "--short")
	if err != nil {
		return ""
	}
	composeCmd := compareVersionsAndGetComposeCommand(composeCLIVersion, "docker-compose")
	if composeCmd != "" {
		return composeCmd
	}

	// docker-compose didn't match requirements so
	// check if "docker compose" meets version requirements
	composeDockerCLIVersion, err := ExecComposeCommand(false, false, "docker compose", "version", "--short")
	if err != nil {
		return ""
	}
	composeCmd = compareVersionsAndGetComposeCommand(composeDockerCLIVersion, "docker compose")
	if composeCmd != "" {
		return composeCmd
	}

	return ""
}

func ExecComposeCommand(setStdOut, dryRun bool, tryCmd string, args ...string) (string, error) {
	if !strings.Contains(tryCmd, "docker") {
		return "", fmt.Errorf("Command %s not supported", tryCmd)
	}

	composeCmd := new(exec.Cmd)

	cmd := strings.Split(tryCmd, " ")
	if len(cmd) == 1 {

		composeCmd = exec.Command(cmd[0]) // #nosec G204
		if dryRun {
			composeCmd.Args = append(composeCmd.Args, "--dry-run")
		}
		composeCmd.Args = append(composeCmd.Args, args...)

	} else if len(cmd) > 1 {

		// need this to handle docker compose command
		composeCmd = exec.Command(cmd[0], cmd[1]) // #nosec G204
		if dryRun {
			composeCmd.Args = append(composeCmd.Args, "--dry-run")
		}
		composeCmd.Args = append(composeCmd.Args, args...)

	} else {
		return "", fmt.Errorf("unknown compose command")
	}

	if setStdOut {
		composeCmd.Stdout = os.Stdout
		composeCmd.Stderr = os.Stderr

		err := composeCmd.Run()
		if err != nil {
			return "", err
		}

		return "", nil
	}

	stdout, err := composeCmd.CombinedOutput()
	if err != nil {
		return string(stdout), err
	}

	return string(stdout), nil
}

// validate the environment
func (cc *ClusterConfig) validateEnv() error {
	// check if docker exists
	_, err := exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("Error while looking for docker. Err: %s. Please install docker %s+.", err.Error(), common.MinDockerVersion)
	}

	serverVersionCmd := exec.Command("docker", "version", "-f", "{{.Server.Version}}")
	serverVersion, err := serverVersionCmd.Output()
	if err != nil {
		return err
	}

	serverVersionStr := strings.TrimSpace(string(serverVersion))
	if serverVersionStr != "" {
		if serverVersionStr[0] != 'v' {
			serverVersionStr = "v" + serverVersionStr
		}

		if semver.Compare(serverVersionStr, common.MinDockerVersion) < 0 {
			return fmt.Errorf("docker version %s not supported", serverVersionStr)
		}
	}

	composeCmd := GetComposeCommand()
	if composeCmd == "" {
		return fmt.Errorf("Please install docker-compose %s+", common.MinDockerComposeVersion)
	}

	cc.composeCmd = composeCmd

	return nil
}
