package cmd

import (
	"fmt"

	"github.com/accuknox/accuknox-cli-v2/pkg/discoveryengine"
	"github.com/kubearmor/kubearmor-client/install"
	"github.com/spf13/cobra"
)

var (
	installOptions     install.Options
	dev2InstallOptions discoveryengine.Options
	key                string
	user               string
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Kubearmor, Discovery Engine and License",
	Long:  `Discover applicable policies`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Installing Kubearmor
		if err := installOptions.Env.CheckAndSetValidEnvironmentOption(cmd.Flag("env").Value.String()); err != nil {
			return fmt.Errorf("error in checking environment option: %v", err)
		}
		if err := install.K8sInstaller(client, installOptions); err != nil {
			return err
		}
		//installing Dev2
		if err := discoveryengine.K8sInstaller(client, dev2InstallOptions); err != nil {
			return err
		}
		discoveryengine.CheckPods(client)

		//installing license
		if user == "" || key == "" {
			return nil
		}

		err := discoveryengine.InstallLicense(client, key, user)
		return err
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
	//license
	installCmd.Flags().StringVar(&key, "key", "", "license key for installing license (required)")
	installCmd.Flags().StringVar(&user, "user", "", "user id for installing license")
	//kubearmor
	installCmd.Flags().StringVarP(&installOptions.Namespace, "kubearmor-namespace", "n", "kube-system", "Namespace for resources")
	installCmd.Flags().StringVarP(&installOptions.KubearmorImage, "image", "i", "kubearmor/kubearmor:stable", "Kubearmor daemonset image to use")
	installCmd.Flags().StringVarP(&installOptions.InitImage, "init-image", "", "kubearmor/kubearmor-init:stable", "Kubearmor daemonset init container image to use")
	installCmd.Flags().StringVarP(&installOptions.Tag, "tag", "t", "", "Change image tag/version for default kubearmor images (This will overwrite the tags provided in --image/--init-image)")
	installCmd.Flags().StringVarP(&installOptions.Audit, "audit", "a", "", "Kubearmor Audit Posture Context [all,file,network,capabilities]")
	installCmd.Flags().StringVarP(&installOptions.Block, "block", "b", "", "Kubearmor Block Posture Context [all,file,network,capabilities]")
	installCmd.Flags().BoolVar(&installOptions.Save, "save", false, "Save KubeArmor Manifest ")
	installCmd.Flags().BoolVar(&installOptions.Local, "local", false, "Use Local KubeArmor Images (sets ImagePullPolicy to 'IfNotPresent') ")
	installCmd.Flags().StringVarP(&installOptions.Env.Environment, "env", "e", "", "Supported KubeArmor Environment [k3s,microK8s,minikube,gke,bottlerocket,eks,docker,oke,generic]")
	//dev2
	installCmd.Flags().StringVarP(&dev2InstallOptions.Namespace, "dev2-namespace", "p", "accuknox-agents", "Namespace for resources")
	installCmd.Flags().StringVarP(&dev2InstallOptions.AccountName, "dev2-name", "q", "dev2", "Name of the service account")

}
