package onboard

import (
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/sprig"
	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/imagescan"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/fatih/color"
)

// Initialize RRA config
func (cc *ClusterConfig) InitRRAConfig(authToken, url, tenantID, clusterID, clusterName, label, schedule, profile string, benchmark string, registry, registryConfigPath string, insecureRegistryConnection, httpRegistryConnection bool, rraImage, rraVersionTag, releaseVersion string, preserveUpstream, agentsDeployed bool, spireImage, spireHost, spireDir, knoxGateway string) error {
	var err error
	var releaseInfo cm.ReleaseMetadata
	if releaseVersion == "" {
		_, releaseInfo = cm.GetLatestReleaseInfo()
	} else if releaseInfoTemp, ok := cm.ReleaseInfo[releaseVersion]; ok {
		releaseInfo = releaseInfoTemp
	} else {
		// TODO: publish release JSON as OCI artifact to remove dependency
		// on needing to build knoxctl again and again
		return fmt.Errorf("unknown image tag %s", releaseVersion)
	}
	cc.RRAConfigObject.EnableVMScan = true
	cc.AgentsVersion = releaseVersion
	cc.RRAConfigObject.ArtifactToken = authToken
	cc.RRAConfigObject.Url = url
	cc.RRAConfigObject.TenantID = tenantID
	cc.RRAConfigObject.ClusterID = clusterID
	cc.RRAConfigObject.ClusterName = clusterName
	cc.RRAConfigObject.Label = label
	cc.RRAConfigObject.Hostname, err = os.Hostname()
	if err != nil {
		return err
	}

	if !agentsDeployed && spireHost != "" {
		host, port, _, err := getSpireDetails(spireHost, "")
		if err != nil {
			return err
		}
		cc.AdditionalArgs["SpireHostAddr"] = host
		cc.AdditionalArgs["SpireHostPort"] = port
		cc.AdditionalArgs["SpireEnabled"] = true
	}

	cc.RRAConfigObject.SpireSecretDir = spireDir
	cc.RRAConfigObject.GatewayServer = knoxGateway

	if cc.Mode == VMMode_Systemd {
		cc.RRAConfigObject.Schedule, err = ConvertCronToSystemd(schedule)
		if err != nil {
			return err
		}
	} else {
		cc.RRAConfigObject.Schedule = schedule
	}

	cc.RRAConfigObject.Profile = profile
	cc.RRAConfigObject.Benchmark = benchmark

	// docker config
	cc.RRAConfigObject.DockerLogDriver = GetDockerLogDriver()
	cc.RRAConfigObject.DockerLogRotateMaxSize = cc.LogRotateMaxSize
	cc.RRAConfigObject.DockerLogRotateMaxFile = fmt.Sprintf("%d", cc.LogRotateMaxFile)

	switch cc.Mode {
	case VMMode_Docker:
		cc.RRAImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, rraImage, releaseInfo.RraImage,
			rraVersionTag, releaseInfo.RraTag, "", "", preserveUpstream)
		if err != nil {
			return err
		}

		if !agentsDeployed {
			cc.SPIREAgentImage, err = getImage(registry, cm.DefaultDockerRegistry,
				cm.DefaultAccuKnoxRepo, spireImage, cm.DefaultSPIREAgentImage,
				"latest", releaseInfo.SPIREAgentImageTag, "", "", preserveUpstream)
			if err != nil {
				return err
			}

			cc.WaitForItImage, err = getImage(registry, cm.DefaultDockerRegistry,
				cm.DefaultAccuKnoxRepo, "", cm.DefaultWaitForItImage,
				"latest", "", "", "", preserveUpstream)
			if err != nil {
				return err
			}
			cc.AdditionalArgs["SPIREAgentImage"] = cc.SPIREAgentImage
			cc.AdditionalArgs["WaitForItImage"] = cc.WaitForItImage
			cc.AdditionalArgs["ImagePullPolicy"] = "always"
		}

		fmt.Println(cc.RRAImage)
		cc.RRAConfigObject.RRAImage = cc.RRAImage

	case VMMode_Systemd:
		cc.LogRotateTemplateString = logRotateFile
		cc.RRAImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, rraImage, cm.AgentRepos[cm.RRA],
			rraVersionTag, releaseInfo.RraTag, "v", cm.SystemdTagSuffix, preserveUpstream)
		if err != nil {
			return err
		}

		if !agentsDeployed {
			cc.SPIREAgentImage, err = getImage(registry, cm.DefaultDockerRegistry,
				cm.DefaultAccuKnoxRepo, spireImage, cm.AgentRepos[cm.SpireAgent],
				"", releaseInfo.SPIREAgentImageTag, "v", cm.SystemdTagSuffix, preserveUpstream)
			if err != nil {
				return err
			}
		}

		cc.SystemdServiceObjects = append(cc.SystemdServiceObjects,

			SystemdServiceObject{
				AgentName:             cm.RRA,
				PackageName:           cm.RRA,
				ServiceName:           cm.RRA + ".service",
				AgentDir:              cm.RRAPath,
				ServiceTemplateString: rraServiceFile,
				TimerTemplateString:   rraTimerFile,
				AgentImage:            cc.RRAImage,
				InstallOnWorkerNode:   true,
			},
		)

		if !agentsDeployed {

			cc.AdditionalArgs["PartOfVMScan"] = true

			cc.SystemdServiceObjects = append(cc.SystemdServiceObjects, SystemdServiceObject{
				AgentName:             cm.SpireAgent,
				PackageName:           cm.SpireAgent,
				ServiceName:           cm.SpireAgent + ".service",
				AgentDir:              cm.SpireConfigPath,
				ServiceTemplateString: spireAgentFile,
				ConfigFilePath:        "conf/agent/agent.conf",
				ConfigTemplateString:  spireAgentConfig,
				AgentImage:            cc.SPIREAgentImage,
				LogRotateMaxFileSize:  cc.LogRotateMaxSize,
				LogRotateMaxFile:      cc.LogRotateMaxFile,
			})
		}

		loginOptions := LoginOptions{
			Insecure:           insecureRegistryConnection,
			PlainHTTP:          httpRegistryConnection,
			Registry:           registry,
			RegistryConfigPath: registryConfigPath,
		}
		loginOptions.PlainHTTP = loginOptions.isPlainHttp(registry)
		cc.PlainHTTP = loginOptions.PlainHTTP
		var err error
		cc.ORASClient, err = loginOptions.ORASGetAuthClient()
		if err != nil {
			return err
		}
	}

	return nil
}

