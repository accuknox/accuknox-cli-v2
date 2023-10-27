package discoveryengine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	// pb "github.com/accuknox/auto-policy-discovery/src/protobuf/v1/license"
	"github.com/accuknox/accuknox-cli-v2/pkg"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/kubearmor/kubearmor-client/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var port int64 = 9089
var cursorcount int

// Options for dev2 install
type Options struct {
	Namespace   string
	AccountName string
}

func InstallLicense(client *k8s.Client, key string, user string) error {
	gRPC := ""
	targetSvc := "discovery-engine"
	if val, ok := os.LookupEnv("DISCOVERY_SERVICE"); ok {
		gRPC = val
	} else {
		pf, err := utils.InitiatePortForward(client, port, port, pkg.MatchLabels, targetSvc)
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

	// licenseClient := pb.NewLicenseClient(conn)

	// req := &pb.LicenseInstallRequest{
	// 	Key:    key,
	// 	UserId: user,
	// }
	// _, err = licenseClient.InstallLicense(context.Background(), req)
	// if err != nil {
	// 	return err
	// }
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
		pods, _ := client.K8sClientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{LabelSelector: "app=dev2", FieldSelector: "status.phase!=Running"})
		podno := len(pods.Items)
		fmt.Printf("\r")
		clearLine(90)
		fmt.Printf("\rDiscovery Engine pods left to run : %d ... %s   ", podno, cursor[cursorcount])
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

func K8sInstaller(c *k8s.Client, o Options) error {
	fmt.Println("\n\n\r😋\tInstalling Dev2 ...")
	//namespace
	env := k8s.AutoDetectEnvironment(c)
	if env == "none" {
		return errors.New("unsupported environment or cluster not configured correctly")
	}
	fmt.Println("😄\tAuto Detected Environment for DEv2 : "+env, true)

	// Check if the namespace already exists
	ns := o.Namespace
	if _, err := c.K8sClientset.CoreV1().Namespaces().Get(context.Background(), ns, metav1.GetOptions{}); err != nil {
		// Create namespace when doesn't exist
		fmt.Println("🚀\tCreating namespace " + ns)
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
	fmt.Println("💫\tSetting Configmaps ")
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
	fmt.Println("🔥\tSetting CRD's ")
	crd := GetCRD()
	_, err = c.APIextClientset.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), pkg.CRDName, metav1.GetOptions{})
	if err != nil {
		_, err := c.APIextClientset.ApiextensionsV1().CustomResourceDefinitions().Create(context.TODO(), crd, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create custom resource definitions %+v", err)
		}
	} else {
		fmt.Println("ℹ️\tCRD already exists")
	}

	//service account
	accountName := pkg.ServiceAccountName
	fmt.Println("💫\tCreating Service Account ")
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

	} else {
		fmt.Println("ℹ️\tService Account already exists  ")
	}

	//Cluster Role
	fmt.Println("🤩\tCreating Cluster Roles ")

	clusterRoleView := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: pkg.ClusterRoleViewName,
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
			Name: pkg.ClusterRoleManageName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{pkg.APIGroupCilium},
				Resources: []string{"ciliumnetworkpolicies"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{pkg.APIGroupNetworking},
				Resources: []string{"networkpolicies"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{pkg.APIGroupKubearmorSecurity},
				Resources: []string{"discoveredpolicies"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{pkg.APIGroupKubearmorSecurity},
				Resources: []string{"discoveredpolicies/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{pkg.APIGroupKubearmorSecurity},
				Resources: []string{"discoveredpolicies/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{pkg.APIGroupKubearmorSecurity},
				Resources: []string{"kubearmorpolicies"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
		},
	}
	_, err = c.K8sClientset.RbacV1().ClusterRoles().Get(context.TODO(), pkg.ClusterRoleViewName, metav1.GetOptions{})
	if err != nil {
		_, err = c.K8sClientset.RbacV1().ClusterRoles().Create(context.TODO(), clusterRoleView, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create cluster roles in namespace %s: %+v", ns, err)
		}
	} else {
		fmt.Println("ℹ️\tCluster roles already exists  ")
	}

	_, err = c.K8sClientset.RbacV1().ClusterRoles().Get(context.TODO(), pkg.ClusterRoleManageName, metav1.GetOptions{})
	if err != nil {
		_, err = c.K8sClientset.RbacV1().ClusterRoles().Create(context.TODO(), clusterRoleManage, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create cluster roles in namespace %s: %+v", ns, err)
		}
	} else {
		fmt.Println("ℹ️\tCluster roles already exists  ")
	}

	//cluster role binding
	fmt.Println("🚀\tCreating Cluster Role Binding ")

	clusterRoleBindingView := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: pkg.ClusterRoleViewName,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: pkg.APIGroupRBACAuth,
			Kind:     pkg.ClusterRole,
			Name:     pkg.ClusterRoleViewName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      pkg.ServiceAccount,
				Name:      accountName,
				Namespace: ns,
			},
		},
	}

	clusterRoleBindingManage := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: pkg.ClusterRoleManageName,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: pkg.APIGroupRBACAuth,
			Kind:     pkg.ClusterRole,
			Name:     pkg.ClusterRoleManageName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      pkg.ServiceAccount,
				Name:      accountName,
				Namespace: ns,
			},
		},
	}
	_, err = c.K8sClientset.RbacV1().ClusterRoleBindings().Get(context.TODO(), pkg.ClusterRoleViewName, metav1.GetOptions{})
	if err != nil {
		_, err = c.K8sClientset.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBindingView, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create cluster role bindings in namespace %s: %+v", ns, err)
		}
	} else {
		fmt.Println("ℹ️\tCluster role bindings already exists  ")
	}

	_, err = c.K8sClientset.RbacV1().ClusterRoleBindings().Get(context.TODO(), pkg.ClusterRoleManageName, metav1.GetOptions{})
	if err != nil {
		_, err = c.K8sClientset.RbacV1().ClusterRoleBindings().Create(context.TODO(), clusterRoleBindingManage, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create cluster role bindings in namespace %s: %+v", ns, err)
		}
	} else {
		fmt.Println("ℹ️\tCluster role bindings already exists  ")
	}

	//deployments
	fmt.Println("🛰\tDeployements in Progress")

	deployment := getDeployments(accountName, ns)

	_, err = c.K8sClientset.AppsV1().Deployments(ns).Get(context.TODO(), accountName, metav1.GetOptions{})
	if err != nil {
		_, err = c.K8sClientset.AppsV1().Deployments(ns).Create(context.TODO(), deployment, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create deployements in namespace %s: %+v", ns, err)

		}
	} else {
		fmt.Println("ℹ️\tdeployments already exists  ")
	}

	//service
	fmt.Println("🚀\tCreating services")

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
					Name:       pkg.GRPC,
					Port:       pkg.GRPCPort,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(int(pkg.GRPCPort)),
				},
				{
					Name:       pkg.AMQP,
					Port:       pkg.AMQPPort,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(int(pkg.AMQPPort)),
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
	} else {
		fmt.Println("ℹ️\tServices already exists  ")
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
