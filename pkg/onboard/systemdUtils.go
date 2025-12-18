package onboard

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Masterminds/sprig"
	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/coreos/go-systemd/v22/dbus"
	"golang.org/x/mod/semver"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
)

var (
	workerNodeAgents = []string{cm.KubeArmor, cm.VMAdapter, cm.SummaryEngine}
	cpNodeAgents     = []string{cm.SpireAgent, cm.SIAAgent, cm.PEAAgent, cm.FeederService, cm.SummaryEngine, cm.DiscoverAgent, cm.HardeningAgent}
)

func (cc *ClusterConfig) CreateSystemdServiceObjects() {
	systemdObjects := []SystemdServiceObject{
		{
			AgentName:             cm.KubeArmor,
			PackageName:           cm.KubeArmor,
			ServiceName:           cm.KubeArmor + ".service",
			AgentDir:              cm.KAconfigPath,
			ConfigFilePath:        "kubearmor.yaml",
			ServiceTemplateString: kubearmorServiceFile,
			ConfigTemplateString:  kubeArmorConfig,
			AgentImage:            cc.KubeArmorImage,
			ExtraFilePathSrc: map[string]string{
				"system_monitor.bpf.o": cc.SystemMonitorPath,
			},
			ExtraFilePathDest: map[string]string{
				"system_monitor.bpf.o": cm.KASystemMonitorPath,
			},
			LogRotate: cc.LogRotate,
		},
		{
			AgentName:             cm.RelayServer,
			PackageName:           cm.RelayServer,
			ServiceName:           cm.RelayServer + ".service",
			AgentDir:              cm.RelayServerConfigPath,
			ServiceTemplateString: relayServerServiceFile,
			AgentImage:            cc.KubeArmorRelayServerImage,
			LogRotate:             cc.LogRotate,
		},
		{
			AgentName:             cm.SpireAgent,
			PackageName:           cm.SpireAgent,
			ServiceName:           cm.SpireAgent + ".service",
			AgentDir:              cm.SpireConfigPath,
			ServiceTemplateString: spireAgentFile,
			ConfigFilePath:        "conf/agent/agent.conf",
			ConfigTemplateString:  spireAgentConfig,
			AgentImage:            cc.SPIREAgentImage,
			LogRotate:             cc.LogRotate,
		},
		{
			AgentName:                cm.PEAAgent,
			PackageName:              cm.PEAAgent,
			ServiceName:              cm.PEAAgent + ".service",
			AgentDir:                 cm.PEAconfigPath,
			ConfigFilePath:           "conf/application.yaml",
			ServiceTemplateString:    peaServiceFile,
			ConfigTemplateString:     peaConfig,
			KmuxConfigPath:           filepath.Join(cm.PEAconfigPath, cm.KmuxConfigFileName),
			KmuxConfigTemplateString: kmuxConfig,
			KmuxConfigFileName:       cm.KmuxConfigFileName,
			AgentImage:               cc.PEAImage,
			LogRotate:                cc.LogRotate,
		},
		{
			AgentName:                cm.FeederService,
			PackageName:              cm.FeederService,
			ServiceName:              cm.FeederService + ".service",
			AgentDir:                 cm.FSconfigPath,
			ConfigFilePath:           "conf/env",
			ServiceTemplateString:    feederServiceFile,
			ConfigTemplateString:     fsEnvVal,
			KmuxConfigPath:           filepath.Join(cm.FSconfigPath, cm.KmuxConfigFileName),
			KmuxConfigTemplateString: kmuxConfig,
			KmuxConfigFileName:       cm.KmuxConfigFileName,
			AgentImage:               cc.FeederImage,
			LogRotate:                cc.LogRotate,
		},
		{
			AgentName:                cm.SummaryEngine,
			PackageName:              cm.SummaryEngine,
			ServiceName:              cm.SummaryEngine + ".service",
			AgentDir:                 cm.SumEngineConfigPath,
			ConfigFilePath:           "conf/config.yaml",
			ServiceTemplateString:    sumEngineFile,
			ConfigTemplateString:     sumEngineConfig,
			AgentImage:               cc.SumEngineImage,
			KmuxConfigPath:           filepath.Join(cm.SumEngineConfigPath, cm.KmuxConfigFileName),
			KmuxConfigTemplateString: kmuxConfig,
			KmuxConfigFileName:       cm.KmuxConfigFileName,
			LogRotate:                cc.LogRotate,
		},
		{
			AgentName:                cm.DiscoverAgent,
			PackageName:              cm.DiscoverAgent,
			ServiceName:              cm.DiscoverAgent + ".service",
			AgentDir:                 cm.DiscoverConfigPath,
			ConfigFilePath:           "conf/config.yaml",
			ServiceTemplateString:    discoverFile,
			ConfigTemplateString:     discoverConfig,
			AgentImage:               cc.DiscoverImage,
			KmuxConfigPath:           filepath.Join(cm.DiscoverConfigPath, cm.KmuxConfigFileName),
			KmuxConfigTemplateString: kmuxConfig,
			KmuxConfigFileName:       cm.KmuxConfigFileName,
			LogRotate:                cc.LogRotate,
		},
		{
			AgentName:                cm.HardeningAgent,
			PackageName:              cm.HardeningAgent,
			ServiceName:              cm.HardeningAgent + ".service",
			AgentDir:                 cm.HardeningAgentConfigPath,
			ConfigFilePath:           "conf/config.yaml",
			ServiceTemplateString:    hardeningAgentFile,
			ConfigTemplateString:     hardeningAgentConfig,
			AgentImage:               cc.HardeningAgentImage,
			KmuxConfigPath:           filepath.Join(cm.HardeningAgentConfigPath, cm.KmuxConfigFileName),
			KmuxConfigTemplateString: kmuxConfig,
			KmuxConfigFileName:       cm.KmuxConfigFileName,
			LogRotate:                cc.LogRotate,
		},
		{
			AgentName:             cm.VMAdapter,
			PackageName:           cm.KubeArmorVMAdapter,
			ServiceName:           cm.KubeArmorVMAdapter + ".service",
			AgentDir:              cm.VmAdapterConfigPath,
			ServiceTemplateString: vmAdapterServiceFile,
			ConfigFilePath:        "vm-adapter-config.yaml",
			ConfigTemplateString:  vmAdapterConfig,
			AgentImage:            cc.KubeArmorVMAdapterImage,
			LogRotate:             cc.LogRotate,
		},
		{
			AgentName:                cm.SIAAgent,
			PackageName:              cm.SIAAgent,
			ServiceName:              cm.SIAAgent + ".service",
			AgentDir:                 cm.SIAconfigPath,
			ConfigFilePath:           "conf/app.yaml",
			ServiceTemplateString:    siaServiceFile,
			ConfigTemplateString:     siaConfig,
			KmuxConfigPath:           filepath.Join(cm.SIAconfigPath, cm.KmuxConfigFileName),
			KmuxConfigTemplateString: kmuxConfig,
			KmuxConfigFileName:       cm.KmuxConfigFileName,
			AgentImage:               cc.SIAImage,
			LogRotate:                cc.LogRotate,
		},
	}

	systemdObjects = append(systemdObjects, getSystemdAgentsKmuxConfigs(cc)...)

	// should be installed on control plane?
	for i, obj := range systemdObjects {
		if slices.Contains(workerNodeAgents, obj.AgentName) {
			systemdObjects[i].InstallOnWorkerNode = true
		}

		if obj.AgentName == cm.SummaryEngine && cc.WorkerNode && cc.DeploySumengine {
			systemdObjects[i].InstallOnWorkerNode = true
		}

		if obj.AgentName == cm.SpireAgent && cc.WorkerNode && cc.SpireEnabled {
			systemdObjects[i].InstallOnWorkerNode = true
		}

	}

	cc.SystemdServiceObjects = systemdObjects
	cc.LogRotateTemplateString = logRotateFile
}