func (cc *ClusterConfig) InstallRRA() error {
	var configObject map[string]any
	configBytes, _ := json.Marshal(cc.RRAConfigObject)
	if err := json.Unmarshal(configBytes, &configObject); err != nil {
		return err
	}

	if cc.AdditionalArgs != nil {
		maps.Copy(configObject, cc.AdditionalArgs)
	}

	cc.AdditionalArgs = configObject

	switch cc.Mode {
	case VMMode_Docker:
		var err error
		cc.composeCmd, cc.composeVersion, err = GetComposeCommand()
		if err != nil {
			return err
		}
		configPath, err := createDefaultConfigPath()
		if err != nil {
			return err
		}
		// initialize sprig for templating
		sprigFuncs := sprig.GenericFuncMap()
		// create compose file
		composeFilePath, err := copyOrGenerateFile(cc.UserConfigPath, configPath, fmt.Sprintf("docker-compose_%s.yaml", cm.RRA), sprigFuncs, rraComposeFileTemplate, configObject)
		if err != nil {
			return err
		}
		args := []string{"-f", composeFilePath, "--profile", "accuknox-agents"}

		if cc.AdditionalArgs != nil && cc.AdditionalArgs["SpireEnabled"] == true {
			args = append(args, "--profile", "spire-agent")
		}

		args = append(args, "up", "-d")

		// run compose command
		_, err = ExecComposeCommand(true, cc.DryRun, cc.composeCmd, args...)
		if err != nil {
			return err
		}

	case VMMode_Systemd:
		serviceFiles := []string{"accuknox-rra.timer"}
		for _, agent := range cc.SystemdServiceObjects {
			fmt.Print(color.CyanString("Downloading Agent - %s | Image - %s\n", agent.AgentName, agent.AgentImage))
			packageMeta := splitLast(agent.AgentImage, ":")
			err := cc.installAgent(agent.AgentName, packageMeta[0], packageMeta[1])
			if err != nil {
				fmt.Println(color.RedString("RRA Installation failed!! Cleaning up downloaded asset..."))
				Deletedir(cm.DownloadDir)
				return err
			}
			if agent.AgentName == cm.SpireAgent {
				serviceFiles = append(serviceFiles, agent.ServiceName)

				if agent.ConfigFilePath != "" {
					_, err = copyOrGenerateFile(cc.UserConfigPath, agent.AgentDir, agent.ConfigFilePath, sprig.GenericFuncMap(), agent.ConfigTemplateString, configObject)
					if err != nil {
						return err
					}
				}
			}
		}
		err := cc.placeServiceFiles()
		if err != nil {
			return err
		}
		for _, service := range serviceFiles {
			err := StartSystemdService(service)
			if err != nil {
				return err
			}
		}
		fmt.Println(color.BlueString("\nCleaning up downloaded assets..."))
		Deletedir(cm.DownloadDir)
	}
	return nil
}

