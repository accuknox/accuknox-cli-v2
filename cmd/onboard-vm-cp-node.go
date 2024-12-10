package cmd

import (
	"fmt"
	"os"

	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	// essential flags
	joinToken   string
	spireHost   string
	ppsHost     string
	knoxGateway string

	// non-essential
	spireTrustBundle string
	enableLogs       bool

	// access-key flags
	accessKey string
	vmName    string
	tokenURL  string

	// cp-node only images
	kubeArmorRelayServerImage string
	siaImage                  string
	peaImage                  string
	feederImage               string
	spireAgentImage           string
	waitForItImage            string
	discoverImage             string
	sumEngineImage            string
	hardeningAgentImage       string
	rmqImage                  string

	deployRMQ bool

	// cp-node systemd tags
	kubeArmorRelayServerTag  string
	siaVersionTag            string
	peaVersionTag            string
	feederVersionTag         string
	discoverVersionTag       string
	sumEngineVersionTag      string
	hardeningAgentVersionTag string

	enableHostPolicyDiscovery bool
)

// cpNodeCmd represents the init command
var cpNodeCmd = &cobra.Command{
	Use:   "cp-node",
	Short: "Initialize a control plane node for onboarding onto SaaS",
	Long:  "Initialize a control plane node for onboarding onto SaaS",
	RunE: func(cmd *cobra.Command, args []string) error {
		// validate environment for pre-requisites
		var (
			cc  onboard.ClusterConfig
			err error
		)

		if accessKey != "" {
			joinToken, err = onboard.GetJoinTokenFromAccessKey(accessKey, vmName, tokenURL, insecure)
			if err != nil {
				return fmt.Errorf(color.RedString(err.Error()))
			}
		}

		_, err = cc.ValidateEnv()

		if vmMode == "" {
			if err == nil {
				vmMode = onboard.VMMode_Docker
			} else {
				fmt.Printf(
					color.YellowString("warning: Docker requirements did not match:\n%s.\nFalling back to systemd mode for installation.\n", err.Error()))
				vmMode = onboard.VMMode_Systemd
			}

		} else if vmMode == onboard.VMMode_Docker && err != nil {
			// docker mode specified explicitly but requirements didn't match
			return fmt.Errorf(color.RedString("failed to validate environment:\n%s", err.Error()))
		}

		vmConfig, err := onboard.CreateClusterConfig(onboard.ClusterType_VM, userConfigPath, vmMode,
			vmAdapterTag, kubeArmorRelayServerTag, peaVersionTag, siaVersionTag,
			feederVersionTag, sumEngineVersionTag, discoverVersionTag, hardeningAgentVersionTag, kubearmorVersion, releaseVersion, kubeArmorImage,
			kubeArmorInitImage, kubeArmorVMAdapterImage, kubeArmorRelayServerImage, siaImage,
			peaImage, feederImage, rmqImage, sumEngineImage, hardeningAgentImage, spireAgentImage, waitForItImage, discoverImage, nodeAddr, dryRun,
			false, deployRMQ, imagePullPolicy, visibility, hostVisibility, sumengineVisibility, audit, block, hostAudit, hostBlock,
			alertThrottling, maxAlertPerSec, throttleSec,
			cidr, secureContainers, skipBTF, systemMonitorPath, rmqAddress, deploySumegine, registry, registryConfigPath, insecure, plainHTTP, preserveUpstream, topicPrefix, tls, enableHostPolicyDiscovery)
		if err != nil {
			return fmt.Errorf(color.RedString("failed to create cluster config: %s", err.Error()))
		}

		onboardConfig := onboard.InitCPNodeConfig(*vmConfig, joinToken, spireHost, ppsHost, knoxGateway, spireTrustBundle, enableLogs)

		err = onboardConfig.CreateBaseTemplateConfig()
		if err != nil {
			return fmt.Errorf(color.RedString("failed to create base template config: %s", err.Error()))
		}

		switch vmMode {

		case onboard.VMMode_Systemd:
			err = onboardConfig.InitializeControlPlaneSD()
			if err != nil {
				return fmt.Errorf(color.RedString("failed to onboard control plane node: %s", err.Error()))
			}
		case onboard.VMMode_Docker:
			err = onboardConfig.InitializeControlPlane()
			if err != nil {
				return fmt.Errorf(color.RedString("failed to onboard control plane node: %s", err.Error()))
			}

		default:
			return fmt.Errorf(color.RedString("vm mode: %s invalid, accepted values (docker/systemd)", vmMode))
		}
		if enableVMScan {
			err := vmConfig.InitRATConfig(authToken, url, tenantID, clusterID, clusterName, label, schedule, profile, benchmark, registry, registryConfigPath, insecure, plainHTTP, ratImage, ratTag, releaseVersion, preserveUpstream)
			if err != nil {
				fmt.Println(color.RedString("error initializing RAT config", vmMode))
			}
			err = onboardConfig.InstallRAT()
			if err != nil {
				fmt.Println(color.RedString("error installing RAT in %s mode /n", vmMode))
			}
		}

		fmt.Println(color.GreenString(
			`VM successfully onboarded!

Now run the below command to onboard any worker nodes.
Please assign appropriate IP address to --cp-node-addr to make sure
that worker nodes can connect to this node`))

		onboardConfig.PrintJoinCommand()

		return nil
	},
}