func getSystemdAgentsKmuxConfigs(cc *ClusterConfig) []SystemdServiceObject {
	return []SystemdServiceObject{
		{
			AgentName:                cm.VMAdapter,
			AgentDir:                 cm.VmAdapterConfigPath,
			KmuxConfigPath:           filepath.Join(cm.VmAdapterConfigPath, cm.KmuxStateEventFileName),
			KmuxConfigTemplateString: kmuxPublisherConfig,
			KmuxConfigFileName:       cm.KmuxStateEventFileName,
			AgentImage:               cc.KubeArmorVMAdapterImage,
		},
		{
			AgentName:                cm.VMAdapter,
			AgentDir:                 cm.VmAdapterConfigPath,
			KmuxConfigPath:           filepath.Join(cm.VmAdapterConfigPath, cm.KmuxAlertsFileName),
			KmuxConfigTemplateString: kmuxPublisherConfig,
			KmuxConfigFileName:       cm.KmuxAlertsFileName,
			AgentImage:               cc.KubeArmorVMAdapterImage,
		},
		{
			AgentName:                cm.VMAdapter,
			AgentDir:                 cm.VmAdapterConfigPath,
			KmuxConfigPath:           filepath.Join(cm.VmAdapterConfigPath, cm.KmuxLogsFileName),
			KmuxConfigTemplateString: kmuxPublisherConfig,
			KmuxConfigFileName:       cm.KmuxLogsFileName,
			AgentImage:               cc.KubeArmorVMAdapterImage,
		},
		{
			AgentName:                cm.VMAdapter,
			AgentDir:                 cm.VmAdapterConfigPath,
			KmuxConfigPath:           filepath.Join(cm.VmAdapterConfigPath, cm.KmuxPoliciesFileName),
			KmuxConfigTemplateString: kmuxConsumerConfig,
			KmuxConfigFileName:       cm.KmuxPoliciesFileName,
			AgentImage:               cc.KubeArmorVMAdapterImage,
		},
		{
			AgentName:                cm.VMAdapter,
			AgentDir:                 cm.VmAdapterConfigPath,
			KmuxConfigPath:           filepath.Join(cm.VmAdapterConfigPath, cm.KmuxAnnotationFileName),
			KmuxConfigTemplateString: kmuxConsumerConfig,
			KmuxConfigFileName:       cm.KmuxAnnotationFileName,
			AgentImage:               cc.KubeArmorVMAdapterImage,
		},
		{
			AgentName:                cm.SummaryEngine,
			AgentDir:                 cm.SumEngineConfigPath,
			KmuxConfigPath:           filepath.Join(cm.SumEngineConfigPath, cm.KmuxSummaryFileName),
			KmuxConfigTemplateString: kmuxPublisherConfig,
			KmuxConfigFileName:       cm.KmuxSummaryFileName,
			AgentImage:               cc.SumEngineImage,
		},
		{
			AgentName:                cm.SummaryEngine,
			AgentDir:                 cm.SumEngineConfigPath,
			KmuxConfigPath:           filepath.Join(cm.SumEngineConfigPath, cm.KmuxAlertsFileName),
			KmuxConfigTemplateString: kmuxConsumerConfig,
			KmuxConfigFileName:       cm.KmuxAlertsFileName,
			AgentImage:               cc.SumEngineImage,
		},
		{
			AgentName:                cm.SummaryEngine,
			AgentDir:                 cm.SumEngineConfigPath,
			KmuxConfigPath:           filepath.Join(cm.SumEngineConfigPath, cm.KmuxLogsFileName),
			KmuxConfigTemplateString: kmuxConsumerConfig,
			KmuxConfigFileName:       cm.KmuxLogsFileName,
			AgentImage:               cc.SumEngineImage,
		},
		{
			AgentName:                cm.DiscoverAgent,
			AgentDir:                 cm.DiscoverConfigPath,
			KmuxConfigPath:           filepath.Join(cm.DiscoverConfigPath, cm.KmuxSummaryFileName),
			KmuxConfigTemplateString: kmuxConsumerConfig,
			KmuxConfigFileName:       cm.KmuxSummaryFileName,
			AgentImage:               cc.DiscoverImage,
		},
		{
			AgentName:                cm.DiscoverAgent,
			AgentDir:                 cm.DiscoverConfigPath,
			KmuxConfigPath:           filepath.Join(cm.DiscoverConfigPath, cm.KmuxPolicyFileName),
			KmuxConfigTemplateString: kmuxPublisherConfig,
			KmuxConfigFileName:       cm.KmuxPolicyFileName,
			AgentImage:               cc.DiscoverImage,
		},
		{
			AgentName:                cm.HardeningAgent,
			AgentDir:                 cm.HardeningAgentConfigPath,
			KmuxConfigPath:           filepath.Join(cm.HardeningAgentConfigPath, cm.KmuxSummaryFileName),
			KmuxConfigTemplateString: kmuxConsumerConfig,
			KmuxConfigFileName:       cm.KmuxSummaryFileName,
			AgentImage:               cc.HardeningAgentImage,
		},
		{
			AgentName:                cm.HardeningAgent,
			AgentDir:                 cm.HardeningAgentConfigPath,
			KmuxConfigPath:           filepath.Join(cm.HardeningAgentConfigPath, cm.KmuxPolicyFileName),
			KmuxConfigTemplateString: kmuxPublisherConfig,
			KmuxConfigFileName:       cm.KmuxPolicyFileName,
			AgentImage:               cc.HardeningAgentImage,
		},
		{
			AgentName:                cm.PEAAgent,
			AgentDir:                 cm.PEAconfigPath,
			KmuxConfigPath:           filepath.Join(cm.PEAconfigPath, cm.KmuxPoliciesFileName),
			KmuxConfigTemplateString: kmuxConsumerConfig,
			KmuxConfigFileName:       cm.KmuxPoliciesFileName,
			AgentImage:               cc.PEAImage,
		},
		{
			AgentName:                cm.PEAAgent,
			AgentDir:                 cm.PEAconfigPath,
			KmuxConfigPath:           filepath.Join(cm.PEAconfigPath, cm.KmuxStateEventFileName),
			KmuxConfigTemplateString: kmuxPublisherConfig,
			KmuxConfigFileName:       cm.KmuxStateEventFileName,
			AgentImage:               cc.PEAImage,
		},
		{
			AgentName:                cm.FeederService,
			AgentDir:                 cm.FSconfigPath,
			KmuxConfigPath:           filepath.Join(cm.FSconfigPath, cm.KmuxAlertsFileName),
			KmuxConfigTemplateString: kmuxConsumerConfig,
			KmuxConfigFileName:       cm.KmuxAlertsFileName,
			AgentImage:               cc.FeederImage,
		},
		{
			AgentName:                cm.FeederService,
			AgentDir:                 cm.FSconfigPath,
			KmuxConfigPath:           filepath.Join(cm.FSconfigPath, cm.KmuxLogsFileName),
			KmuxConfigTemplateString: kmuxConsumerConfig,
			KmuxConfigFileName:       cm.KmuxLogsFileName,
			AgentImage:               cc.FeederImage,
		},
		{
			AgentName:                cm.FeederService,
			AgentDir:                 cm.FSconfigPath,
			KmuxConfigPath:           filepath.Join(cm.FSconfigPath, cm.KmuxSummaryFileName),
			KmuxConfigTemplateString: kmuxConsumerConfig,
			KmuxConfigFileName:       cm.KmuxSummaryFileName,
			AgentImage:               cc.FeederImage,
		},
		{
			AgentName:                cm.FeederService,
			AgentDir:                 cm.FSconfigPath,
			KmuxConfigPath:           filepath.Join(cm.FSconfigPath, cm.KmuxPolicyFileName),
			KmuxConfigTemplateString: kmuxConsumerConfig,
			KmuxConfigFileName:       cm.KmuxPolicyFileName,
			AgentImage:               cc.FeederImage,
		},
	}
}

