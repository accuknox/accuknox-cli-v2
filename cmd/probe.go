// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	dockerprobe "github.com/accuknox/accuknox-cli-v2/pkg/probe"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/kubearmor/kubearmor-client/probe"
	mobyclient "github.com/moby/moby/client"
	"github.com/spf13/cobra"
)

var probeInstallOptions probe.Options
var probeVMMode string

// probeCmd represents the get command
var probeCmd = &cobra.Command{
	Use:               "probe",
	PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
	Short:             "Checks for supported KubeArmor features in the current environment",
	Long: `Checks for supported KubeArmor features in the current environment.

If KubeArmor is not running, it does a precheck to know if kubearmor will be supported in the environment
and what KubeArmor features will be supported e.g: observability, enforcement, etc. 
	 
If KubeArmor is running, It probes which environment KubeArmor is running on (Docker, systemd, or Kubernetes),
the supported KubeArmor features in the environment, the pods being handled by KubeArmor and the policies running on each of these pods`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if probeVMMode != "auto" && probeVMMode != "docker" && probeVMMode != "systemd" {
			return fmt.Errorf("--vm-mode must be auto, docker, or systemd")
		}
		if probeInstallOptions.Output != "" && probeInstallOptions.Output != "json" && probeInstallOptions.Output != "no-color" {
			return fmt.Errorf("--output must be json or no-color")
		}
		probeInstallOptions.Writer = cmd.OutOrStdout()
		systemdRunning := false
		if probeVMMode != "docker" {
			ctx, cancel := context.WithTimeout(cmd.Context(), 3*time.Second)
			systemdRunning = exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", "kubearmor").Run() == nil
			cancel()
		}
		if probeVMMode == "systemd" && !systemdRunning {
			return fmt.Errorf("KubeArmor is not running as a systemd service")
		}
		if probeVMMode == "docker" || (probeVMMode == "auto" && !systemdRunning) {
			found, err := dockerprobe.PrintDocker(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), dockerprobe.DockerOptions{Output: probeInstallOptions.Output, GRPC: probeInstallOptions.GRPC})
			if err != nil && (errors.Is(err, os.ErrPermission) || !mobyclient.IsErrConnectionFailed(err)) {
				return err
			}
			if found || probeVMMode == "docker" {
				if err != nil {
					return err
				}
				if !found {
					return fmt.Errorf("no Docker KubeArmor container found")
				}
				return nil
			}
		}
		var kubeClient *k8s.Client
		if !systemdRunning {
			var err error
			kubeClient, err = k8s.ConnectK8sClient()
			if err != nil {
				return err
			}
		}
		if err := probe.PrintProbeResultCmd(kubeClient, probeInstallOptions); err != nil {
			return err
		}
		return nil

	},
}

func init() {
	rootCmd.AddCommand(probeCmd)
	probeCmd.Flags().StringVar(&probeVMMode, "vm-mode", "auto", "installation mode: auto, docker, or systemd")
	probeCmd.Flags().StringVar(&probeInstallOptions.Output, "output", "", "output format: json or no-color (default: table)")
	probeCmd.Flags().StringVar(&probeInstallOptions.GRPC, "grpc", "", "KubeArmor gRPC endpoint (default: KUBEARMOR_SERVICE or localhost:32767)")
	probeCmd.Flags().StringVarP(&probeInstallOptions.Namespace, "namespace", "n", "kube-system", "Namespace for resources")
	probeCmd.Flags().BoolVar(&probeInstallOptions.Full, "full", false, `If KubeArmor is not running, it deploys a daemonset to have access to more
information on KubeArmor support in the environment and deletes daemonset after probing`)
}
