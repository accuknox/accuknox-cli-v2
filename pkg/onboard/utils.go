package onboard

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/docker/docker/client"
	"github.com/google/go-github/github"
	"golang.org/x/mod/semver"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
)

var Agents_download = map[string]string{

	cm.Vm_adapter:     "docker.io/accuknox/vm-adapter-systemd",
	cm.Relay_server:   "docker.io/accuknox/kubearmor-relay-server-systemd",
	cm.Pea_agent:      "docker.io/accuknox/accuknox-policy-enforcement-agent-systemd",
	cm.Sia_agent:      "docker.io/accuknox/accuknox-shared-informer-agent-systemd",
	cm.Feeder_service: "docker.io/accuknox/accuknox-feeder-service-systemd",
	cm.Spire_agent:    "docker.io/accuknox/spire-agent-systemd",
}

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

	// ignoring G304 - fullFilePath contains the path to configDir - hard coding
	// paths won't be efficient
	// ignoring G302 - if containers are run by the root user, members of the
	// docker group should be able to read the files
	// overwrite files if need
	resultFile, err := os.OpenFile(fullFilePath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644) // #nosec G304 G302
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

		if semver.Compare(v1Clean, v2Clean) >= 0 && semver.Compare(v1Clean, common.MinDockerComposeVersion) >= 0 {
			return v1Cmd, v1Clean
		} else if semver.Compare(v1Clean, v2Clean) <= 0 && semver.Compare(v2Clean, common.MinDockerComposeVersion) >= 0 {
			return v2Cmd, v2Clean
		} else {
			return "", ""
		}

	} else if v1Clean != "" {
		if v1Clean[0] != 'v' {
			v1Clean = "v" + v1Clean
		}

		if semver.Compare(v1Clean, common.MinDockerComposeVersion) >= 0 {
			return v1Cmd, v1Clean
		} else {
			return "", ""
		}
	} else if v2Clean != "" {
		if v2Clean[0] != 'v' {
			v2Clean = "v" + v2Clean
		}

		if semver.Compare(v2Clean, common.MinDockerComposeVersion) >= 0 {
			return v2Cmd, v2Clean
		} else {
			return "", ""
		}
	}

	return "", ""
}

// GetComposeCommand gets the compose command with perfect version
// caller must check for empty
func GetComposeCommand() (string, string) {
	var err error

	_, err = exec.LookPath("docker-compose")
	if err != nil {
		// docker-compose doesn't exist
		// we'll use "docker compose"
		composeDockerCLIVersion, err := ExecComposeCommand(false, false, "docker compose", "version", "--short")
		if err != nil {
			return "", ""
		}

		return compareVersionsAndGetComposeCommand(composeDockerCLIVersion, "docker compose", common.MinDockerComposeVersion, "")
	}

	// docker-compose exists, compare versions
	composeCLIVersion, err := ExecComposeCommand(false, false, "docker-compose", "version", "--short")
	if err != nil {
		return "", ""
	}

	// docker-compose didn't match requirements so
	// check if "docker compose" meets version requirements
	composeDockerCLIVersion, err := ExecComposeCommand(false, false, "docker compose", "version", "--short")
	if err != nil {
		return "", ""
	}

	composeCmd, finalVersion := compareVersionsAndGetComposeCommand(composeCLIVersion, "docker-compose", composeDockerCLIVersion, "docker compose")
	if composeCmd != "" {
		return composeCmd, finalVersion
	}

	return "", ""
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
		return "", fmt.Errorf("Error while looking for docker. Err: %s. Please install docker %s+.", err.Error(), common.MinDockerVersion)
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

		if semver.Compare(serverVersionStr, common.MinDockerVersion) < 0 {
			return "", fmt.Errorf("docker version %s not supported", serverVersionStr)
		}
	}

	composeCmd, composeVersion := GetComposeCommand()
	if composeCmd == "" {
		return "", fmt.Errorf("Please install docker-compose %s+", common.MinDockerComposeVersion)
	}

	cc.composeCmd = composeCmd
	cc.composeVersion = composeVersion

	return fmt.Sprintf("Using %s version %s\n", composeCmd, composeVersion), nil
}

func CreateDockerClient() (*client.Client, error) {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	return dockerClient, nil
}

