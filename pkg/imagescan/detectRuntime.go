package imagescan

import (
	"os"
)

// referred from https://github.com/kubearmor/KubeArmor/blob/v1.6.3/KubeArmor/common/common.go#L428-L444
// containerRuntimeSocketMap Structure
var containerRuntimeSocketMap = map[string][]string{
	"docker": {
		"/var/run/docker.sock",
		"/run/docker.sock",
	},
	"containerd": {
		"/var/snap/microk8s/common/run/containerd.sock",
		"/run/k3s/containerd/containerd.sock",
		"/run/containerd/containerd.sock",
		"/var/run/containerd/containerd.sock",
		"/run/dockershim.sock",
	},
	"cri-o": {
		"/var/run/crio/crio.sock",
		"/run/crio/crio.sock",
	},
}

func DiscoverRuntime(pathPrefix string, k8sRuntime string) (string, []string, bool) {
	runtime, criPath := detectRuntimeViaMap(pathPrefix, k8sRuntime)
	detected := runtime != "" && len(criPath) > 0
	return runtime, criPath, detected
}

func detectRuntimeViaMap(pathPrefix string, runtime string) (string, []string) {
	var sockPaths []string
	if runtime != "" {
		for _, path := range containerRuntimeSocketMap[runtime] {
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
