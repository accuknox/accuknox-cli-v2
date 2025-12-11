package onboard

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Masterminds/sprig"
	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
)

func (ic *InitConfig) InitializeControlPlaneSD() error {

	ic.TCArgs.SpireSecretDir = ic.SpireSecretDir
	ic.TCArgs.KubeArmorURL = "0.0.0.0:32767"
	ic.TCArgs.KubeArmorPort = "32767"

	ic.TCArgs.RelayServerURL = "0.0.0.0:32768"
	ic.TCArgs.RelayServerAddr = "0.0.0.0"
	ic.TCArgs.RelayServerPort = "32768"

	ic.TCArgs.WorkerNode = ic.WorkerNode

	ic.TCArgs.SIAAddr = "0.0.0.0:32769"
	ic.TCArgs.PEAAddr = "0.0.0.0:32770"
	ic.TCArgs.HardenAddr = "0.0.0.0:32771"

	ic.TCArgs.VmMode = ic.Mode

	ic.TCArgs.NodeStateRefreshTime = ic.NodeStateRefreshTime

	var err error
	if ic.Tls.Enabled {
		ic.TCArgs.TlsEnabled = ic.Tls.Enabled
		ic.TCArgs.TlsCertFile = fmt.Sprintf("%s%s%s/%s", ic.UserConfigPath, "/opt", cm.DefaultCACertDir, cm.DefaultEncodedFileName)
		if err = ic.handleTLS(); err != nil {
			return err
		}
	}

	ic.TCArgs.AccessKey = ic.AccessKey

	ic.populateCommonArgs()

	if ic.TCArgs.SplunkConfigObject.Enabled {
		if err = validateSplunkCredential(ic.TCArgs.SplunkConfigObject); err != nil {
			return err
		}
	}

	// copy config files according to custom configuration specified by the user
	kmuxConfigArgs := KmuxConfigTemplateArgs{
		ReleaseVersion: ic.AgentsVersion,
		StreamName:     "knox-gateway",
		ServerURL:      ic.KnoxGateway,
		RMQServer:      "rabbitmq:5672",
		RMQUsername:    ic.TCArgs.RMQUsername,
		RMQPassword:    ic.TCArgs.RMQPassword,
		TlsEnabled:     ic.TCArgs.TlsEnabled,
		TlsCertFile:    ic.TCArgs.TlsCertFile,
		ProxyEnabled:   ic.TCArgs.ProxyEnabled,
		ProxyAddress:   ic.TCArgs.ProxyAddress,
	}

	if ic.Proxy.Address != "" {
		kmuxConfigArgs.ProxyEnabled = true
	}

	if ic.RMQServer != "" {
		ic.TCArgs.RMQAddr = ic.RMQServer
		kmuxConfigArgs.RMQServer = ic.TCArgs.RMQAddr
	} else if ic.CPNodeAddr != "" {
		ic.TCArgs.RMQAddr = ic.CPNodeAddr + ":5672"
		kmuxConfigArgs.RMQServer = ic.TCArgs.RMQAddr
	} else {
		ic.TCArgs.RMQAddr = "0.0.0.0:5672"
		kmuxConfigArgs.RMQServer = ic.TCArgs.RMQAddr
	}

	ic.TCArgs.RMQUsername,
		ic.TCArgs.RMQPassword,
		err = getRMQUserPass(ic.Tls.RMQCredentials)
	if err != nil {
		return err
	}

	if ic.TCArgs.RMQAddr != "" {
		if err = testRMQConnection(ic.TCArgs.RMQAddr, ic.TCArgs.RMQUsername, ic.TCArgs.RMQPassword, ic.Tls.CaCert, ic.Tls.CaPath); err != nil {
			return err
		}
	}

	// initialize sprig for templating
	ic.TemplateFuncs = sprig.GenericFuncMap()

	// download and extract systemd packages
	logger.Info2(("Downloading agents..."))
	err = ic.SystemdInstall()
	if err != nil {
		logger.Error("Installation failed!! Cleaning up downloaded assets...")
		// ignoring G104 - can't send nil in installation failed case
		Deletedir(cm.DownloadDir)
		DeboardSystemd(NodeType_ControlPlane) // #nosec G104
		return err
	}

	logger.Info2("\nConfiguring services...")
	for _, obj := range ic.SystemdServiceObjects {

		if obj.AgentName == cm.HardeningAgent && !ic.EnableHardeningAgent {
			continue
		}

		if ic.Proxy.Enabled && obj.AgentName == cm.SpireAgent {
			continue
		}

		if obj.ConfigFilePath != "" {
			// copy template args
			tcArgs := ic.TCArgs

			// copy kmux config path for specifying in agent config
			if obj.KmuxConfigPath != "" {
				tcArgs.KmuxConfigPath = obj.KmuxConfigPath
			}

			_, err = copyOrGenerateFile(ic.UserConfigPath, obj.AgentDir, obj.ConfigFilePath, ic.TemplateFuncs, obj.ConfigTemplateString, tcArgs)
			if err != nil {
				logger.Error("err config generate: %v\n", err)
				return err
			}
		}
		// copy kmux config
		if obj.KmuxConfigPath != "" {
			populateKmuxArgs(&kmuxConfigArgs, obj.AgentName, obj.KmuxConfigFileName, ic.TCArgs.RMQTopicPrefix, ic.TCArgs.Hostname, ic.RMQConnectionName)
			kmuxConfigArgs.UseCaFile = useCaFile(&ic.TCArgs, obj.AgentName, obj.AgentImage)
			_, err = copyOrGenerateFile(ic.UserConfigPath, obj.AgentDir, obj.KmuxConfigFileName, ic.TemplateFuncs, obj.KmuxConfigTemplateString, kmuxConfigArgs)
			if err != nil {
				logger.Error("err kmux generate: %v\n", err)
				return err
			}
		}

		// copy additional files
		for filename, srcPath := range obj.ExtraFilePathSrc {
			if srcPath == "" {
				continue
			}

			destPath, ok := obj.ExtraFilePathDest[filename]
			if !ok {
				logger.Warn("Warning! No destination for extra file %s", filename)
				continue
			}

			srcPathDir := filepath.Dir(srcPath)
			destPathDir := filepath.Dir(destPath)

			_, err = copyOrGenerateFile(srcPathDir, destPathDir, filename, nil, "", nil)
			if err != nil {
				return err
			}
		}

	}

	// FINALLY START THE SYSTEMD SERVICES //
	logger.Info2("\nEnabling services...")
	for _, obj := range ic.SystemdServiceObjects {

		if obj.AgentName == cm.HardeningAgent && !ic.EnableHardeningAgent {
			continue
		}
		err = StartSystemdService(obj.ServiceName)
		if err != nil {
			logger.Warn("failed to start service %s: %s\n", obj.ServiceName, err.Error())
			return err
		}

	}

	// Clean Up
	logger.Info1("\nCleaning up downloaded assets...")
	Deletedir(cm.DownloadDir)
	return nil
}

func getSystemdVersion() int {
	cmd := exec.Command("systemctl", "--version")
	out, err := cmd.Output()
	if err != nil {
		return 240
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) == 0 {
		return 240
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 2 {
		return 240
	}
	version, err := strconv.Atoi(fields[1])
	if err != nil {
		return 240
	}
	return version
}
