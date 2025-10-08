package onboard

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	se_splunk "github.com/accuknox/dev2/sumengine/pkg/sumengine/kubearmor"
	"github.com/docker/docker/client"
	"github.com/golang-jwt/jwt"
	"golang.org/x/mod/semver"
)

func DumpConfig(config interface{}, path string) error {
	byteData, err := json.Marshal(config)
	if err != nil {
		return err
	}

	err = os.WriteFile(path, byteData, 0o644) // #nosec G306 need perms to be this for archiving
	if err != nil {
		return err
	}

	return nil
}

// path for writing configuration files
func createDefaultConfigPath() (string, error) {
	configPath, err := cm.GetDefaultConfigPath()
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

	} else if tempFuncs != nil {
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

	if dataFile == nil || len(dataFile.Bytes()) == 0 {
		return "", fmt.Errorf("Failed to read config file for %s: Empty file", filePath)
	}

	fullFilePath := filepath.Join(dirPath, filePath)
	fullFileDir := filepath.Dir(fullFilePath)

	// create needed directories at the path to write
	err := os.MkdirAll(fullFileDir, os.ModeDir|os.ModePerm)
	if err != nil && !os.IsExist(err) {
		return "", err
	}

	// ignoring G304 - fullFilePath contains the path to configDir - hard coding
	// paths won't be efficient
	// ignoring G302 - if containers are run by the root user, members of the
	// docker group should be able to read the files
	// overwrite files if need
	resultFile, err := os.OpenFile(fullFilePath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644) // #nosec G304 G302
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

func compareVersionsAndGetComposeCommand(v1, v1Cmd, v2, v2Cmd string) (string, string) {
	v1Clean := strings.TrimSpace(string(v1))
	v2Clean := strings.TrimSpace(string(v2))

	if v1Clean != "" && v2Clean != "" {
		if v1Clean[0] != 'v' {
			v1Clean = "v" + v1Clean
		}

		if v2Clean[0] != 'v' {
			v2Clean = "v" + v2Clean
		}

		if semver.Compare(v1Clean, v2Clean) >= 0 && semver.Compare(v1Clean, cm.MinDockerComposeVersion) >= 0 {
			return v1Cmd, v1Clean
		} else if semver.Compare(v1Clean, v2Clean) <= 0 && semver.Compare(v2Clean, cm.MinDockerComposeVersion) >= 0 {
			return v2Cmd, v2Clean
		} else {
			return "", ""
		}

	} else if v1Clean != "" {
		if v1Clean[0] != 'v' {
			v1Clean = "v" + v1Clean
		}

		if semver.Compare(v1Clean, cm.MinDockerComposeVersion) >= 0 {
			return v1Cmd, v1Clean
		} else {
			return "", ""
		}
	} else if v2Clean != "" {
		if v2Clean[0] != 'v' {
			v2Clean = "v" + v2Clean
		}

		if semver.Compare(v2Clean, cm.MinDockerComposeVersion) >= 0 {
			return v2Cmd, v2Clean
		} else {
			return "", ""
		}
	}

	return "", ""
}

// GetComposeCommand gets the compose command with perfect version
// caller must check for empty
func GetComposeCommand() (string, string, error) {
	var (
		err            error
		tryComposeCMDs = []string{"docker-compose", "docker compose"}
		minVersion     = cm.MinDockerComposeVersion
		prevCommand    = ""
	)

	for _, command := range tryComposeCMDs {
		version, execErr := ExecComposeCommand(false, false, command, "version", "--short")
		if execErr != nil {
			if err != nil {
				err = fmt.Errorf("%s. while executing %s: %s", err.Error(), command, execErr.Error())
			} else {
				err = fmt.Errorf("while executing %s: %s", command, execErr.Error())
			}

			continue
		}

		composeCmd, finalVersion := compareVersionsAndGetComposeCommand(version, command, minVersion, prevCommand)
		if composeCmd != "" {
			return composeCmd, finalVersion, nil
		}

		// use command with latest version
		prevCommand = command
	}

	if err != nil {
		return "", "", fmt.Errorf("docker requirements not met: %s", err.Error())
	}

	return "", "", fmt.Errorf("docker requirements not met")
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
			if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
				return "", errors.New(string(exitErr.Stderr))
			}
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

func ExecDockerCommand(setStdOut, dryRun bool, tryCmd string, args ...string) (string, error) {
	dockerCmd := exec.Command(tryCmd) // #nosec G204
	if dryRun {
		dockerCmd.Args = append(dockerCmd.Args, "--dry-run")
	}

	dockerCmd.Args = append(dockerCmd.Args, args...)

	if setStdOut {
		dockerCmd.Stdout = os.Stdout
		dockerCmd.Stderr = os.Stderr

		err := dockerCmd.Run()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
				return "", errors.New(string(exitErr.Stderr))
			}

			return "", err
		}

		return "", nil
	}

	stdout, err := dockerCmd.CombinedOutput()
	if err != nil {
		return string(stdout), err
	}

	return string(stdout), nil
}

