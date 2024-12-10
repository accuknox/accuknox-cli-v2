package onboard

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Masterminds/sprig"
	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/fatih/color"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
)

var (
	workerNodeAgents = []string{cm.KubeArmor, cm.VMAdapter, cm.SummaryEngine}
	cpNodeAgents     = []string{cm.SpireAgent, cm.SIAAgent, cm.PEAAgent, cm.FeederService, cm.SummaryEngine, cm.DiscoverAgent, cm.HardeningAgent}
)

func (cc *ClusterConfig) createSystemdServiceObjects() {
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
		},
		{
			AgentName:             cm.RelayServer,
			PackageName:           cm.RelayServer,
			ServiceName:           cm.RelayServer + ".service",
			AgentDir:              cm.RelayServerConfigPath,
			ServiceTemplateString: relayServerServiceFile,
			AgentImage:            cc.KubeArmorRelayServerImage,
		},
		{
			AgentName:            cm.SpireAgent,
			PackageName:          cm.SpireAgent,
			ServiceName:          cm.SpireAgent + ".service",
			AgentDir:             cm.SpireConfigPath,
			ConfigFilePath:       "conf/agent/agent.conf",
			ConfigTemplateString: spireAgentConfig,
			AgentImage:           cc.SPIREAgentImage,
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
		},
	}

	systemdObjects = append(systemdObjects, getSystemdAgentsKmuxConfigs()...)

	// should be installed on control plane?
	for i, obj := range systemdObjects {
		if slices.Contains(workerNodeAgents, obj.AgentName) {
			systemdObjects[i].InstallOnWorkerNode = true
		}

		if obj.AgentName == cm.SummaryEngine && cc.WorkerNode && cc.DeploySumengine {
			systemdObjects[i].InstallOnWorkerNode = true
		}
	}

	cc.SystemdServiceObjects = systemdObjects
}

