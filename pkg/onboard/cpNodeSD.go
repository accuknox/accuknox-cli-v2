package onboard

import (
	"fmt"
	"path/filepath"

	"github.com/Masterminds/sprig"
	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/fatih/color"
)

func (ic *InitConfig) InitializeControlPlaneSD() error {
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

	ic.TCArgs.DiscoverRules = combineVisibilities(ic.Visibility, ic.HostVisibility)

	// initialize sprig for templating
	ic.TemplateFuncs = sprig.GenericFuncMap()

	// download and extract systemd packages
	fmt.Println(color.MagentaString("Downloading agents..."))
	err := ic.SystemdInstall()
	if err != nil {
		fmt.Println(color.RedString("Installation failed!! Cleaning up downloaded assets..."))
		// ignoring G104 - can't send nil in installation failed case
		Deletedir(cm.DownloadDir)
		DeboardSystemd(NodeType_ControlPlane) // #nosec G104
		return err
	}

	// copy config files according to custom configuration specified by the user

	kmuxConfigArgs := KmuxConfigTemplateArgs{
		ReleaseVersion: ic.AgentsVersion,
		StreamName:     "knox-gateway",
		ServerURL:      ic.KnoxGateway,
	}

	if ic.RMQServer != "" {
		ic.TCArgs.RMQAddr = ic.RMQServer
		kmuxConfigArgs.RMQServer = ic.RMQServer
	} else if ic.CPNodeAddr != "" {
		ic.TCArgs.RMQAddr = ic.CPNodeAddr + ":5672"
		kmuxConfigArgs.RMQServer = ic.CPNodeAddr + ":5672"
	} else {
		ic.TCArgs.RMQAddr = "0.0.0.0:5672"
		kmuxConfigArgs.RMQServer = "0.0.0.0:5672"
	}

	fmt.Println(color.MagentaString("\nConfiguring services..."))
	for _, obj := range ic.SystemdServiceObjects {
		// copy generic config files
		if obj.ConfigFilePath != "" {
			// copy template args
			tcArgs := ic.TCArgs

			// copy kmux config path for specifying in agent config
			if obj.KmuxConfigPath != "" {
				tcArgs.KmuxConfigPath = obj.KmuxConfigPath
			}

			_, err = copyOrGenerateFile(ic.UserConfigPath, obj.AgentDir, obj.ConfigFilePath, ic.TemplateFuncs, obj.ConfigTemplateString, tcArgs)
			if err != nil {
				return err
			}
		}

		// copy kmux config
		if obj.KmuxConfigPath != "" {
			kmuxConfigArgs.ConsumerTag = obj.AgentName
			_, err = copyOrGenerateFile(ic.UserConfigPath, obj.AgentDir, cm.KmuxConfigFileName, ic.TemplateFuncs, obj.KmuxConfigTemplateString, kmuxConfigArgs)
			if err != nil {
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
				fmt.Println(color.YellowString("Warning! No destination for extra file %s", filename))
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
	fmt.Println(color.MagentaString("\nEnabling services..."))
	for _, obj := range ic.SystemdServiceObjects {
		err = StartSystemdService(obj.ServiceName)
		if err != nil {
			fmt.Printf("failed to start service %s: %s\n", obj.ServiceName, err.Error())
			return err
		}
	}

	// Clean Up
	fmt.Println(color.BlueString("\nCleaning up downloaded assets..."))
	Deletedir(cm.DownloadDir)
	return nil
}
