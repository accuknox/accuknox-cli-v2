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
	kubeArmorAddr   string
	relayServerAddr string
	siaAddr         string
	peaAddr         string
	hardenAddr      string

	deploySumEngine bool

	// spire config - to use spire and spire cert for tls
	spireEnabled bool
	spireCert    bool
)

// joinNodeCmd represents the join command
var joinNodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Join this worker node with the control plane node for onboarding onto SaaS",
	Long:  "Join this worker node with the control plane node for onboarding onto SaaS",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		inspectionOptions := options
		inspectionOptions.Print = printInspectOutput

		if err := vm.InspectVM(&inspectionOptions); err != nil {
			return err
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error

		if spireEnabled && spireHost == "" {
			logger.Error("spire is enabled, spire host must be specified")
			return fmt.Errorf("spire is enabled, spire host must be specified")
		}

		// need at least either one of the below flags
		if nodeAddr == "" {
			if siaAddr == "" || relayServerAddr == "" || peaAddr == "" || hardenAddr == "" {
				logger.Error("cp-node-addr (control-plane address) or address of each agent must be specified")
				return err
			}

			if deploySumEngine && rmqAddress == "" {
				logger.Error("cp-node-addr (control-plane address) or address of control plane RabbitMQ server must be specified")
				return err
			}
		}
		// validate environment for pre-requisites
		var cc onboard.ClusterConfig
		_, err = cc.ValidateEnv()
		if vmMode == "" {
			if err == nil {
				vmMode = onboard.VMMode_Docker
			} else {
				logger.Warn("Warning: Docker requirements did not match:\n%s.\nFalling back to systemd mode for installation.\n", err.Error())
				vmMode = onboard.VMMode_Systemd
			}
		} else if vmMode == onboard.VMMode_Docker && err != nil {
			// docker mode specified explicitly but requirements didn't match
			logger.Error("failed to validate environment: %s", err.Error())
			return err
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

		vmConfigs, err := onboard.CreateClusterConfig(onboard.ClusterType_VM, userConfigPath, vmMode,
			vmAdapterTag, kubeArmorRelayServerTag, peaVersionTag, siaVersionTag,
			feederVersionTag, sumEngineVersionTag, discoverVersionTag, hardeningAgentVersionTag, kubearmorVersion, releaseVersion, kubeArmorImage,
			kubeArmorInitImage, kubeArmorVMAdapterImage, kubeArmorRelayServerImage, siaImage,
			peaImage, feederImage, rmqImage, sumEngineImage, hardeningAgentImage, spireAgentImage, waitForItImage, discoverImage, nodeAddr, dryRun,
			true, deployRMQ, imagePullPolicy, visibility, hostVisibility, sumEngineVisibility, audit, block, hostAudit, hostBlock,
			alertThrottling, maxAlertPerSec, throttleSec,
			cidr, secureContainers, skipBTF, systemMonitorPath, rmqAddress, deploySumEngine, registry, registryConfigPath, insecure, plainHTTP, preserveUpstream, topicPrefix, rmqConnectionName, sumEngineCronTime, tls, enableHostPolicyDiscovery, splunk, nodeStateRefreshTime, spireEnabled, spireCert, logRotate, parallel, enableHardeningAgent, releaseFile)
		if err != nil {
			errConfig := onboard.DumpConfig(vmConfigs, configDumpPath)
			if errConfig != nil {
				logger.Warn("Failed to create config dump at %s: %s", configDumpPath, errConfig.Error())
			}

			logger.Error("failed to create VM config: %s", err.Error())
			return err
		}
		vmConfigs.KaResource = kaResource
		vmConfigs.AgentsResource = agentsResource

		if accessKey != "" {
			if joinToken, err = vmConfigs.PopulateAccessKeyConfig(tokenURL, accessKey, topicPrefix, vmName, tokenEndpoint, "Node", insecure); err != nil {
				return err
			}
		}

		joinConfig := onboard.JoinClusterConfig(*vmConfigs, kubeArmorAddr, relayServerAddr, siaAddr, peaAddr, hardenAddr, spireHost, spireTrustBundle, joinToken, spireDir)

		defer func() {
			err := onboard.DumpConfig(joinConfig, configDumpPath)
			if err != nil {
				logger.Warn("Failed to create config dump at %s: %s", configDumpPath, err.Error())
			}
		}()

		err = joinConfig.CreateBaseNodeConfig()
		if err != nil {
			logger.Error("failed to create VM config: %s", err.Error())
			return err
		}

		switch vmMode {

		case onboard.VMMode_Systemd:
			if err := joinConfig.JoinSystemdNode(); err != nil {
				logger.Error("failed to join worker node: %s", err.Error())
				return err
			}

		case onboard.VMMode_Docker:
			err = joinConfig.JoinWorkerNode()
			if err != nil {
				logger.Error("failed to join worker node: %s", err.Error())
				return err
			}

		default:
			logger.Error("vm mode: %s invalid, accepted values (docker/systemd)", vmMode)
			return err
		}
		if enableVMScan {
			err := joinConfig.InitRRAConfig(authToken, url, tenantID, clusterID, clusterName, label, schedule, profile, benchmark, registry, registryConfigPath, insecure, plainHTTP, rraImage, rraTag, releaseVersion, preserveUpstream, true, spireAgentImage, spireHost, spireDir, knoxGateway)
			if err != nil {
				logger.Print("error creating RRA config in %s mode", vmMode)
			} else {
				err = joinConfig.InstallRRA()
				if err != nil {
					logger.Print("error installing RRA in %s mode", vmMode)
				}
			}
		}

		logger.Print("VM successfully joined with control-plane!")
		return nil
	},
}

