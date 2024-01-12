package cmd

import (
	"fmt"

	dev2install "github.com/accuknox/accuknox-cli-v2/pkg/install"
	"github.com/kubearmor/kubearmor-client/install"
	"github.com/spf13/cobra"
)

var (
	installOptions         install.Options
	key                    string
	user                   string
	discoveryEngineOptions dev2install.Options
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Kubearmor, Discovery Engine and License",
	Long:  `The install command will download and install KubeArmor and AccuKnox's Discovery Engine.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Installing Kubearmor
		if err := installOptions.Env.CheckAndSetValidEnvironmentOption(cmd.Flag("env").Value.String()); err != nil {
			return fmt.Errorf("error in checking environment option: %v", err)
		}
		if err := install.K8sInstaller(client, installOptions); err != nil {
			return err
		}

		if err := dev2install.DiscoveryEngine(client, discoveryEngineOptions); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
	//license
	installCmd.Flags().StringVar(&key, "key", "", "license key for installing license (required)")
	installCmd.Flags().StringVar(&user, "user", "", "user id for installing license")
	//kubearmor
	installCmd.Flags().StringVarP(&installOptions.Namespace, "namespace", "n", "kubearmor", "Namespace for resources")
	installCmd.Flags().StringVarP(&installOptions.KubearmorImage, "image", "i", "kubearmor/kubearmor:stable", "Kubearmor daemonset image to use")
	installCmd.Flags().StringVarP(&installOptions.InitImage, "init-image", "", "kubearmor/kubearmor-init:stable", "Kubearmor daemonset init container image to use")
	installCmd.Flags().StringVarP(&installOptions.Tag, "tag", "t", "", "Change image tag/version for default kubearmor images (This will overwrite the tags provided in --image/--init-image)")
	installCmd.Flags().StringVarP(&installOptions.Audit, "audit", "a", "", "Kubearmor Audit Posture Context [all,file,network,capabilities]")
	installCmd.Flags().StringVarP(&installOptions.Block, "block", "b", "", "Kubearmor Block Posture Context [all,file,network,capabilities]")
	installCmd.Flags().StringVarP(&installOptions.Visibility, "viz", "", "", "Kubearmor Telemetry Visibility [process,file,network,none]")
	installCmd.Flags().BoolVar(&installOptions.Save, "save", false, "Save KubeArmor Manifest ")
	installCmd.Flags().BoolVar(&installOptions.Verify, "verify", true, "Verify whether all KubeArmor resources are created, running and also probes whether KubeArmor has armored the cluster or not")
	installCmd.Flags().BoolVar(&installOptions.Local, "local", false, "Use Local KubeArmor Images (sets ImagePullPolicy to 'IfNotPresent') ")
	installCmd.Flags().StringVarP(&installOptions.Env.Environment, "env", "e", "", "Supported KubeArmor Environment [k0s,k3s,microK8s,minikube,gke,bottlerocket,eks,docker,oke,generic]")
	//dev2
	installCmd.Flags().StringVarP(&discoveryEngineOptions.Tag, "release-tag", "", "", "Release tag for Discovery Engine that is to be installed")
	installCmd.Flags().BoolVar(&discoveryEngineOptions.ListTags, "list-tags", false, "List the latest 3 rolling release tags of Discovery Engine")
	installCmd.Flags().BoolVar(&discoveryEngineOptions.Debug, "debug", false, "Debug will not clean up Discovery Engine's resources in case deployment fails and will print extra info about the resources installed.")
}