// placeServiceFiles copies service files
func (cc *ClusterConfig) placeServiceFiles() error {

	configArgs := map[string]any{
		"WorkerNode":     cc.WorkerNode,
		"SystemdVersion": getSystemdVersion(),
		"ReleaseVersion": cc.AgentsVersion,
		"ProxyEnabled":   cc.Proxy.Enabled,
		"ProxyAddress":   cc.Proxy.Address,
		"ProxySaaSAddr":  cc.Proxy.SaaSAddr,
		"ProxyExtraArgs": cc.Proxy.ExtraArgs,
		"JoinToken":      cc.JoinToken,
	}

	if cc.Proxy.Address != "" {
		configArgs["ProxyEnabled"] = true
	}

	if cc.AdditionalArgs != nil {
		maps.Copy(configArgs, cc.AdditionalArgs)
	}

	for _, obj := range cc.SystemdServiceObjects {
		if cc.WorkerNode && !obj.InstallOnWorkerNode {
			continue
		}
		if obj.AgentName == cm.SummaryEngine && cc.WorkerNode && !cc.DeploySumengine {
			continue
		}
		if obj.AgentName == cm.HardeningAgent && semver.Compare(cc.AgentsVersion, "v0.9.4") >= 0 {
			continue
		}

		configArgs["CPUQuota"] = fmt.Sprintf("%v%s", cc.AgentsResource.CPUQuota, "%")
		configArgs["MemoryMax"] = fmt.Sprintf("%vM", cc.AgentsResource.MemoryMax)
		configArgs["MemoryHigh"] = fmt.Sprintf("%vM", cc.AgentsResource.MemoryHigh)

		if obj.AgentName == cm.KubeArmor {
			configArgs["CPUQuota"] = fmt.Sprintf("%v%s", cc.KaResource.CPUQuota, "%")
			configArgs["MemoryMax"] = fmt.Sprintf("%vM", cc.KaResource.MemoryMax)
			configArgs["MemoryHigh"] = fmt.Sprintf("%vM", cc.KaResource.MemoryHigh)
		}

		if obj.ServiceTemplateString != "" {

			if obj.AgentName == cm.RRA {
				//place service file for RRA
				cc.TemplateFuncs = sprig.GenericFuncMap()
				_, err := copyOrGenerateFile("", cm.SystemdDir, obj.ServiceName, cc.TemplateFuncs, obj.ServiceTemplateString, configArgs)
				if err != nil {
					return err
				}
				//place timer file for RRA
				_, err = copyOrGenerateFile("", cm.SystemdDir, "accuknox-rra.timer", cc.TemplateFuncs, obj.TimerTemplateString, configArgs)
				if err != nil {
					return err
				}

			} else {
				_, err := copyOrGenerateFile("", cm.SystemdDir, obj.ServiceName, cc.TemplateFuncs, obj.ServiceTemplateString, configArgs)
				if err != nil {
					return err
				}

				if cc.LogRotateTemplateString != "" {
					logRotate := map[string]interface{}{
						"AgentDir":    obj.AgentDir,
						"PackageName": obj.PackageName,
						"LogRotate":   obj.LogRotate,
					}
					_, err = copyOrGenerateFile("", cm.LogrotateDir, obj.PackageName, cc.TemplateFuncs, cc.LogRotateTemplateString, logRotate)
					if err != nil {
						fmt.Println(err.Error())
						return err
					}
					logRotate["PackageName"] = obj.PackageName + "-err"
					_, err = copyOrGenerateFile("", cm.LogrotateDir, obj.PackageName+"-err", cc.TemplateFuncs, cc.LogRotateTemplateString, logRotate)
					if err != nil {
						fmt.Println(err.Error())
						return err
					}
				}
			}
		}
	}

	return nil
}