// validate the environment
func (cc *ClusterConfig) ValidateEnv() (string, error) {
	// check if docker exists
	_, err := exec.LookPath("docker")
	if err != nil {
		return "", fmt.Errorf("Error while looking for docker. Err: %s. Please install docker %s+.", err.Error(), cm.MinDockerVersion)
	}

	serverVersionCmd := exec.Command("docker", "version", "-f", "{{.Server.Version}}")
	serverVersion, err := serverVersionCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return "", errors.New(string(exitErr.Stderr))
		}
		return "", err
	}

	serverVersionStr := strings.TrimSpace(string(serverVersion))
	if serverVersionStr != "" {
		if serverVersionStr[0] != 'v' {
			serverVersionStr = "v" + serverVersionStr
		}

		if semver.Compare(serverVersionStr, cm.MinDockerVersion) < 0 {
			return "", fmt.Errorf("docker version %s not supported", serverVersionStr)
		}
	}

	composeCmd, composeVersion, err := GetComposeCommand()
	if err != nil {
		return "", fmt.Errorf("Error: %s. Please install docker-compose %s+", err.Error(), cm.MinDockerComposeVersion)
	}
	cc.composeCmd = composeCmd
	cc.composeVersion = composeVersion

	return fmt.Sprintf("Using %s version %s\n", composeCmd, composeVersion), nil
}

func CreateDockerClient() (*client.Client, error) {

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	return dockerClient, nil
}

func verifyBTF() (bool, error) {
	btfPath := "/sys/kernel/btf/vmlinux"

	// Check if the file exists
	if _, err := os.Stat(btfPath); err == nil {
		// btf present
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

func GetJoinTokenFromAccessKey(accessKey, clusterName, vmName, vmType, url string, insecure bool) (string, error) {
	if accessKey == "" || url == "" {
		return "", fmt.Errorf("invalid accessKey or url")
	}

	if vmName == "" && vmType == "Node" {
		host, _ := os.Hostname()
		vmName = fmt.Sprintf("%v-%v", host, time.Now().Unix())
		vmName = strings.TrimSuffix(vmName, "-")
	}

	if vmName == "" && clusterName == "" {
		return "", fmt.Errorf("clusterName or vmName is required")
	}

	if !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	url = url + AccessKeyEndpoint

	payload, err := createPayload(accessKey, clusterName, vmName, vmType)
	if err != nil {
		fmt.Printf("createPayload failed: %v\n", err)
		return "", err
	}
	return getJoinToken(payload, url, accessKey, insecure)
}

func createPayload(onboardingToken, clusterName, vmName, vmType string) ([]byte, error) {
	payload := map[string]interface{}{
		"cluster_name": clusterName,
		"token":        onboardingToken,
		"type":         vmType,
		"node_name":    vmName,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("json marshal failed: %v\n", err)
		return nil, err
	}

	return jsonPayload, nil
}

func getJoinToken(payload []byte, apiURL, token string, insecure bool) (string, error) {
	// create a new request using http [method; POST]
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payload))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return "", err
	}
	tenantID, err := getTenantID(token)
	if err != nil {
		fmt.Println("Error getting tenant ID:", err)
		return "", err
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("X-Tenant-Id", tenantID)

	// TODO: custom CA

	transportConfig := http.DefaultTransport.(*http.Transport).Clone()
	transportConfig.TLSClientConfig.InsecureSkipVerify = insecure

	httpClient := http.Client{
		Transport: transportConfig,
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return "", err
	}
	defer resp.Body.Close()
	var response TokenResponse

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		fmt.Println("Error decoding response:", err)
		return "", err
	}

	if resp.StatusCode != http.StatusOK || response.Message != "success" {
		return "", fmt.Errorf("received error code: %s message: %s", response.ErrorCode, response.ErrorMessage)
	}

	return response.JoinToken, nil
}

func getTenantID(onboardingToken string) (string, error) {
	parts := strings.Split(onboardingToken, ".")
	if len(parts) != 3 {
		return "", ErrInvalidToken
	}

	var claims jwt.MapClaims

	decodedClaims, err := jwt.DecodeSegment(parts[1])
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(decodedClaims, &claims)
	if err != nil {
		return "", err
	}

	tid := fmt.Sprintf("%v", claims["tenant-id"])
	return tid, nil
}

// ReadLine reads a line from the reader with trailing \r dropped.
func ReadLine(reader io.Reader) ([]byte, error) {
	var line []byte
	var buffer [1]byte
	for {
		n, err := reader.Read(buffer[:])
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if n == 0 {
			continue
		}
		c := buffer[0]
		if c == '\n' {
			break
		}
		line = append(line, c)
	}
	return bytes.TrimSuffix(line, []byte{'\r'}), nil
}

// splitLast splits at the last index of separator
func splitLast(fullString, separator string) []string {
	colonIdx := strings.LastIndex(fullString, separator)

	// bound check
	if colonIdx <= 0 || colonIdx == (len(fullString)-1) {
		return []string{fullString}
	}

	return []string{fullString[:colonIdx], fullString[colonIdx+1:]}
}

func CheckRRASystemdInstallation() (bool, error) {
	// check RRA service and Timer file
	files := []string{"accuknox-rra.service", "accuknox-rra.timer"}

	for _, file := range files {
		filePath := cm.SystemdPath + file
		if _, err := os.Stat(filePath); err == nil {
			// found service or timer file means we have RRA installation as systemd
			return true, nil
		} else if !os.IsNotExist(err) {
			return false, fmt.Errorf("error checking service file %s: %v", filePath, err)
		}
	}
	return false, nil
}

func validateSplunkCredential(splunkConfig SplunkConfig) error {
	return se_splunk.ValidateSplunkCredentials(splunkConfig.Url, splunkConfig.Token, splunkConfig.Source, splunkConfig.SourceType, splunkConfig.Index, splunkConfig.Certificate, splunkConfig.SkipTls)
}

func getLastOldVersion(agentName string) string {
	return cm.LastOldAgentVersion[agentName]
}