func getSystemdAgentsKmuxConfigs() []SystemdServiceObject {
	return []SystemdServiceObject{
		{
			AgentName:                cm.VMAdapter,
			AgentDir:                 cm.VmAdapterConfigPath,
			KmuxConfigPath:           filepath.Join(cm.VmAdapterConfigPath, cm.KmuxStateEventFileName),
			KmuxConfigTemplateString: kmuxPublisherConfig,
			KmuxConfigFileName:       cm.KmuxStateEventFileName,
		},
		{
			AgentName:                cm.VMAdapter,
			AgentDir:                 cm.VmAdapterConfigPath,
			KmuxConfigPath:           filepath.Join(cm.VmAdapterConfigPath, cm.KmuxAlertsFileName),
			KmuxConfigTemplateString: kmuxPublisherConfig,
			KmuxConfigFileName:       cm.KmuxAlertsFileName,
		},
		{
			AgentName:                cm.VMAdapter,
			AgentDir:                 cm.VmAdapterConfigPath,
			KmuxConfigPath:           filepath.Join(cm.VmAdapterConfigPath, cm.KmuxLogsFileName),
			KmuxConfigTemplateString: kmuxPublisherConfig,
			KmuxConfigFileName:       cm.KmuxLogsFileName,
		},
		{
			AgentName:                cm.VMAdapter,
			AgentDir:                 cm.VmAdapterConfigPath,
			KmuxConfigPath:           filepath.Join(cm.VmAdapterConfigPath, cm.KmuxPoliciesFileName),
			KmuxConfigTemplateString: kmuxConsumerConfig,
			KmuxConfigFileName:       cm.KmuxPoliciesFileName,
		},
		{
			AgentName:                cm.SummaryEngine,
			AgentDir:                 cm.SumEngineConfigPath,
			KmuxConfigPath:           filepath.Join(cm.SumEngineConfigPath, cm.KmuxSummaryFileName),
			KmuxConfigTemplateString: kmuxPublisherConfig,
			KmuxConfigFileName:       cm.KmuxSummaryFileName,
		},
		{
			AgentName:                cm.SummaryEngine,
			AgentDir:                 cm.SumEngineConfigPath,
			KmuxConfigPath:           filepath.Join(cm.SumEngineConfigPath, cm.KmuxAlertsFileName),
			KmuxConfigTemplateString: kmuxConsumerConfig,
			KmuxConfigFileName:       cm.KmuxAlertsFileName,
		},
		{
			AgentName:                cm.SummaryEngine,
			AgentDir:                 cm.SumEngineConfigPath,
			KmuxConfigPath:           filepath.Join(cm.SumEngineConfigPath, cm.KmuxLogsFileName),
			KmuxConfigTemplateString: kmuxConsumerConfig,
			KmuxConfigFileName:       cm.KmuxLogsFileName,
		},
		{
			AgentName:                cm.DiscoverAgent,
			AgentDir:                 cm.DiscoverConfigPath,
			KmuxConfigPath:           filepath.Join(cm.DiscoverConfigPath, cm.KmuxSummaryFileName),
			KmuxConfigTemplateString: kmuxConsumerConfig,
			KmuxConfigFileName:       cm.KmuxSummaryFileName,
		},
		{
			AgentName:                cm.DiscoverAgent,
			AgentDir:                 cm.DiscoverConfigPath,
			KmuxConfigPath:           filepath.Join(cm.DiscoverConfigPath, cm.KmuxPolicyFileName),
			KmuxConfigTemplateString: kmuxPublisherConfig,
			KmuxConfigFileName:       cm.KmuxPolicyFileName,
		},
		{
			AgentName:                cm.HardeningAgent,
			AgentDir:                 cm.HardeningAgentConfigPath,
			KmuxConfigPath:           filepath.Join(cm.HardeningAgentConfigPath, cm.KmuxSummaryFileName),
			KmuxConfigTemplateString: kmuxConsumerConfig,
			KmuxConfigFileName:       cm.KmuxSummaryFileName,
		},
		{
			AgentName:                cm.HardeningAgent,
			AgentDir:                 cm.HardeningAgentConfigPath,
			KmuxConfigPath:           filepath.Join(cm.HardeningAgentConfigPath, cm.KmuxPolicyFileName),
			KmuxConfigTemplateString: kmuxPublisherConfig,
			KmuxConfigFileName:       cm.KmuxPolicyFileName,
		},
		{
			AgentName:                cm.PEAAgent,
			AgentDir:                 cm.PEAconfigPath,
			KmuxConfigPath:           filepath.Join(cm.PEAconfigPath, cm.KmuxPoliciesFileName),
			KmuxConfigTemplateString: kmuxConsumerConfig,
			KmuxConfigFileName:       cm.KmuxPoliciesFileName,
		},
		{
			AgentName:                cm.PEAAgent,
			AgentDir:                 cm.PEAconfigPath,
			KmuxConfigPath:           filepath.Join(cm.PEAconfigPath, cm.KmuxStateEventFileName),
			KmuxConfigTemplateString: kmuxPublisherConfig,
			KmuxConfigFileName:       cm.KmuxStateEventFileName,
		},
		{
			AgentName:                cm.FeederService,
			AgentDir:                 cm.FSconfigPath,
			KmuxConfigPath:           filepath.Join(cm.FSconfigPath, cm.KmuxAlertsFileName),
			KmuxConfigTemplateString: kmuxConsumerConfig,
			KmuxConfigFileName:       cm.KmuxAlertsFileName,
		},
		{
			AgentName:                cm.FeederService,
			AgentDir:                 cm.FSconfigPath,
			KmuxConfigPath:           filepath.Join(cm.FSconfigPath, cm.KmuxLogsFileName),
			KmuxConfigTemplateString: kmuxConsumerConfig,
			KmuxConfigFileName:       cm.KmuxLogsFileName,
		},
		{
			AgentName:                cm.FeederService,
			AgentDir:                 cm.FSconfigPath,
			KmuxConfigPath:           filepath.Join(cm.FSconfigPath, cm.KmuxSummaryFileName),
			KmuxConfigTemplateString: kmuxConsumerConfig,
			KmuxConfigFileName:       cm.KmuxSummaryFileName,
		},
		{
			AgentName:                cm.FeederService,
			AgentDir:                 cm.FSconfigPath,
			KmuxConfigPath:           filepath.Join(cm.FSconfigPath, cm.KmuxPolicyFileName),
			KmuxConfigTemplateString: kmuxConsumerConfig,
			KmuxConfigFileName:       cm.KmuxPolicyFileName,
		},
	}
}

