package cmd

import (
	"fmt"

	cm "github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/spf13/cobra"
)

var (
	kubeArmorAddr   string
	relayServerAddr string
	siaAddr         string
	peaAddr         string
)

// joinNodeCmd represents the join command
var joinNodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Join this worker node with the control plane node for onboarding onto SaaS",
	Long:  "Join this worker node with the control plane node for onboarding onto SaaS",
	RunE: func(cmd *cobra.Command, args []string) error {
		// need at least either one of the below flags
		if nodeAddr == "" && (siaAddr == "" || relayServerAddr == "" || peaAddr == "") {
			return fmt.Errorf("cp-node-addr (control-plane address) or address of each agent must be specified")
		}
		// validate environment for pre-requisites
		var (
			cc               onboard.ClusterConfig
			secureContainers = true
		)
		_, err := cc.ValidateEnv()
		if vmMode == "" {
			if err == nil {
				vmMode = onboard.VMMode_Docker
			} else {
				fmt.Printf("Warning: Docker requirements did not match: %s. Falling back to systemd mode for installation.\n", err.Error())
				vmMode = onboard.VMMode_Systemd
				secureContainers = false
			}
		} else if vmMode == onboard.VMMode_Systemd {
			// systemd mode explicitly specified
			if err != nil {
				// docker requirements didn't meet - containers won't be protected
				secureContainers = false
			}
		} else if vmMode == onboard.VMMode_Docker && err != nil {
			// docker mode specified explicitly but requirements didn't match
			return fmt.Errorf("failed to validate environment: %s", err.Error())
		}
		vmConfigs, err := onboard.CreateClusterConfig(onboard.ClusterType_VM, userConfigPath, vmMode,
			vmAdapterTag, kubeArmorRelayServerTag, peaVersionTag, siaVersionTag,
			feederVersionTag, kubearmorVersion, releaseVersion, kubeArmorImage,
			kubeArmorInitImage, kubeArmorVMAdapterImage, kubeArmorRelayServerImage, siaImage,
			peaImage, feederImage, spireAgentImage, nodeAddr, dryRun,
			true, imagePullPolicy, visibility, hostVisibility,
			audit, block, cidr, secureContainers)
		if err != nil {
			return fmt.Errorf("failed to create VM config: %s", err.Error())
		}
		joinConfig := onboard.JoinClusterConfig(*vmConfigs, kubeArmorAddr, relayServerAddr, siaAddr, peaAddr)

		err = joinConfig.CreateBaseNodeConfig()
		if err != nil {
			return fmt.Errorf("failed to create VM config: %s", err.Error())
		}

		switch vmMode {

		case onboard.VMMode_Systemd:

			if err := joinConfig.JoinSystemdNode(); err != nil {
				return fmt.Errorf("failed to join worker node: %s", err.Error())
			}
			fmt.Println(cm.Green + "VM successfully joined with control-plane!" + cm.Reset)

		case onboard.VMMode_Docker:
			err = joinConfig.JoinWorkerNode()
			if err != nil {
				return fmt.Errorf("failed to join worker node: %s", err.Error())
			}
			fmt.Println(cm.Green + "VM successfully joined with control-plane!" + cm.Reset)

		default:
			fmt.Printf("vm mode: %s invalid, accepted values (docker/systemd)", vmMode)
		}

		return nil
	},
}

func init() {
	// configuration for connecting with KubeArmor and control plane
	joinNodeCmd.PersistentFlags().StringVar(&kubeArmorAddr, "kubearmor-addr", "", "address of kubearmor on this node")
	joinNodeCmd.PersistentFlags().StringVar(&relayServerAddr, "relay-server-addr", "", "address of relay-server on control plane to connect with for pushing telemetry events")
	joinNodeCmd.PersistentFlags().StringVar(&siaAddr, "sia-addr", "", "address of shared-informer-agent on control plane to push state events")
	joinNodeCmd.PersistentFlags().StringVar(&peaAddr, "pea-addr", "", "address of policy-enforcement-agent on control plane for receiving policy events")

	joinNodeCmd.PersistentFlags().StringVar(&nodeAddr, "cp-node-addr", "", "address of control plane")

	onboardVMCmd.AddCommand(joinNodeCmd)
}
