package discoveryengine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	//pb "github.com/accuknox/auto-policy-discovery/src/protobuf/v1/license"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/kubearmor/kubearmor-client/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var matchLabels = map[string]string{"app": "discovery-engine"}
var port int64 = 9089
var cursorcount int

func InstallLicense(client *k8s.Client, key string, user string) error {
	gRPC := ""
	targetSvc := "discovery-engine"

	if val, ok := os.LookupEnv("DISCOVERY_SERVICE"); ok {
		gRPC = val
	} else {
		pf, err := utils.InitiatePortForward(client, port, port, matchLabels, targetSvc)
		if err != nil {
			return err
		}
		gRPC = "localhost:" + strconv.FormatInt(pf.LocalPort, 10)
	}

	conn, err := grpc.Dial(gRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	//licenseClient := pb.NewLicenseClient(conn)
	//
	//req := &pb.LicenseInstallRequest{
	//	Key:    key,
	//	UserId: user,
	//}
	//_, err = licenseClient.InstallLicense(context.Background(), req)
	if err != nil {
		return err
	}
	fmt.Printf("🥳  License installed successfully for discovery engine.\n")

	return nil
}

func CheckPods(client *k8s.Client) int {
	cursor := [4]string{"|", "/", "—", "\\"}
	fmt.Printf("\r😋\tChecking if DiscoveryEngine pods are running ...")
	stime := time.Now()
	otime := stime.Add(600 * time.Second)
	for {
		time.Sleep(200 * time.Millisecond)
		pods, _ := client.K8sClientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{LabelSelector: "discovery-engine", FieldSelector: "status.phase!=Running"})
		podno := len(pods.Items)
		clearLine(90)
		fmt.Printf("\rDiscovery Engine pods left to run : %d ... %s", podno, cursor[cursorcount])
		cursorcount++
		if cursorcount == 4 {
			cursorcount = 0
		}
		if !otime.After(time.Now()) {
			fmt.Printf("\r⌚️\tCheck Incomplete due to Time-Out!                     \n")
			break
		}
		if podno == 0 {
			fmt.Printf("\r🥳      Done Checking , ALL Services are running!             \n")
			fmt.Printf("⌚️\tExecution Time : %s \n", time.Since(stime))
			break
		}
	}
	return 0
}

func clearLine(size int) int {
	for i := 0; i < size; i++ {
		fmt.Printf(" ")
	}
	fmt.Printf("\r")
	return 0
}

