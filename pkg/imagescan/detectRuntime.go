package imagescan

import (
	"os"

	"github.com/kubearmor/KubeArmor/KubeArmor/common"
)

func DiscoverRuntime(pathPrefix string, k8sRuntime string) (string, []string, bool) {

	var detected = false
	runtime, criPath := detectRuntimeViaMap(pathPrefix, k8sRuntime)

	if runtime != "" && len(criPath) > 0 {
		detected = true
		return runtime, criPath, detected
	}
	return runtime, criPath, detected
}

func detectRuntimeViaMap(pathPrefix string, runtime string) (string, []string) {
	var sockPaths []string
	if runtime != "" {
		for _, path := range common.ContainerRuntimeSocketMap[runtime] {
			if _, err := os.Stat(pathPrefix + path); err == nil || os.IsPermission(err) {
				if runtime == "docker" {
					path = "unix://" + path
				}
				sockPaths = append(sockPaths, path)
			}
		}
	}
	return runtime, sockPaths
}
