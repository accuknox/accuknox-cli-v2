package onboard

import (
	"fmt"

	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/fatih/color"
)

// prepare template for RAT systemd service timer and the script which will call RAT
// template for RAT variables
func (cc *ClusterConfig) InitRATConfig(authToken, url, tenantID, clusterID, clusterName, label, schedule, profile string, benchmark string, registry, registryConfigPath string, insecureRegistryConnection, httpRegistryConnection bool, ratImage, ratTag, releaseVersion string, preserveUpstream bool, vmMode VMMode) error {

	var releaseInfo cm.ReleaseMetadata
	if releaseVersion == "" {
		_, releaseInfo = cm.GetLatestReleaseInfo()
	} else if releaseInfoTemp, ok := cm.ReleaseInfo[releaseVersion]; ok {
		releaseInfo = releaseInfoTemp
	} else {
		// TODO: publish release JSON as OCI artifact to remove dependency
		// on needing to build knoxctl again and again
		// return nil, fmt.Errorf("Unknown image tag %s", releaseVersion)
	}

	cc.AgentsVersion = releaseVersion
	cc.RATAgentImage, _ = getImage(registry, cm.DefaultDockerRegistry,
		cm.DefaultAccuKnoxRepo, ratImage, cm.AgentRepos[cm.RAT],
		ratTag, releaseInfo.RatTag, "v", cm.SystemdTagSuffix, preserveUpstream)

	cc.RATConfigObject.AuthToken = authToken
	cc.RATConfigObject.Url = url
	cc.RATConfigObject.TenantId = tenantID
	cc.RATConfigObject.ClusterId = clusterID
	cc.RATConfigObject.ClusterName = clusterName
	cc.RATConfigObject.Label = label
	cc.RATConfigObject.Schedule = schedule
	cc.RATConfigObject.Profile = profile
	cc.RATConfigObject.Benchmark = benchmark

	cc.SystemdServiceObjects = append(cc.SystemdServiceObjects,

		SystemdServiceObject{
			AgentName:             cm.RAT,
			PackageName:           cm.RAT,
			ServiceName:           cm.RAT + ".service",
			AgentDir:              cm.RATPath,
			ServiceTemplateString: ratServiceFile,
			TimerTemplateString:   ratTimerFile,
			AgentImage:            cc.RATAgentImage,
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

	return nil
}

func (cc *ClusterConfig) InstallRAT() error {

	obj := cc.SystemdServiceObjects[0]

	fmt.Print(color.CyanString("Downloading Agent - %s | Image - %s\n", obj.AgentName, obj.AgentImage))
	packageMeta := splitLast(obj.AgentImage, ":")

	err := cc.installAgent(obj.AgentName, packageMeta[0], "0.3.0"+packageMeta[1])
	if err != nil {
		fmt.Println("error:", err)
		return err
	}
	err = cc.placeServiceFiles()
	if err != nil {
		//fmt.Println(err)
		return err
	}
	err = StartSystemdService(obj.ServiceName)
	if err != nil {
		//fmt.Println(err)
		return err
	}

	// create template for service and timer file

	//place service file and timer file

	return nil

}
func AddRATConfig(cc *ClusterConfig) {

}
