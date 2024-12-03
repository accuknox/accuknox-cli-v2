package onboard

import (
	"fmt"

	"github.com/Masterminds/sprig"
	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/fatih/color"
)

// Initialize RAT config
func (cc *ClusterConfig) InitRATConfig(authToken, url, tenantID, clusterID, clusterName, label, schedule, profile string, benchmark string, registry, registryConfigPath string, insecureRegistryConnection, httpRegistryConnection bool, ratImage, ratVersionTag, releaseVersion string, preserveUpstream bool) error {
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
	cc.RATConfigObject.EnableVMScan = true
	cc.AgentsVersion = releaseVersion
	cc.RATConfigObject.AuthToken = authToken
	cc.RATConfigObject.Url = url
	cc.RATConfigObject.TenantID = tenantID
	cc.RATConfigObject.ClusterID = clusterID
	cc.RATConfigObject.ClusterName = clusterName
	cc.RATConfigObject.Label = label

	if cc.Mode == VMMode_Systemd {
		cc.RATConfigObject.Schedule, err = ConvertCronToSystemd(schedule)
		if err != nil {
			return err
		}
	} else {
		cc.RATConfigObject.Schedule = schedule
	}

	cc.RATConfigObject.Profile = profile
	cc.RATConfigObject.Benchmark = benchmark

	switch cc.Mode {
	case VMMode_Docker:
		cc.RATImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, ratImage, releaseInfo.RatImage,
			ratVersionTag, releaseInfo.RatTag, "", "", preserveUpstream)
		if err != nil {
			return err
		}
		cc.RATConfigObject.RATImage = cc.RATImage

	case VMMode_Systemd:
		cc.RATImage, err = getImage(registry, cm.DefaultDockerRegistry,
			cm.DefaultAccuKnoxRepo, ratImage, cm.AgentRepos[cm.RAT],
			ratVersionTag, releaseInfo.RatTag, "v", cm.SystemdTagSuffix, preserveUpstream)
		if err != nil {
			return err
		}
		cc.SystemdServiceObjects = append(cc.SystemdServiceObjects,

			SystemdServiceObject{
				AgentName:             cm.RAT,
				PackageName:           cm.RAT,
				ServiceName:           cm.RAT + ".service",
				AgentDir:              cm.RATPath,
				ServiceTemplateString: ratServiceFile,
				TimerTemplateString:   ratTimerFile,
				AgentImage:            cc.RATImage,
				InstallOnWorkerNode:   true,
			},
		)
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

func (cc *ClusterConfig) InstallRAT() error {

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
		//create compose file
		composeFilePath, err := copyOrGenerateFile(cc.UserConfigPath, configPath, "docker-compose_rat.yaml", sprigFuncs, ratComposeFileTemplate, cc.RATConfigObject)
		if err != nil {
			return err
		}
		args := []string{"-f", composeFilePath, "--profile", "accuknox-agents", "up", "-d"}
		// run compose command
		_, err = ExecComposeCommand(true, cc.DryRun, cc.composeCmd, args...)
		if err != nil {
			return err
		}

	case VMMode_Systemd:
		var obj SystemdServiceObject
		for _, agent := range cc.SystemdServiceObjects {
			if agent.AgentName == cm.RAT {
				obj = agent
			}
		}
		fmt.Print(color.CyanString("Downloading Agent - %s | Image - %s\n", obj.AgentName, obj.AgentImage))
		packageMeta := splitLast(obj.AgentImage, ":")
		err := cc.installAgent(obj.AgentName, packageMeta[0], packageMeta[1])
		if err != nil {
			fmt.Println(color.RedString("RAT Installation failed!! Cleaning up downloaded asset..."))
			Deletedir(cm.DownloadDir)
			return err
		}
		err = cc.placeServiceFiles()
		if err != nil {
			return err
		}
		err = StartSystemdService(obj.ServiceName)
		if err != nil {
			return err
		}
		fmt.Println(color.BlueString("\nCleaning up downloaded assets..."))
		Deletedir(cm.DownloadDir)
	}
	return nil
}