// downloadAgent downloads agents as OCI artifacts
func (cc *ClusterConfig) downloadAgent(agentName, agentRepo, agentTag string) (string, error) {
	fs, err := file.New(cm.DownloadDir)
	if err != nil {
		return "", err
	}
	defer fs.Close()

	// 1. Connect to a remote repository
	ctx := context.Background()
	repo, err := remote.NewRepository(agentRepo)
	if err != nil {
		return "", err
	}

	repo.Client = cc.ORASClient
	repo.PlainHTTP = cc.PlainHTTP

	_, err = oras.Copy(ctx, repo, agentTag, fs, agentTag, oras.DefaultCopyOptions)
	if err != nil {
		return "", err
	}

	filepath := path.Join(cm.DownloadDir, agentName+"_"+agentTag+".tar.gz")
	return filepath, nil
}

// extractAgent extracts agent tar
func extractAgent(fileName string) error {
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

// InstallAgent downloads agent using downloadAgent.
// It disables the systemd service first if it is running
func (cc *ClusterConfig) installAgent(agentName, agentRepo, agentTag string) error {
	fileName, err := cc.downloadAgent(agentName, agentRepo, agentTag)
	if err != nil {
		return err
	}

	err = extractAgent(fileName)
	if err != nil {
		return err
	}

	return nil
}

// SystemdInstall downloads agents and installs them.
// It doesn't deal with custom configuration
func (cc *ClusterConfig) SystemdInstall() error {
	// Verify BTF installation
	btfPresent, err := verifyBTF()
	if (cc.SkipBTFCheck && cc.SystemMonitorPath != "") || cc.SystemMonitorPath != "" {
		// skip explicitly specified with system monitor OR system monitor specified
		logger.Warn("Skipping BTF check. Using system monitor at: %s", cc.SystemMonitorPath)
	} else if cc.SkipBTFCheck {
		// we don't care about system monitor or BTF
		logger.Warn("Skipping BTF check...")
	} else {
		// BTF not present, we need to fail
		if err != nil {
			return fmt.Errorf("failed to look for BTF info: %s", err.Error())
		} else if !btfPresent {
			return fmt.Errorf("BTF info not found. Please use '--skip-btf-check' if you want to force install")
		}
	}

	for _, obj := range cc.SystemdServiceObjects {
		// skip installing on worker node
		if cc.WorkerNode && !obj.InstallOnWorkerNode {
			continue
		}
		if obj.AgentImage == "" || obj.PackageName == "" {
			continue
		}

		if cc.Proxy.Enabled && obj.AgentName == cm.SpireAgent {
			continue
		}

		// stop existing service first otherwise errors are encountered due to
		// busy binary
		err := StopSystemdService(obj.ServiceName, true, false)
		if err != nil {
			logger.Warn("Failed to stop existing systemd service %s: %s", obj.ServiceName, err.Error())
		}

		if obj.AgentName == cm.SummaryEngine && cc.WorkerNode && !cc.DeploySumengine {
			continue
		}

		if obj.AgentName == cm.HardeningAgent && !cc.EnableHardeningAgent {
			continue
		}

		if obj.AgentName == cm.DiscoverAgent && !cc.DeployDiscover {
			continue
		}

		logger.Print("Downloading Agent - %s | Image - %s", obj.AgentName, obj.AgentImage)
		packageMeta := splitLast(obj.AgentImage, ":")

		err = cc.installAgent(obj.AgentName, packageMeta[0], packageMeta[1])
		if err != nil {
			//fmt.Println(err)
			return err
		}

		logger.Print("%s version %s downloaded successfully\n", obj.AgentName, packageMeta[1])
	}

	err = cc.placeServiceFiles()
	if err != nil {
		return err
	}

	logger.PrintSuccess("All agents downloaded successfully.")

	return nil
}

func StartSystemdService(serviceName string) error {
	if serviceName == "" {
		return nil
	}
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

	// Start the service
	ch := make(chan string)
	if _, err := conn.RestartUnitContext(ctx, serviceName, "replace", ch); err != nil {
		return fmt.Errorf("failed to start %s: %v", serviceName, err)
	}
	logger.Print("Started %s", serviceName)

	return nil
}

func StopSystemdService(serviceName string, skipDeleteDisable, force bool) error {
	ctx := context.Background()
	conn, err := dbus.NewWithContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to systemd: %v", err)
	}
	defer conn.Close()

	stopChan := make(chan string)

	property, err := conn.GetUnitPropertyContext(ctx, serviceName, "ActiveState")
	if err != nil {
		return fmt.Errorf("Failed to check service status: %s", err.Error())
	}

	// service not active, return
	if property.Value.Value() != "active" && !force {
		return nil
	}

	if _, err := conn.StopUnitContext(ctx, serviceName, "replace", stopChan); err != nil {
		if !strings.Contains(err.Error(), "not loaded") {
			return fmt.Errorf("Failed to stop existing %s service: %v\n", serviceName, err)
		}
	} else {
		logger.Info1("Stopping existing %s...", serviceName)
		<-stopChan
		logger.Info1("%s stopped successfully.", serviceName)
	}

	if !skipDeleteDisable {
		if _, err := conn.DisableUnitFilesContext(ctx, []string{serviceName}, false); err != nil {
			if !strings.Contains(err.Error(), "does not exist") {
				logger.Error("Failed to disable %s : %v", serviceName, err)
				return err
			}
		} else {
			logger.Info1("Disabled %s", serviceName)
		}

		svcFilePath := cm.SystemdDir + serviceName
		if err := os.Remove(svcFilePath); err != nil {
			if !os.IsNotExist(err) {
				logger.Error("Failed to delete %s file: %v", serviceName, err)
				return err
			}
		}

		// reload systemd config, equivalent to systemctl daemon-reload
		if err := conn.ReloadContext(ctx); err != nil {
			return fmt.Errorf("failed to reload systemd configuration: %v", err)
		}
	}

	return nil
}