func init() {
	// configuration for connecting with accuKnox SaaS
	cpNodeCmd.PersistentFlags().StringVarP(&releaseVersion, "version", "v", "", "agents release version to use")

	cpNodeCmd.PersistentFlags().StringVar(&joinToken, "join-token", "", "join-token to use")
	cpNodeCmd.PersistentFlags().StringVar(&spireHost, "spire-host", "", "address of spire-host to connect for authenticating with accuknox SaaS")
	cpNodeCmd.PersistentFlags().StringVar(&ppsHost, "pps-host", "", "address of policy-provider-service to connect with for receiving policies")
	cpNodeCmd.PersistentFlags().StringVar(&knoxGateway, "knox-gateway", "", "address of knox-gateway to connect with for pushing telemetry data")

	cpNodeCmd.PersistentFlags().StringVar(&spireTrustBundle, "spire-trust-bundle-addr", "", "address of spire trust bundle (CA cert for accuknox spire-server)")

	cpNodeCmd.PersistentFlags().BoolVar(&enableLogs, "enable-logs", false, "enable pushing logs from feeder service")

	cpNodeCmd.PersistentFlags().StringVar(&nodeAddr, "cp-node-addr", "", "address of control plane node for generating join command")

	cpNodeCmd.PersistentFlags().StringVar(&kubeArmorRelayServerImage, "kubearmor-relay-server", "", "KubeArmor relay-server image to use")
	cpNodeCmd.PersistentFlags().StringVar(&siaImage, "sia-image", "", "sia image to use")
	cpNodeCmd.PersistentFlags().StringVar(&peaImage, "pea-image", "", "pea image to use")
	cpNodeCmd.PersistentFlags().StringVar(&feederImage, "feeder-image", "", "feeder-service image to use")
	cpNodeCmd.PersistentFlags().StringVar(&kubeArmorRelayServerTag, "relayserver-version", "", "relay server version to use")
	cpNodeCmd.PersistentFlags().StringVar(&siaVersionTag, "sia-version", "", "sia version to use")
	cpNodeCmd.PersistentFlags().StringVar(&peaVersionTag, "pea-version", "", "pea version to use")
	cpNodeCmd.PersistentFlags().StringVar(&feederVersionTag, "feeder-version", "", "feeder version to use")
	cpNodeCmd.PersistentFlags().StringVar(&spireAgentImage, "spire-agent-image", "", "spire-agent image to use")
	cpNodeCmd.PersistentFlags().StringVar(&waitForItImage, "wait-for-it-image", "", "wait-for-it image to use")
	cpNodeCmd.PersistentFlags().StringVar(&discoverImage, "discover-image", "", "discover image to use")
	cpNodeCmd.PersistentFlags().StringVar(&discoverVersionTag, "discover-version", "", "discover version to use")
	cpNodeCmd.PersistentFlags().StringVar(&hardeningAgentImage, "hardening-agent-image", "", "hardening-agent image to use")
	cpNodeCmd.PersistentFlags().StringVar(&hardeningAgentVersionTag, "hardening-agent-version", "", "hardening-agent version to use")

	cpNodeCmd.PersistentFlags().StringVar(&rmqImage, "rmq-image", "", "RabbitMQ image to use")
	cpNodeCmd.PersistentFlags().BoolVar(&deployRMQ, "deploy-rmq", true, "To deploy RabbitMQ")

	// Access Key configurations
	cpNodeCmd.PersistentFlags().StringVar(&accessKey, "access-key", "", "access-key for onboarding")
	cpNodeCmd.PersistentFlags().StringVar(&vmName, "vm-name", "", "vm name for onboarding")
	cpNodeCmd.PersistentFlags().StringVar(&tokenURL, "access-key-url", "", "access-key-url for onboarding")

	// dev2 config
	cpNodeCmd.PersistentFlags().BoolVar(&enableHostPolicyDiscovery, "enable-host-policy-discovery", false, "to enable host policy auto-discovery")

	err := cpNodeCmd.MarkPersistentFlagRequired("spire-host")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = cpNodeCmd.MarkPersistentFlagRequired("pps-host")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = cpNodeCmd.MarkPersistentFlagRequired("knox-gateway")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = cpNodeCmd.MarkPersistentFlagRequired("version")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	cpNodeCmd.MarkFlagsRequiredTogether("access-key", "vm-name", "access-key-url")
	cpNodeCmd.MarkFlagsMutuallyExclusive("access-key", "join-token")

	onboardVMCmd.AddCommand(cpNodeCmd)
}
