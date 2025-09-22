package imagescan

import (
	"os"

	"github.com/kubearmor/KubeArmor/KubeArmor/common"
)

func DiscoverRuntime(pathPrefix string, k8sRuntime string) (string, []string, bool) {
	runtime, criPath := detectRuntimeViaMap(pathPrefix, k8sRuntime)
	detected := runtime != "" && len(criPath) > 0
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
