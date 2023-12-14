package top

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/kubearmor/kubearmor-client/k8s"
	"k8s.io/metrics/pkg/client/clientset/versioned"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

type Options struct {
	RealTimeUpdateInterval int
}

type PodInfo struct {
	Name     string
	Status   string
	NodeName string
	Age      string
	Ready    string
	QoSClass string
	Metrics  []ContainerMetrics
}

type ContainerMetrics struct {
	Name        string
	CPU         int64 // MilliValue
	Memory      int64 // MiB
	Restarts    int32
	CPULimit    int64 // MilliValue
	MemoryLimit int64 // MiB
}

func createMetricsClient(k8sClient *k8s.Client) (*versioned.Clientset, error) {
	metricsClient, err := versioned.NewForConfig(k8sClient.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics client: %v", err)
	}

	return metricsClient, nil
}

func Top(k8sClient *k8s.Client, opts Options) error {
	k8sClientGlobal = k8sClient

	return runRealTimeTop(opts)
}

func fetchPodMetrics() ([]PodInfo, error) {
	metricsClient, err := createMetricsClient(k8sClientGlobal)
	if err != nil {
		return nil, err
	}

	pods, err := k8sClientGlobal.K8sClientset.CoreV1().Pods("accuknox-agents").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing pods in 'accuknox-agents' namespace: %v", err)
	}

	podMetricsList, err := metricsClient.MetricsV1beta1().PodMetricses("accuknox-agents").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing pod metrics in 'accuknox-agents' namespace: %v", err)
	}

	podMetricsMap := make(map[string]*metricsv1beta1.PodMetrics)
	for _, pm := range podMetricsList.Items {
		pmCopy := pm
		podMetricsMap[pm.Name] = &pmCopy
	}

	var podInfos []PodInfo

	for _, pod := range pods.Items {
		podStatus := string(pod.Status.Phase)
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
				podStatus = cs.State.Waiting.Reason
			} else if cs.State.Terminated != nil && cs.State.Terminated.Reason != "" {
				podStatus = cs.State.Terminated.Reason
			}
		}

		containerMetricsMap := make(map[string]ContainerMetrics)
		for _, containerStatus := range pod.Status.ContainerStatuses {
			containerMetricsMap[containerStatus.Name] = ContainerMetrics{
				Name:     containerStatus.Name,
				Restarts: containerStatus.RestartCount,
			}
		}

		for _, containerSpec := range pod.Spec.Containers {
			if cm, exists := containerMetricsMap[containerSpec.Name]; exists {
				cpuLimit := containerSpec.Resources.Limits.Cpu().MilliValue()
				memLimit := containerSpec.Resources.Limits.Memory().Value() / 1024 / 1024
				cm.CPULimit = cpuLimit
				cm.MemoryLimit = memLimit
				containerMetricsMap[containerSpec.Name] = cm
			}
		}

		var containerMetrics []ContainerMetrics
		if pm, found := podMetricsMap[pod.Name]; found {
			for _, c := range pm.Containers {
				if cm, exists := containerMetricsMap[c.Name]; exists {
					cm.CPU = c.Usage.Cpu().MilliValue()
					cm.Memory = c.Usage.Memory().Value() / 1024 / 1024
					containerMetrics = append(containerMetrics, cm)
				}
			}
		}

		ageDuration := time.Since(pod.CreationTimestamp.Time)
		ageHours := ageDuration.Hours()
		formattedAge := fmt.Sprintf("%.0fh", ageHours)

		podInfos = append(podInfos, PodInfo{
			Name:     pod.Name,
			Status:   podStatus,
			NodeName: pod.Spec.NodeName,
			Age:      formattedAge,
			QoSClass: string(pod.Status.QOSClass),
			Metrics:  containerMetrics,
		})
	}
	sortedPodInfos := sortPodInfosByMaxCPU(podInfos)

	return sortedPodInfos, nil
}

// Sorting is not properly working for some reason, it mostly has to do with how
// lipgloss renders the table.

func sortPodInfosByMaxCPU(podInfos []PodInfo) []PodInfo {
	sort.Slice(podInfos, func(i, j int) bool {
		return maxContainerCPU(podInfos[i].Metrics) > maxContainerCPU(podInfos[j].Metrics)
	})
	return podInfos
}

func maxContainerCPU(metrics []ContainerMetrics) int64 {
	var maxCPU int64
	for _, m := range metrics {
		if m.CPU > maxCPU {
			maxCPU = m.CPU
		}
	}
	return maxCPU
}