func Deletedir(dirName string) {
	//	Clean Up
	err := os.RemoveAll(dirName)
	if err != nil && !os.IsNotExist(err) {
		// Check if the error is due to the directory not existing
		logger.Error("error deleting %s : %v", dirName, err)
	}
}

func DeboardSystemd(nodeType NodeType) error {
	pseudoCC := new(ClusterConfig)
	pseudoCC.CreateSystemdServiceObjects()

	for _, obj := range pseudoCC.SystemdServiceObjects {
		if obj.ServiceName == "" {
			continue
		}
		err := StopSystemdService(obj.ServiceName, false, true)
		if err != nil {
			logger.Error("error stopping %s: %s", obj.ServiceName, err)
			return err
		}

		Deletedir(obj.AgentDir)
		// Delete Logrotate Files
		Deletedir(cm.LogrotateDir + obj.PackageName)
	}

	knoxctlDir := filepath.Clean(filepath.Join(cm.SystemdKnoxctlDir, cm.KnoxctlConfigFilename))
	err := os.Remove(knoxctlDir)
	if err != nil {
		logger.Warn("Failed to remove dir %s: %s", knoxctlDir, err.Error())
	}

	return nil
}

func CheckInstalledSystemdServices() ([]string, error) {
	allAgents := []string{"kubearmor", cm.KubeArmorVMAdapter, cm.RelayServer, cm.PEAAgent, cm.SIAAgent, cm.FeederService, cm.SpireAgent, cm.SummaryEngine, cm.DiscoverAgent, cm.HardeningAgent}
	installedAgents := make([]string, 0)

	for _, agent := range allAgents {
		filePath := cm.SystemdPath + agent + ".service"
		if _, err := os.Stat(filePath); err == nil {
			// found service file means we have agents as systemd service
			installedAgents = append(installedAgents, agent)
		} else if !os.IsNotExist(err) {
			logger.Warn("Error checking service file %s: %v", filePath, err)
			continue
		}
	}

	return installedAgents, nil
}
func InstallAgent(agentName, agentRepo, agentTag string) error {
	fileName, err := DownloadAgent(agentName, agentRepo, agentTag)
	if err != nil {
		return err
	}

	err = extractAgent(fileName)
	if err != nil {
		return err
	}

	return nil
}