func K8sInstaller(c *k8s.Client) error {
	//namespace
	env := k8s.AutoDetectEnvironment(c)
	if env == "none" {
		return errors.New("unsupported environment or cluster not configured correctly")
	}
	fmt.Println("😄\tAuto Detected Environment for DEv2 : "+env, true)

	// Check if the namespace already exists
	ns := "accuknox-agents"
	if _, err := c.K8sClientset.CoreV1().Namespaces().Get(context.Background(), ns, metav1.GetOptions{}); err != nil {
		// Create namespace when doesn't exist
		fmt.Println("🚀\tCreating namespace "+ns+"  ", true)
		newns := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
			},
		}
		if _, err := c.K8sClientset.CoreV1().Namespaces().Create(context.Background(), &newns, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("failed to create namespace %s: %+v", ns, err)
		}
	}

	//configmap
	configMaps, err := GetConfigmap(ns)
	if err != nil {
		return err
	}
	fmt.Println("🚀\tSetting Configmaps ", true)
	for _, cm := range configMaps {
		if _, err := c.K8sClientset.CoreV1().ConfigMaps(cm.namespace).Get(context.Background(), cm.name, metav1.GetOptions{}); err == nil {
			continue
		}
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cm.name,
				Namespace: cm.namespace,
			},
			Data: cm.data,
		}

		_, err := c.K8sClientset.CoreV1().ConfigMaps(cm.namespace).Create(context.Background(), configMap, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to set configmaps in namespace %s: %+v", ns, err)
		}
	}

	//Custom resource definition
	fmt.Println("🚀\tSetting CRD's ", true)
	crd := GetCRD()
	crdName := "discoveredpolicies.security.kubearmor.com"
	_, err = c.APIextClientset.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), crdName, metav1.GetOptions{})
	if err != nil {
		_, err := c.APIextClientset.ApiextensionsV1().CustomResourceDefinitions().Create(context.TODO(), crd, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create custom resource definitions %+v", err)
		}
	}

	//service account
	accountName := "dev2"
	fmt.Println("🚀\tCreating Service Account ", true)
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      accountName,
			Namespace: ns,
		},
	}
	_, err = c.K8sClientset.CoreV1().ServiceAccounts(ns).Get(context.Background(), accountName, metav1.GetOptions{})
	if err != nil {
		_, err := c.K8sClientset.CoreV1().ServiceAccounts(ns).Create(context.Background(), serviceAccount, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create service accounts in namespace %s: %+v", ns, err)
		}

	}

	//Cluster Role
	fmt.Println("🚀\tCreating Cluster Roles ", true)
	clusterRoleViewName := "dev2-view-cluster-resources"
	clusterRoleManageName := "dev2-manage-policies"
	clusterRoleView := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleViewName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{
					"pods",
					"services",
					"deployments",
					"namespaces",
					"nodes",
					"replicasets",
					"statefulsets",
					"daemonsets",
					"configmaps",
					"jobs",
					"cronjobs",
				},
				Verbs: []string{"get", "list", "watch"},
			},
		},
	}

	clusterRoleManage := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleManageName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"cilium.io"},
				Resources: []string{"ciliumnetworkpolicies"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"networking.k8s.io"},
				Resources: []string{"networkpolicies"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"security.kubearmor.com"},
				Resources: []string{"discoveredpolicies"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"security.kubearmor.com"},
				Resources: []string{"discoveredpolicies/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"security.kubearmor.com"},
				Resources: []string{"discoveredpolicies/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"security.kubearmor.com"},
				Resources: []string{"kubearmorpolicies"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
		},
	}
	_, err = c.K8sClientset.RbacV1().ClusterRoles().Get(context.TODO(), clusterRoleViewName, metav1.GetOptions{})
	if err != nil {
		_, err = c.K8sClientset.RbacV1().ClusterRoles().Create(context.TODO(), clusterRoleView, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create cluster roles in namespace %s: %+v", ns, err)
		}
	}

	_, err = c.K8sClientset.RbacV1().ClusterRoles().Get(context.TODO(), clusterRoleManageName, metav1.GetOptions{})
	if err != nil {
		_, err = c.K8sClientset.RbacV1().ClusterRoles().Create(context.TODO(), clusterRoleManage, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create cluster roles in namespace %s: %+v", ns, err)
		}
	}

	//cluster role binding
	fmt.Println("🚀\tCreating Cluster Role Binding ", true)

	clusterRoleBindingView := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleViewName,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRoleViewName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      accountName,
				Namespace: ns,
			},
		},
	}

	clusterRoleBindingManage := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleManageName,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRoleManageName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      accountName,
				Namespace: ns,
			},
		},
	}
	_, err = c.K8sClientset.RbacV1().ClusterRoleBindings().Get(context.TODO(), clusterRoleViewName, metav1.GetOptions{})
	if err != nil {
		_, err = c.K8sClientset.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBindingView, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create cluster role bindings in namespace %s: %+v", ns, err)
		}
	}

	_, err = c.K8sClientset.RbacV1().ClusterRoleBindings().Get(context.TODO(), clusterRoleManageName, metav1.GetOptions{})
	if err != nil {
		_, err = c.K8sClientset.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBindingManage, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create cluster role bindings in namespace %s: %+v", ns, err)
		}
	}

	//deployments
	fmt.Println("🚀\tDeployements in Progress", true)

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
							Name:    "summary-engine",
							Image:   "accuknox/dev2-sumengine:latest",
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
						// Add other containers as needed
					},
					Volumes: []corev1.Volume{
						{
							Name: "config-sumengine",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "dev2-sumengine",
									},
								},
							},
						},
						// Add other volumes as needed
					},
					ServiceAccountName:            accountName,
					TerminationGracePeriodSeconds: int64Ptr(10),
				},
			},
		},
	}

	_, err = c.K8sClientset.AppsV1().Deployments(ns).Get(context.TODO(), accountName, metav1.GetOptions{})
	if err != nil {
		_, err = c.K8sClientset.AppsV1().Deployments(ns).Create(context.TODO(), deployment, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create deployements in namespace %s: %+v", ns, err)

		}
	}

	//service
	fmt.Println("🚀\tCreating services", true)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      accountName,
			Namespace: ns,
			Labels: map[string]string{
				"app": accountName,
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "grpc",
					Port:       8090,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(8090),
				},
				{
					Name:       "amqp",
					Port:       5672,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(5672),
				},
			},
			Selector: map[string]string{
				"app": accountName,
			},
		},
	}
	_, err = c.K8sClientset.CoreV1().Services(ns).Get(context.TODO(), accountName, metav1.GetOptions{})
	if err != nil {
		_, err = c.K8sClientset.CoreV1().Services(ns).Create(context.TODO(), service, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create services in namespace %s: %+v", ns, err)
		}
	}
	return nil
}
func int32Ptr(i int32) *int32 {
	return &i
}
func int64Ptr(i int64) *int64 {
	return &i
}
func boolPtr(i bool) *bool {
	return &i
}
func apiextInactivePtr() *apiextv1.JSON {
	x := &apiextv1.JSON{
		Raw: []byte("\"Inactive\""),
	}
	return x
}
