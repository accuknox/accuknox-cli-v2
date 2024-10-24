package onboard

import (
	"fmt"
	"path/filepath"

	"github.com/Masterminds/sprig"
	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/fatih/color"
)

func (jc *JoinConfig) JoinSystemdNode() error {
	// initialize template funcs
	jc.TemplateFuncs = sprig.GenericFuncMap()

	// Download and intall agents
	fmt.Println(color.MagentaString("Downloading agents..."))
	err := jc.SystemdInstall()
	if err != nil {
		fmt.Println(color.RedString("Installation failed!! Error: %s.\nCleaning up downloaded assets...", err.Error()))
		Deletedir(cm.DownloadDir)
		DeboardSystemd(NodeType_WorkerNode) // #nosec G104
		return err
	}

	// config services
	kmuxConfigArgs := KmuxConfigTemplateArgs{
		ReleaseVersion: jc.AgentsVersion,
		RMQServer:      jc.RMQServer,
	}

	fmt.Println(color.MagentaString("\nConfiguring services..."))
	for _, obj := range jc.SystemdServiceObjects {
		if !obj.InstallOnWorkerNode {
			continue
		}
		if obj.ConfigFilePath != "" {
			// copy template args
			tcArgs := jc.TCArgs

			// copy kmux config path for specifying in agent config
			if obj.KmuxConfigPath != "" {
				tcArgs.KmuxConfigPath = obj.KmuxConfigPath
			}

			// copy generic config files
			_, err = copyOrGenerateFile(jc.UserConfigPath, obj.AgentDir, obj.ConfigFilePath, jc.TemplateFuncs, obj.ConfigTemplateString, tcArgs)
			if err != nil {
				return err
			}
		}

		// copy kmux config
		if obj.KmuxConfigPath != "" {
			kmuxConfigArgs.ConsumerTag = obj.AgentName
			// copy generic config files
			_, err = copyOrGenerateFile(jc.UserConfigPath, obj.AgentDir, cm.KmuxConfigFileName, jc.TemplateFuncs, obj.KmuxConfigTemplateString, kmuxConfigArgs)
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

	// Start services
	fmt.Println(color.MagentaString("\nEnabling services..."))
	for _, obj := range jc.SystemdServiceObjects {
		if !obj.InstallOnWorkerNode {
			continue
		}

		err = StartSystemdService(obj.ServiceName)
		if err != nil {
			fmt.Printf("failed to start service %s: %s\n", obj.ServiceName, err.Error())
			return err
		}
	}
	fmt.Println(color.GreenString("\nAll services enabled successfully."))

	fmt.Println(color.BlueString("\nCleaning up downloaded assets..."))
	Deletedir(cm.DownloadDir)
	return nil
}