// for Systemd mode
func ExtractAndRun(fileName string) error {

	file, err := os.Open(filepath.Clean(fileName))
	if err != nil {
		fmt.Println("Error opening file:", fileName, err)
		return err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		fmt.Println("Error creating gzip reader:", err)
		return err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			fmt.Println("Error reading tar header:", err)
			return err
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}
		rootDir := "/"

		// Extract the file
		filename := filepath.Join(rootDir, header.Name) // #nosec G305

		// Create parent directories if not exist

		err = os.MkdirAll(filepath.Dir(filename), 0755) // #nosec G301
		if err != nil {
			return err
		}
		file, err := os.Create(filepath.Clean(filename))
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(file, tarReader) // #nosec G110
		if err != nil {
			return err
		}

		// Set execute permissions for the binaries

		if header.Mode&0111 != 0 {
			err := os.Chmod(filename, 0755) // #nosec G302
			if err != nil {
				return err
			}
		}
	}
	return nil

}

func DownloadPackage(owner, repo, tag string, folderName string) (string, error) {

	ctx := context.Background()
	client := github.NewClient(nil)
	release, _, err := client.Repositories.GetReleaseByTag(ctx, owner, repo, tag)
	if err != nil {
		fmt.Printf("Error getting release: %v\n", err)
		return "", err
	}
	arch := runtime.GOARCH

	// Download .tar.gz assets
	filename := ""
	for _, asset := range release.Assets {

		if filepath.Ext(asset.GetName()) == ".gz" && strings.Contains(asset.GetName(), arch) {

			filename = filepath.Join(folderName, asset.GetName())
			fmt.Printf("Downloading asset: %s\n", filename)

			output, err := os.Create(filename)
			if err != nil {
				fmt.Printf("Error creating file: %v\n", err)
				return "", err
			}
			defer output.Close()

			resp, err := http.Get(asset.GetBrowserDownloadURL())
			if err != nil {
				fmt.Printf("Error downloading asset: %v\n", err)
				return "", err
			}
			defer resp.Body.Close()

			// Write downloaded content to file
			_, err = io.Copy(output, resp.Body)
			if err != nil {
				fmt.Printf("Error writing to file: %v\n", err)
				return "", err
			}
		}
	}
	return filename, nil

}

