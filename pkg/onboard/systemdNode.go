package onboard

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Masterminds/sprig"
	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
)

func (jc *JoinConfig) JoinSystemdNode() error {
	// initialize template funcs
	jc.TemplateFuncs = sprig.GenericFuncMap()

	// Download and install agents
	logger.Info2("Downloading agents...")
	err := jc.SystemdInstall()
	if err != nil {
		logger.Error("Installation failed!! Error: %s.\nCleaning up downloaded assets...", err.Error())
		Deletedir(cm.DownloadDir)
		DeboardSystemd(NodeType_WorkerNode) // #nosec G104
		return err
	}

	jc.TCArgs.TlsEnabled = jc.Tls.Enabled

	if jc.Tls.RMQCredentials != "" {

		rmqData := strings.Split(Decode(jc.Tls.RMQCredentials), ":")
		if len(rmqData) != 2 {
			return fmt.Errorf("invalid RMQ credentials")
		}
		jc.TCArgs.RMQUsername = rmqData[0]
		jc.TCArgs.RMQPassword = rmqData[1]
	}

	if jc.Tls.Enabled {
		jc.TCArgs.TlsCertFile = "/opt" + cm.DefaultCACertDir + "/" + cm.DefaultEncodedFileName
		if err := StoreCert(map[string]string{
			jc.TCArgs.TlsCertFile: jc.Tls.CaCert,
		}); err != nil {
			return err
		}
	}

	jc.TCArgs.SplunkConfigObject = jc.Splunk

	if jc.TCArgs.SplunkConfigObject.Enabled {
		if err := validateSplunkCredential(jc.TCArgs.SplunkConfigObject); err != nil {
			return err
		}
	}

	// config services
	kmuxConfigArgs := KmuxConfigTemplateArgs{
		ReleaseVersion: jc.AgentsVersion,
		RMQServer:      jc.RMQServer,
		RMQUsername:    jc.TCArgs.RMQUsername,
		RMQPassword:    jc.TCArgs.RMQPassword,
		TlsEnabled:     jc.TCArgs.TlsEnabled,
		TlsCertFile:    jc.TCArgs.TlsCertFile,
	}

	logger.Info2("\nConfiguring services...")
	for _, obj := range jc.SystemdServiceObjects {
		if !obj.InstallOnWorkerNode {
			continue
		}
		if obj.AgentName == cm.SummaryEngine && !jc.DeploySumengine {
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
			populateKmuxArgs(&kmuxConfigArgs, obj.AgentName, obj.KmuxConfigFileName, jc.RMQTopicPrefix, jc.TCArgs.Hostname, jc.RMQConnectionName)
			// copy generic config files
			_, err = copyOrGenerateFile(jc.UserConfigPath, obj.AgentDir, obj.KmuxConfigFileName, jc.TemplateFuncs, obj.KmuxConfigTemplateString, kmuxConfigArgs)
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

	// Start services
	logger.Info2("\nEnabling services...")
	for _, obj := range jc.SystemdServiceObjects {
		if !obj.InstallOnWorkerNode {
			continue
		}
		if obj.AgentName == cm.SummaryEngine && !jc.DeploySumengine {
			continue
		}
		err = StartSystemdService(obj.ServiceName)
		if err != nil {
			logger.Debug("failed to start service %s: %s\n", obj.ServiceName, err.Error())
			return err
		}
	}
	logger.PrintSuccess("\nAll services enabled successfully.")

	logger.Info1("\nCleaning up downloaded assets...")
	Deletedir(cm.DownloadDir)
	return nil
}