func init() {
	// configuration for connecting with KubeArmor and control plane
	joinNodeCmd.PersistentFlags().StringVar(&kubeArmorAddr, "kubearmor-addr", "", "address of kubearmor on this node")
	joinNodeCmd.PersistentFlags().StringVar(&relayServerAddr, "relay-server-addr", "", "address of relay-server on control plane to connect with for pushing telemetry events")
	joinNodeCmd.PersistentFlags().StringVar(&siaAddr, "sia-addr", "", "address of shared-informer-agent on control plane to push state events")
	joinNodeCmd.PersistentFlags().StringVar(&peaAddr, "pea-addr", "", "address of policy-enforcement-agent on control plane for receiving policy events")
	joinNodeCmd.PersistentFlags().StringVar(&hardenAddr, "harden-addr", "", "address of hardening-agent on control plane for receiving state events")
	joinNodeCmd.PersistentFlags().StringVar(&nodeAddr, "cp-node-addr", "", "address of control plane")

	joinNodeCmd.PersistentFlags().StringVarP(&releaseVersion, "version", "v", "", "version to use - recommended to keep same as control plane node version")

	joinNodeCmd.PersistentFlags().StringVar(&tls.CaCert, "ca-cert", "", "ca certificate in bas64 encoded format to validate tls connection")

	joinNodeCmd.PersistentFlags().BoolVar(&deploySumEngine, "deploy-summary-engine", false, "to deploy summary engine in worker node")

	// spire config - to use spire and spire cert for tls
	joinNodeCmd.PersistentFlags().BoolVar(&spireEnabled, "spire", false, "enable spire")

	joinNodeCmd.PersistentFlags().BoolVar(&spireCert, "spire-cert", false, "spire cert in base64 encoded format")

	// License Key configurations
	joinNodeCmd.PersistentFlags().StringVar(&accessKey, "license-key", "", "license-key for onboarding")
	joinNodeCmd.PersistentFlags().StringVar(&tokenURL, "license-key-url", "", "license-key-url for onboarding")
	joinNodeCmd.PersistentFlags().StringVar(&tokenEndpoint, "license-key-endpoint", "/access-token/api/v1/process", "license-key-endpoint for onboarding")

	joinNodeCmd.MarkFlagsRequiredTogether("license-key", "license-key-url")

	onboardVMCmd.AddCommand(joinNodeCmd)
}