// downloadAgent downloads agents as OCI artifacts
func DownloadAgent(agentName, agentRepo, agentTag string) (string, error) {
	fs, err := file.New(cm.DownloadDir)
	if err != nil {
		return "", err
	}
	defer fs.Close()

	// 1. Connect to a remote repository
	ctx := context.Background()
	repo, err := remote.NewRepository(agentRepo)
	if err != nil {
		return "", err
	}

	// repo.Client = cc.ORASClient
	// repo.PlainHTTP = cc.PlainHTTP

	_, err = oras.Copy(ctx, repo, agentTag, fs, agentTag, oras.DefaultCopyOptions)
	if err != nil {
		return "", err
	}

	filepath := path.Join(cm.DownloadDir, agentName+"_"+agentTag+".tar.gz")
	return filepath, nil
}

// wraps around journalctl
// could've used bindings from systemd-go but they require CGO, adding which
// would make it impossible to run knoxctl in any environment
func runJournalCTLCommand(args ...string) ([]byte, error) {
	journalCTLCommand := exec.Command("journalctl", args...)

	data, err := journalCTLCommand.CombinedOutput()
	if err != nil {
		return nil, err
	}

	return data, nil
}

func DumpSystemdLogs(sysdumpDir string, services []string) {
	for _, service := range services {
		logs, err := runJournalCTLCommand("--no-pager", "-u", service)
		if err != nil {
			logger.Warn("Error while getting logs from %s: %s", service, err.Error())
		} else {
			filename := filepath.Join(sysdumpDir, service+".log")
			err := os.WriteFile(filename, logs, 0644) // #nosec G306 need perms for archiving
			if err != nil {
				logger.Warn("Error while writing logs to file %s: %s", filename, err.Error())
				continue
			}
		}
	}
}