// placeServiceFiles copies service files
func (cc *ClusterConfig) placeServiceFiles() error {
	for _, obj := range cc.SystemdServiceObjects {
		if cc.WorkerNode && !obj.InstallOnWorkerNode {
			continue
		}

		if obj.ServiceTemplateString != "" {

			if obj.AgentName == cm.RAT {
				//place service file for RAT
				cc.TemplateFuncs = sprig.GenericFuncMap()
				_, err := copyOrGenerateFile("", cm.SystemdDir, obj.ServiceName, cc.TemplateFuncs, obj.ServiceTemplateString, cc.RATConfigObject)
				if err != nil {
					return err
				}
				//place timer file for RAT
				_, err = copyOrGenerateFile("", cm.SystemdDir, "accuknox-rat.timer", cc.TemplateFuncs, obj.TimerTemplateString, cc.RATConfigObject)
				if err != nil {
					return err
				}

			} else {
				_, err := copyOrGenerateFile("", cm.SystemdDir, obj.ServiceName, cc.TemplateFuncs, obj.ServiceTemplateString, interface{}(nil))
				if err != nil {
					return err
				}
			}

		}
	}

	return nil
}

// downloadAgent downloads agents as OCI artifiacts
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
		// skip explicitly specified with system monitor OR system montior specified
		fmt.Println(color.YellowString("Skipping BTF check. Using system monitor at: %s", cc.SystemMonitorPath))
	} else if cc.SkipBTFCheck {
		// we don't care about system monitor or BTF
		fmt.Println(color.YellowString("Skipping BTF check..."))
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
		if obj.AgentImage == "" {
			continue
		}

		// stop existing service first otherwise errors are encountered due to
		// busy binary
		err := StopSystemdService(obj.ServiceName, true, false)
		if err != nil {
			fmt.Println(color.YellowString("Failed to stop existing systemd service %s: %s", obj.ServiceName, err.Error()))
		}

		fmt.Printf(color.CyanString("Downloading Agent - %s | Image - %s\n", obj.AgentName, obj.AgentImage))
		packageMeta := splitLast(obj.AgentImage, ":")

		err = cc.installAgent(obj.AgentName, packageMeta[0], packageMeta[1])
		if err != nil {
			//fmt.Println(err)
			return err
		}

		fmt.Printf(color.CyanString("%s version %s downloaded successfully\n\n", obj.AgentName, packageMeta[1]))
	}

	err = cc.placeServiceFiles()
	if err != nil {
		//fmt.Println(err)
		return err
	}

	// copy custom system monitor
	/*
		if cc.SystemMonitorPath != "" {
			targetPath := filepath.Join(cm.KAconfigPath, "BPF")
			_, err = copyOrGenerateFile(cc.SystemMonitorPath, targetPath, "system_monitor.bpf.o", nil, "", nil)
			if err != nil {
				return fmt.Errorf("Failed to copy custom system monitor: %s", err.Error())
			}
		}
	*/

	fmt.Println(color.GreenString("All agents downloaded successfully."))

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
	fmt.Printf(color.CyanString("Started %s\n", serviceName))

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
		fmt.Printf(color.BlueString("Stopping existing %s...\n", serviceName))
		<-stopChan
		fmt.Printf(color.BlueString("%s stopped successfully.\n", serviceName))
	}

	if !skipDeleteDisable {
		if _, err := conn.DisableUnitFilesContext(ctx, []string{serviceName}, false); err != nil {
			if !strings.Contains(err.Error(), "does not exist") {
				fmt.Printf("Failed to disable %s : %v\n", serviceName, err)
				return err
			}
		} else {
			fmt.Printf("Disabled %s\n", serviceName)
		}

		svcfilePath := cm.SystemdDir + serviceName
		if err := os.Remove(svcfilePath); err != nil {
			if !os.IsNotExist(err) {
				fmt.Printf("Failed to delete %s file: %v\n", serviceName, err)
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
		fmt.Printf("error deleting %s : %v\n", dirName, err)
	}
}

func DeboardSystemd(nodeType NodeType) error {
	pseudoCC := new(ClusterConfig)
	pseudoCC.createSystemdServiceObjects()

	for _, obj := range pseudoCC.SystemdServiceObjects {
		if obj.ServiceName == "" {
			continue
		}
		err := StopSystemdService(obj.ServiceName, false, true)
		if err != nil {
			fmt.Printf("error stopping %s: %s\n", obj.ServiceName, err)
			return err
		}

		Deletedir(obj.AgentDir)
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
			fmt.Println(color.YellowString("Error checking service file %s: %v", filePath, err))
			continue
		}
	}

	return installedAgents, nil
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
