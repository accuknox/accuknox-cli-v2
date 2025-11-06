package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/logger"
	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/accuknox/accuknox-cli-v2/pkg/vm"
	"github.com/spf13/cobra"
)

var (
	// essential flags
	ppsHost     string
	knoxGateway string

	// non-essential
	spireTrustBundle string
	enableLogs       bool

	// access-key flags
	accessKey     string
	vmName        string
	tokenURL      string
	tokenEndpoint string

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
	enableHardeningAgent      bool

	proxy onboard.Proxy
)

// cpNodeCmd represents the init command
var cpNodeCmd = &cobra.Command{
	Use:   "cp-node",
	Short: "Initialize a control plane node for onboarding onto SaaS",
	Long:  "Initialize a control plane node for onboarding onto SaaS",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		inspectionOptions := options
		inspectionOptions.Print = printInspectOutput

		if err := vm.InspectVM(&inspectionOptions); err != nil {
			return err
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// validate environment for pre-requisites
		var (
			cc  onboard.ClusterConfig
			err error
		)

		_, err = cc.ValidateEnv()

		if vmMode == "" {
			if err == nil {
				vmMode = onboard.VMMode_Docker
			} else {
				logger.Warn("warning: Docker requirements did not match:\n%s.\nFalling back to systemd mode for installation.\n", err.Error())
				vmMode = onboard.VMMode_Systemd
			}

		} else if vmMode == onboard.VMMode_Docker && err != nil {
			// docker mode specified explicitly but requirements didn't match
			logger.Error("failed to validate environment:\n%s", err.Error())
			return err
		}

		if spireHost == "" {
			logger.Error("SPIRE host is required")
			return fmt.Errorf("SPIRE host is required")
		}

		var configDumpPath string
		switch vmMode {
		case onboard.VMMode_Systemd:
			err := os.Mkdir(common.SystemdKnoxctlDir, 0755) // #nosec G301 need for archiving and file operations
			if err != nil && !os.IsExist(err) {
				return err
			}

			configDumpPath = filepath.Join(common.SystemdKnoxctlDir, common.KnoxctlConfigFilename)
			err = logger.SetOut(filepath.Join(common.SystemdKnoxctlDir, common.KnoxctlLogFilename))
			if err != nil {
				logger.Warn("failed to set log output file: %s", err.Error())
			}
			logger.Debug("===\n%s - Running %s", time.Now().Format(time.RFC3339), strings.Join(os.Args, " "))
		case onboard.VMMode_Docker:
			// TODO
			defaultConfigPath, err := common.GetDefaultConfigPath()
			if err == nil {
				configDumpPath = filepath.Join(defaultConfigPath, common.KnoxctlConfigFilename)
			}
		}

		vmConfig, err := onboard.CreateClusterConfig(onboard.ClusterType_VM, userConfigPath, vmMode,
			vmAdapterTag, kubeArmorRelayServerTag, peaVersionTag, siaVersionTag,
			feederVersionTag, sumEngineVersionTag, discoverVersionTag, hardeningAgentVersionTag, kubearmorVersion, releaseVersion, kubeArmorImage,
			kubeArmorInitImage, kubeArmorVMAdapterImage, kubeArmorRelayServerImage, siaImage,
			peaImage, feederImage, rmqImage, sumEngineImage, hardeningAgentImage, spireAgentImage, waitForItImage, discoverImage, nodeAddr, dryRun,
			false, deployRMQ, imagePullPolicy, visibility, hostVisibility, sumEngineVisibility, audit, block, hostAudit, hostBlock,
			alertThrottling, maxAlertPerSec, throttleSec,
			cidr, secureContainers, skipBTF, systemMonitorPath, rmqAddress, deploySumEngine, registry, registryConfigPath, insecure, plainHTTP, preserveUpstream, topicPrefix, rmqConnectionName, sumEngineCronTime, tls, enableHostPolicyDiscovery, splunk, nodeStateRefreshTime, spireEnabled, spireCert, logRotate, parallel, enableHardeningAgent, releaseFile, proxy)
		if err != nil {
			errConfig := onboard.DumpConfig(vmConfig, configDumpPath)
			if errConfig != nil {
				logger.Warn("Failed to create config dump at %s: %s", configDumpPath, errConfig.Error())
			}

			logger.Error("failed to create cluster config: %s", err.Error())
			return err
		}

		vmConfig.KaResource = kaResource
		vmConfig.AgentsResource = agentsResource

		if accessKey != "" {
			if joinToken, err = vmConfig.PopulateAccessKeyConfig(tokenURL, accessKey, topicPrefix, vmName, tokenEndpoint, "vm", insecure); err != nil {
				return err
			}
		}

		onboardConfig := onboard.InitCPNodeConfig(*vmConfig, joinToken, spireHost, ppsHost, knoxGateway, spireTrustBundle, spireDir, enableLogs)

		defer func() {
			err := onboard.DumpConfig(onboardConfig, configDumpPath)
			if err != nil {
				logger.Warn("Failed to create config dump at %s: %s", configDumpPath, err.Error())
			}
		}()
		err = onboardConfig.CreateBaseTemplateConfig()
		if err != nil {
			logger.Error("failed to create base template config: %s", err.Error())
			return err
		}

		switch vmMode {

		case onboard.VMMode_Systemd:
			err = onboardConfig.InitializeControlPlaneSD()
			if err != nil {
				logger.Error("failed to onboard control plane node: %s", err.Error())
				return err
			}
		case onboard.VMMode_Docker:
			err = onboardConfig.InitializeControlPlane()
			if err != nil {
				logger.Error("failed to onboard control plane node: %s", err.Error())
				return err
			}

		default:
			logger.Error("vm mode: %s invalid, accepted values (docker/systemd)", vmMode)
			return err
		}
		if enableVMScan {
			err := vmConfig.InitRRAConfig(authToken, url, tenantID, clusterID, clusterName, label, schedule, profile, benchmark, registry, registryConfigPath, insecure, plainHTTP, rraImage, rraTag, releaseVersion, preserveUpstream, true, spireAgentImage, spireHost, spireDir, knoxGateway)
			if err != nil {
				logger.Error("error initializing RRA config", vmMode)
			}
			err = onboardConfig.InstallRRA()
			if err != nil {
				logger.Error("error installing RRA in %s mode /n", vmMode)
			}
		}

		logger.PrintSuccess((`VM successfully onboarded!

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

	cpNodeCmd.PersistentFlags().StringVar(&ppsHost, "pps-host", "", "address of policy-provider-service to connect with for receiving policies")

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

	cpNodeCmd.PersistentFlags().StringVar(&discoverImage, "discover-image", "", "discover image to use")
	cpNodeCmd.PersistentFlags().StringVar(&discoverVersionTag, "discover-version", "", "discover version to use")
	cpNodeCmd.PersistentFlags().StringVar(&hardeningAgentImage, "hardening-agent-image", "", "hardening-agent image to use")
	cpNodeCmd.PersistentFlags().StringVar(&hardeningAgentVersionTag, "hardening-agent-version", "", "hardening-agent version to use")

	cpNodeCmd.PersistentFlags().StringVar(&rmqImage, "rmq-image", "", "RabbitMQ image to use")
	cpNodeCmd.PersistentFlags().BoolVar(&deployRMQ, "deploy-rmq", true, "To deploy RabbitMQ")

	// Access Key configurations
	cpNodeCmd.PersistentFlags().StringVar(&accessKey, "access-key", "", "access-key for onboarding")
	cpNodeCmd.PersistentFlags().StringVar(&tokenURL, "access-key-url", "", "access-key-url for onboarding")
	cpNodeCmd.PersistentFlags().StringVar(&tokenEndpoint, "access-key-endpoint", "/access-token/api/v1/process", "access-key-endpoint for onboarding")

	// dev2 config
	cpNodeCmd.PersistentFlags().BoolVar(&enableHostPolicyDiscovery, "enable-host-policy-discovery", false, "to enable host policy auto-discovery")

	cpNodeCmd.PersistentFlags().BoolVar(&enableHardeningAgent, "enable-hardening-agent", false, "to enable hardening agent")

	cpNodeCmd.PersistentFlags().BoolVar(&proxy.Enabled, "proxy", false, "bypass spire and use proxy")

	cpNodeCmd.PersistentFlags().StringVar(&proxy.Address, "proxy-address", "", "proxy address")

	cpNodeCmd.PersistentFlags().StringArrayVar(&proxy.ExtraArgs, "proxy-args", []string{}, "extra env variables for proxy")

	err := cpNodeCmd.MarkPersistentFlagRequired("pps-host")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	cpNodeCmd.MarkFlagsRequiredTogether("access-key", "access-key-url")

	onboardVMCmd.AddCommand(cpNodeCmd)
}