// takes in full paths, reads files from source recursively and dumps them
// to destination
func readAndDumpDir(sourceDirPath, destDirPath string) error {
	err := filepath.WalkDir(sourceDirPath, func(path string, d fs.DirEntry, err error) error {
		if d == nil {
			return nil
		}

		relativePath, err := filepath.Rel(sourceDirPath, path)
		if err != nil {
			return err
		}

		var sysdumpFullPath string
		sysdumpFullPath = filepath.Join(destDirPath, relativePath)

		if d.Name() == "/" || relativePath == "/" {
			return fmt.Errorf("Invalid path %s", d.Name())
		}

		if d.IsDir() {
			errMkdir := os.MkdirAll(sysdumpFullPath, 0755) // #nosec G301 perms needed for archiving
			if errMkdir != nil {
				return errMkdir
			}

			return nil
		}

		fileinfo, err := d.Info()
		if err != nil {
			return err
		}

		// skip binaries etc
		if !d.Type().IsRegular() || strings.Contains(fileinfo.Mode().String(), "x") {
			logger.Warn("Skipping file %s", path)
			return nil
		}

		fileContent, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return err
		}

		if len(fileContent) == 0 {
			logger.Warn("Skipping empty file %s", path)
			return nil
		}

		err = os.WriteFile(sysdumpFullPath, fileContent, 0644) // #nosec G306 need perms for archiving
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// takes in full paths and copies the given file
func readAndDumpFile(sourceFilePath, destFilePath string) error {
	if sourceFilePath == "/" || destFilePath == "/" {
		return fmt.Errorf("Invalid path '\\/'")
	}

	_, err := os.Stat(sourceFilePath)
	if err != nil {
		return err
	}

	sourceFileContent, err := os.ReadFile(filepath.Clean(sourceFilePath))
	if err != nil {
		return err
	}

	err = os.WriteFile(destFilePath, sourceFileContent, 0644) // #nosec G306 perms needed for archiving
	if err != nil {
		return err
	}

	return nil
}

func DumpSystemdAgentInstallation(sysdumpDir string) {
	pseudoCC := new(ClusterConfig)
	pseudoCC.CreateSystemdServiceObjects()

	for _, service := range pseudoCC.SystemdServiceObjects {
		if service.AgentDir != "" {
			if err := readAndDumpDir(service.AgentDir, filepath.Join(sysdumpDir, service.AgentName)); err != nil && !os.IsNotExist(err) {
				logger.Warn("Failed to copy files form %s to %s: %s", service.AgentDir, sysdumpDir, err.Error())
			}
		}

		if service.ServiceName != "" {
			systemdServicePath := filepath.Join(cm.SystemdDir, service.ServiceName)
			sysdumpServicePath := filepath.Join(sysdumpDir, service.ServiceName)
			if err := readAndDumpFile(systemdServicePath, sysdumpServicePath); err != nil && !os.IsNotExist(err) {
				logger.Warn("Failed to copy files form %s to %s: %s", systemdServicePath, sysdumpServicePath, err.Error())
			}
		}
	}
}

func DumpSystemdKnoxctlDir(sysdumpDir string) {
	knoxctlDir := "/opt/knoxctl"
	if err := readAndDumpDir(knoxctlDir, filepath.Join(sysdumpDir, "knoxctl")); err != nil {
		logger.Warn("Failed to copy files form %s to %s: %s", knoxctlDir, sysdumpDir, err.Error())
	}
}

// ConvertCronToSystemd converts a crontab schedule into a systemd timer schedule format.
func ConvertCronToSystemd(schedule string) (string, error) {

	fields := strings.Fields(schedule)

	// Validate that the schedule has exactly 5 fields.
	if len(fields) != 5 {
		return "", fmt.Errorf("invalid schedule format: expected 5 fields (minute, hour, day, month, weekday)")
	}

	weekDays := map[string]string{
		"0": "Sun",
		"1": "Mon",
		"2": "Tue",
		"3": "Wed",
		"4": "Thu",
		"5": "Fri",
		"6": "Sat",
		"7": "Sun",
	}

	minute := fields[0]
	hour := fields[1]
	day := fields[2]
	month := fields[3]
	weekDay := weekDays[fields[4]]

	// Construct the systemd schedule using the template.
	scheduleTemplate := "%s *-%s-%s %s:%s:00"
	finalSchedule := fmt.Sprintf(scheduleTemplate, weekDay, month, day, hour, minute)

	return finalSchedule, nil
}
func GetSystemdServiceStatus(name string) (string, error) {
	ctx := context.Background()
	status := ""
	// Connect to systemd dbus
	conn, err := dbus.NewWithContext(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to connect to systemd: %v", err)
	}
	defer conn.Close()

	props, err := conn.GetUnitPropertiesContext(ctx, name)
	if err != nil {
		return "", fmt.Errorf("failed to get properties for service %s: %v", name, err)
	}
	status, ok := props["ActiveState"].(string)
	if !ok {
		return "", fmt.Errorf("could not interpret ActiveState for %s", name)
	}

	return status, nil
}
