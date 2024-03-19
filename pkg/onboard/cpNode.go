package onboard

import (
	"fmt"
	"os"
	"strings"

	"github.com/Masterminds/sprig"
	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	"golang.org/x/mod/semver"
)

func InitCPNodeConfig(cc ClusterConfig, joinToken, spireHost, ppsHost, knoxGateway, spireTrustBundle string, enableLogs bool) *InitConfig {
	return &InitConfig{
		ClusterConfig: cc,
		JoinToken:     joinToken,
		SpireHost:     spireHost,
		PPSHost:       ppsHost,
		KnoxGateway:   knoxGateway,

		SpireTrustBundleURL: spireTrustBundle,
		EnableLogs:          enableLogs,
	}
}

func (ic *InitConfig) InitializeControlPlane() error {
	// validate this environment
	err := ic.validateEnv()
	if err != nil {
		return err
	}

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	configPath, err := createDefaultConfigPath()
	if err != nil {
		return err
	}

	spireHost, spirePort, err := parseURL(ic.SpireHost)
	if spirePort == "80" {
		// default spire port
		spirePort = "8081"
	}

	// currently unused as we use insecure bootstrap
	var spireTrustBundleURL = ic.SpireTrustBundleURL
	if spireTrustBundleURL == "" {
		if strings.Contains(ic.SpireHost, "spire.dev.accuknox.com") {
			spireTrustBundleURL = spireTrustBundleURLMap["dev"]
		} else if strings.Contains(ic.SpireHost, "spire.stage.accuknox.com") {
			spireTrustBundleURL = spireTrustBundleURLMap["stage"]
		} else if strings.Contains(ic.SpireHost, "spire.demo.accuknox.com") {
			spireTrustBundleURL = spireTrustBundleURLMap["demo"]
		} else if strings.Contains(ic.SpireHost, "spire.prod.accuknox.com") {
			spireTrustBundleURL = spireTrustBundleURLMap["prod"]
		} else if strings.Contains(ic.SpireHost, "spire.xcitium.accuknox.com") {
			spireTrustBundleURL = spireTrustBundleURLMap["xcitium"]
		}
	}

	ic.TCArgs = TemplateConfigArgs{
		KubeArmorImage:            ic.KubeArmorImage,
		KubeArmorInitImage:        ic.KubeArmorInitImage,
		KubeArmorRelayServerImage: ic.KubeArmorRelayServerImage,
		KubeArmorVMAdapterImage:   ic.KubeArmorVMAdapterImage,
		SIAImage:                  ic.SIAImage,
		PEAImage:                  ic.PEAImage,
		FeederImage:               ic.FeederImage,

		Hostname: hostname,
		// TODO: make configurable
		KubeArmorURL:  "kubearmor:32767",
		KubeArmorPort: "32767",

		RelayServerURL:  "kubearmor-relay-server:32768",
		RelayServerAddr: "kubearmor-relay-server",
		RelayServerPort: "32768",

		WorkerNode: ic.WorkerNode,

		SIAAddr: "shared-informer-agent:32769",
		PEAAddr: "policy-enforcement-agent:32770",

		PPSHost: ic.PPSHost,

		JoinToken:     ic.JoinToken,
		SpireHostAddr: spireHost,
		SpireHostPort: spirePort,

		SpireTrustBundleURL: spireTrustBundleURL,
		ImagePullPolicy:     string(ic.ImagePullPolicy),
		EnableLogs:          ic.EnableLogs,

		// kubearmor config
		KubeArmorVisibility:     ic.Visibility,
		KubeArmorHostVisibility: ic.HostVisibility,

		KubeArmorFilePosture:    ic.DefaultFilePosture,
		KubeArmorNetworkPosture: ic.DefaultNetworkPosture,
		KubeArmorCapPosture:     ic.DefaultCapPosture,

		KubeArmorHostFilePosture:    ic.DefaultHostFilePosture,
		KubeArmorHostNetworkPosture: ic.DefaultHostNetworkPosture,
		KubeArmorHostCapPosture:     ic.DefaultHostCapPosture,

		ConfigPath: configPath,

		NetworkCIDR: ic.CIDR,
	}

	// initialize sprig for templating
	sprigFuncs := sprig.GenericFuncMap()

	// write compose file
	composeFilePath, err := copyOrGenerateFile(ic.UserConfigPath, configPath, "docker-compose.yaml", sprigFuncs, cpComposeFileTemplate, ic.TCArgs)
	if err != nil {
		return err
	}

	_, err = copyOrGenerateFile(ic.UserConfigPath, configPath, "pea/application.yaml", sprigFuncs, peaConfig, ic.TCArgs)
	if err != nil {
		return err
	}

	_, err = copyOrGenerateFile(ic.UserConfigPath, configPath, "sia/app.yaml", sprigFuncs, siaConfig, ic.TCArgs)
	if err != nil {
		return err
	}

	_, err = copyOrGenerateFile(ic.UserConfigPath, configPath, "spire/conf/agent.conf", sprigFuncs, spireAgentConfig, ic.TCArgs)
	if err != nil {
		return err
	}

	kmuxConfigArgs := KmuxConfigTemplateArgs{
		StreamName: "knox-gateway",
		ServerURL:  ic.KnoxGateway,
	}

	_, err = copyOrGenerateFile(ic.UserConfigPath, configPath, "sia/kmux-config.yaml", sprigFuncs, kmuxConfig, kmuxConfigArgs)
	if err != nil {
		return err
	}

	_, err = copyOrGenerateFile(ic.UserConfigPath, configPath, "feeder-service/kmux-config.yaml", sprigFuncs, kmuxConfig, kmuxConfigArgs)
	if err != nil {
		return err
	}

	diagnosis := true
	args := []string{"-f", composeFilePath, "--profile",
		"spire-agent", "--profile", "kubearmor", "--profile", "accuknox-agents",
		"up", "-d"}

	if semver.Compare(ic.composeVersion, common.MinDockerComposeWithWaitSupported) >= 0 {
		args = append(args, "--wait", "--wait-timeout", "60")
	} else {
		diagnosis = false
	}

	// run compose command
	_, err = ExecComposeCommand(true, ic.DryRun, ic.composeCmd, args...)
	if err != nil && diagnosis {
		diagnosis, diagErr := diaganose(NodeType_ControlPlane)
		if diagErr != nil {
			diagnosis = diagErr.Error()
		}
		return fmt.Errorf("Error: %s.\n\nDIAGNOSIS:\n%s", err.Error(), diagnosis)
	} else if err != nil {
		return err
	}

	return nil
}
