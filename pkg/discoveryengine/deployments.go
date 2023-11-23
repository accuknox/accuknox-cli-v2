package discoveryengine

import (
	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getDeployments(accountName string, ns string) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      accountName,
			Namespace: ns,
			Labels: map[string]string{
				"app": accountName,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": accountName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": accountName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    common.SumEngine,
							Image:   common.SumEngineImage,
							Command: []string{"/usr/bin/sumengine"},
							Args:    []string{"--config", "/var/lib/sumengine/app.yaml", "--kmux-config", "/var/lib/sumengine/kmux.yaml"},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									"cpu":    resource.MustParse("500m"),
									"memory": resource.MustParse("1Gi"),
								},
								Requests: corev1.ResourceList{
									"cpu":    resource.MustParse("100m"),
									"memory": resource.MustParse("100Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: "/var/lib/sumengine/",
									Name:      "config-sumengine",
									ReadOnly:  true,
								},
							},
						},
						{
							Name:    common.Offlaoder,
							Image:   common.OffloaderImage,
							Command: []string{"/usr/bin/offloader"},
							Args:    []string{"--config", "/var/lib/offloader/app.yaml", "--kmux-config", "/var/lib/offloader/kmux.yaml"},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: common.GRPCPort,
									Name:          common.GRPC,
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									"cpu":    resource.MustParse("500m"),
									"memory": resource.MustParse("1Gi"),
								},
								Requests: corev1.ResourceList{
									"cpu":    resource.MustParse("100m"),
									"memory": resource.MustParse("100Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config-offloader",
									MountPath: "/var/lib/offloader/",
									ReadOnly:  true,
								},
							},
						},
						{
							Name:    common.Discover,
							Image:   common.DiscoverImage,
							Command: []string{"/usr/bin/discover"},
							Args:    []string{"--config", "/var/lib/discover/app.yaml", "--kmux-config", "/var/lib/discover/kmux.yaml"},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8090,
									Name:          "grpc",
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									"cpu":    resource.MustParse("500m"),
									"memory": resource.MustParse("1Gi"),
								},
								Requests: corev1.ResourceList{
									"cpu":    resource.MustParse("200m"),
									"memory": resource.MustParse("200Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config-discover",
									MountPath: "/var/lib/discover/",
									ReadOnly:  true,
								},
							},
						},
						{
							Name:  common.Rabbitmq,
							Image: common.RabbitmqImage,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: common.AMQPPort,
									Name:          common.AMQP,
								},
								{
									ContainerPort: common.ManagementPort,
									Name:          common.Management,
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									"cpu":    resource.MustParse("500m"),
									"memory": resource.MustParse("1Gi"),
								},
								Requests: corev1.ResourceList{
									"cpu":    resource.MustParse("100m"),
									"memory": resource.MustParse("100Mi"),
								},
							},
						},
						{
							Name:    common.Hardening,
							Image:   common.HardeningImage,
							Command: []string{"/usr/bin/hardening", "start"},
							Args:    []string{"--config", "/var/lib/hardening/app.yaml", "--kmux-config", "/var/lib/hardening/kmux.yaml"},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									"cpu":    resource.MustParse("500m"),
									"memory": resource.MustParse("1Gi"),
								},
								Requests: corev1.ResourceList{
									"cpu":    resource.MustParse("100m"),
									"memory": resource.MustParse("100Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config-hardening",
									MountPath: "/var/lib/hardening/",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config-sumengine",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: common.SumengineConfmap,
									},
								},
							},
						},
						{
							Name: "config-offloader",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: common.OffloaderConfMap,
									},
								},
							},
						},
						{
							Name: "config-discover",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: common.DiscoverConfMap,
									},
								},
							},
						},
						{
							Name: "config-hardening",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: common.HardeningConfMap,
									},
								},
							},
						},
					},
					ServiceAccountName:            accountName,
					TerminationGracePeriodSeconds: int64Ptr(10),
				},
			},
		},
	}
	return deployment
}
