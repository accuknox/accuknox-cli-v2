package install

import (
	"fmt"

	"github.com/kubearmor/kubearmor-client/k8s"
)

// Remove will uninstall discovery engine from cluster
func Remove(client *k8s.Client) error {
	fmt.Println("Uninstalling Discovery Engine...")

	rollbackDeployment(client)
	fmt.Println("Uninstalled!")

	return nil
}