func VerifyBTF() (bool, error) {

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
func InstallAgent(agntName, agentTag string) error {

	fileName, err := DownloadAccunkoxAgent(agntName, agentTag)
	if err != nil {
		return err
	}
	fmt.Println("Downloaded:", agntName)

	err = ExtractAndRun(fileName)
	if err != nil {
		return err
	}
	fmt.Println("Extracted:", agntName)
	return nil
}
func GetLatestTag(owner_name, repo_name string) (string, error) {

	ctx := context.Background()
	client := github.NewClient(nil)

	// Get the latest release
	release, _, err := client.Repositories.GetLatestRelease(ctx, owner_name, repo_name)
	if err != nil {

		return "", fmt.Errorf("failed to get latest release: %v", err)
	}
	return release.GetTagName(), nil

}
func DownloadAccunkoxAgent(pkgName string, tag string) (string, error) {

	fs, err := file.New(cm.Download_dir)
	if err != nil {
		return "", err
	}
	defer fs.Close()

	// 1. Connect to a remote repository
	ctx := context.Background()
	repo, err := remote.NewRepository(Agents_download[pkgName])
	if err != nil {
		return "", err
	}
	_, err = oras.Copy(ctx, repo, tag, fs, tag, oras.DefaultCopyOptions)
	if err != nil {
		return "", err
	}
	filepath := path.Join(cm.Download_dir, pkgName+"_"+tag+".tar.gz")
	return filepath, nil
}
func InstallKa(kaVersionTag string) error {

	// Verify BTF installation
	btfPresent, _ := VerifyBTF()
	if !btfPresent {
		return fmt.Errorf("installation failed: BTF not found ")
	} else {
		fmt.Println("BTF installation found ")
	}

	err := os.MkdirAll(cm.Download_dir, 0755) // #nosec G301
	if err != nil {
		if !os.IsExist(err) {
			return fmt.Errorf("error creating folder: %s", err.Error())
		}
	}
	fmt.Println("Folder created:", cm.Download_dir)
	owner := "kubearmor"
	repo := "KubeArmor"
	tag := kaVersionTag

	filename, err := DownloadPackage(owner, repo, tag, cm.Download_dir)

	if err != nil {
		return err
	}

	err = ExtractAndRun(filename)

	if err != nil {
		return err
	}
	return nil

}

func GetSystemdTag(userTag, releaseTag string) string {
	tagSuffix := "_" + runtime.GOOS + "-" + runtime.GOARCH
	tag := ""
	if userTag != "" {
		tag = strings.TrimPrefix(userTag, "v") + tagSuffix
	} else {
		tag = strings.TrimPrefix(releaseTag, "v") + tagSuffix
	}
	return tag
}
func CheckSystemdInstallation() (bool, error) {
	agents := []string{"kubearmor", cm.KA_Vm_Adapter, cm.Relay_server, cm.Pea_agent, cm.Sia_agent, cm.Feeder_service, cm.Spire_agent}
	systemdPath := "/usr/lib/systemd/system/"
	for _, agent := range agents {
		filePath := systemdPath + agent + ".service"
		if _, err := os.Stat(filePath); err == nil {
			// found service file means we have agents as systemd service
			return true, nil
		} else if !os.IsNotExist(err) {
			return false, fmt.Errorf("Error checking service file for %s: %v\n", agent, err)
		}
	}
	return false, nil
}
func StartSystemdService(serviceName string) error {
	ctx := context.Background()
	// Connect to systemd dbus
	conn, err := dbus.NewWithContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to systemd: %v", err)
	}
	defer conn.Close()

	// reload systemd config, equivalent to systemctl daemon-reload
	if err := conn.ReloadContext(ctx); err != nil {
		return fmt.Errorf("failed to reload systemd configuration: %v", err)
	}

	// enable service
	_, _, err = conn.EnableUnitFilesContext(ctx, []string{serviceName}, false, true)
	if err != nil {
		return fmt.Errorf("failed to enable %s: %v", serviceName, err)
	}
	fmt.Println("Enabled service:", serviceName)

	// Start the service
	ch := make(chan string)
	fmt.Printf("Starting %s...\n", serviceName)
	if _, err := conn.RestartUnitContext(ctx, serviceName, "replace", ch); err != nil {
		return fmt.Errorf("failed to start %s: %v", serviceName, err)
	}
	return nil
}
func StopSystemdService(serviceName string) error {

	ctx := context.Background()
	conn, err := dbus.NewWithContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to systemd: %v", err)
	}
	defer conn.Close()
	stopChan := make(chan string)

	if _, err := conn.StopUnitContext(ctx, serviceName, "replace", stopChan); err != nil {
		if !strings.Contains(err.Error(), "not loaded") {
			fmt.Printf("Failed to stop %s: %v\n", serviceName, err)
			return err
		}
	} else {
		fmt.Printf("Stopping %s .\n", serviceName)
		<-stopChan
		fmt.Printf("%s service stopped successfully.\n", serviceName)
	}

	if _, err := conn.DisableUnitFilesContext(ctx, []string{serviceName}, false); err != nil {
		if !strings.Contains(err.Error(), "does not exist") {
			fmt.Printf("Failed to disable %s : %v\n", serviceName, err)
			return err
		}
	} else {
		fmt.Printf("Disabled %s .\n", serviceName)
	}

	svcfilePath := cm.SystemdDir + serviceName
	if err := os.Remove(svcfilePath); err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("Failed to delete %s file: %v\n", serviceName, err)
			return err
		}
	} else {
		fmt.Printf("Deleted %s file.\n", serviceName)
	}
	// reload systemd config, equivalent to systemctl daemon-reload
	if err := conn.ReloadContext(ctx); err != nil {

		return fmt.Errorf("failed to reload systemd configuration: %v", err)
	}

	return err
}
func Deletedir(dirName string) {
	//	Clean Up
	err := os.RemoveAll(dirName)
	if err != nil && !os.IsNotExist(err) {
		// Check if the error is due to the directory not existing
		fmt.Printf("error deleting %s : %v\n", dirName, err)
	}

}
func DeboardSystemd(nodeType NodeType) error {

	// stop services
	err := StopSystemdService("kubearmor.service")
	if err != nil {
		fmt.Println("error stopping service kubearmor.service:", err)
		return err
	}
	err = StopSystemdService("kubearmor-vm-adapter.service")
	if err != nil {
		fmt.Println("error stopping service kubearmor-vm-adapter.service:", err)
		return err
	}
	if nodeType == NodeType_ControlPlane {

		agents := []string{cm.Pea_agent, cm.Sia_agent, cm.Feeder_service, cm.Relay_server, cm.Spire_agent}

		for _, agent := range agents {
			err := StopSystemdService(agent + ".service")
			if err != nil {
				fmt.Printf("error stopping service %s.service:%s\n", agent, err)
				return err
			}
		}
	}
	// delete directories
	dirs := []string{cm.KAconfigPath, cm.PEAconfigPath,
		cm.SIAconfigPath, cm.VmAdapterconfigPath,
		cm.FSconfigPath, cm.RelayServerconfigPath, cm.SpireconfigPath, cm.PeaPolicyPath}

	for _, dirName := range dirs {
		Deletedir(dirName)
	}
	return nil
}