// Initialize Imagescan config and delete if it already exists
func (cc *ClusterConfig) InitImageScan(authToken, url, tenantID, clusterID, clusterName, label, schedule string, allImages bool) error {

	if cc.Mode == VMMode_Systemd {
		var err error
		schedule, err = ConvertCronToSystemd(schedule)
		if err != nil {
			logger.Error("error while converting cron into systemd timer: ", err)
			return err
		}
	}

	cc.ImageScanConfig = ImageScanConfig{
		tenantID:    tenantID,
		clusterName: clusterName,
		clusterID:   clusterID,
		authToken:   authToken,
		schedule:    schedule,
		url:         url,
		label:       label,
		allImages:   allImages,
	}

	cc.SystemdServiceObjects = append(cc.SystemdServiceObjects, SystemdServiceObject{
		AgentName:               cm.Imagescan,
		AgentImage:              cm.ImagescanBaseImage,
		PackageName:             cm.Imagescan,
		ServiceName:             cm.Imagescan + ".service",
		AgentDir:                cm.ImageScanConfigPath,
		ServiceTemplateString:   containerImageScannerFile,
		TimerTemplateString:     containerImageScannerTimerFile,
		EnvironmentFileTemplate: imagescanEnvVal,
		LogRotateMaxFileSize:    cc.LogRotateMaxSize,
		LogRotateMaxFile:        cc.LogRotateMaxFile,
	},
	)

	cc.LogRotateTemplateString = logRotateFile

	cc.TemplateFuncs = sprig.FuncMap()

	// Stop existing running service
	return StopSystemdService(cm.Imagescan+".service", true, false)
}

func (cc *ClusterConfig) InstallImagescan() error {
	knoxctlPath, err := getKnoxctlPath()
	if err != nil {
		return fmt.Errorf("unable to indentify knoxctl path: %v", err)
	}

	var sockPaths []string
	if cc.Mode == VMMode_Docker {
		// Detect CRI paths(sockets) for mounting it into the container
		runtimes := []string{"docker", "containerd", "cri-o", "nri"}
		for _, r := range runtimes {
			if _, criPaths, ok := imagescan.DiscoverRuntime("", r); ok {
				for _, criPath := range criPaths {
					criPath = strings.TrimPrefix(criPath, "unix://")
					sockPaths = append(sockPaths, criPath)
				}
			}
		}
	}

	args := map[string]any{
		"Url":               cc.ImageScanConfig.url,
		"AuthToken":         cc.ImageScanConfig.authToken,
		"Label":             cc.ImageScanConfig.label,
		"AllImages":         cc.ImageScanConfig.allImages,
		"TenantID":          cc.ImageScanConfig.tenantID,
		"ClusterID":         cc.ImageScanConfig.clusterID,
		"ClusterName":       cc.ImageScanConfig.clusterName,
		"EnvironmentFile":   cmp.Or(cc.UserConfigPath, filepath.Join(cm.ImageScanConfigPath, cm.Imagescan+".env")),
		"ImagescanUnitName": cm.Imagescan + ".service",
		"Schedule":          cc.ImageScanConfig.schedule,
		"AgentName":         cm.Imagescan,
		"BaseImage":         cm.ImagescanBaseImage,
		"KnoxctlPath":       knoxctlPath,
		"SockPaths":         sockPaths,
	}

	if cc.AdditionalArgs == nil {
		cc.AdditionalArgs = make(map[string]any)
	}

	maps.Copy(cc.AdditionalArgs, args)

	switch cc.Mode {
	case VMMode_Systemd:
		if err := cc.placeServiceFiles(); err != nil {
			logger.Error("err while placing service file: %v\n", err)
			Deletedir(cm.ImageScanConfigPath)
			return err
		}

		// Create required environment files for the scan
		cc.createEnvironmentFile()

		err := StartSystemdService(cm.Imagescan + ".service")
		if err != nil {
			logger.Warn("failed to start service %s: %s\n", cm.Imagescan, err.Error())
			return err
		}

	case VMMode_Docker:
		var err error
		cc.composeCmd, cc.composeVersion, err = GetComposeCommand()
		if err != nil {
			return err
		}

		configPath, err := createDefaultConfigPath()
		if err != nil {
			return err
		}

		// initialize sprig for templating
		sprigFuncs := sprig.GenericFuncMap()

		// create compose file
		composeFilePath, err := copyOrGenerateFile(cc.UserConfigPath, configPath, fmt.Sprintf("docker-compose_%s.yaml", cm.Imagescan), sprigFuncs, accuknoxScannerComposeFileTemplate, cc.AdditionalArgs)
		if err != nil {
			return err
		}

		args := []string{"-f", composeFilePath, "--profile", "accuknox-agents"}

		args = append(args, "up", "-d")

		// run compose command
		_, err = ExecComposeCommand(true, cc.DryRun, cc.composeCmd, args...)
		if err != nil {
			return err
		}

	}

	return nil
}

func (cc *ClusterConfig) createEnvironmentFile() {
	for _, obj := range cc.SystemdServiceObjects {
		if obj.EnvironmentFileTemplate != "" {
			cc.TemplateFuncs = sprig.FuncMap()
			_, err := copyOrGenerateFile(cc.UserConfigPath, obj.AgentDir, obj.AgentName+".env", cc.TemplateFuncs, obj.EnvironmentFileTemplate, cc.AdditionalArgs)
			if err != nil {
				logger.Error("err while creating env file for %s: %v\n", obj.AgentName, err)
				continue
			}
		}
	}
}

func getKnoxctlPath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return exePath, err
	}
	return exePath, nil
}
