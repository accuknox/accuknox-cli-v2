package discoveryengine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/accuknox/accuknox-cli-v2/pkg"
	"github.com/kubearmor/kubearmor-client/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func checkTerminatingPods(c *k8s.Client) int {
	cursor := [4]string{"|", "/", "—", "\\"}
	fmt.Printf("🔴   Checking if Dev2 pods are stopped ...")
	stime := time.Now()
	otime := stime.Add(600 * time.Second)
	for {
		time.Sleep(200 * time.Millisecond)
		pods, _ := c.K8sClientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{LabelSelector: "app=dev2", FieldSelector: "status.phase=Running"})
		podno := len(pods.Items)
		fmt.Printf("\r")
		clearLine(90)
		fmt.Printf("\rDev2 pods left to stop : %d ... %s  ", podno, cursor[cursorcount])
		cursorcount++
		if cursorcount == 4 {
			cursorcount = 0
		}
		if !otime.After(time.Now()) {
			fmt.Printf("\r⌚️  Check Incomplete due to Time-Out!                     \n")
			break
		}
		if podno == 0 {
			fmt.Printf("\r🔴   Done Checking , ALL Services are stopped!             \n")
			fmt.Printf("⌚️   Termination Time : %s \n", time.Since(stime))
			break
		}
	}
	return 0
}

func K8sUninstaller(c *k8s.Client, o Options) error {

	fmt.Print("\n❌   dev2 Deployments ...\n")
	kaDeployments, _ := c.K8sClientset.AppsV1().Deployments("").List(context.TODO(), metav1.ListOptions{LabelSelector: "app=dev2"})
	for _, d := range kaDeployments.Items {
		if err := c.K8sClientset.AppsV1().Deployments(d.Namespace).Delete(context.Background(), d.Name, metav1.DeleteOptions{}); err != nil {
			fmt.Printf("ℹ️   Error while uninstalling dev2 Deployment %s : %s\n", d.Name, err.Error())
		}
	}

	fmt.Print("❌   dev2 Controller Roles ...\n")
	if err := c.K8sClientset.RbacV1().ClusterRoles().Delete(context.Background(), pkg.ClusterRoleViewName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			fmt.Print("Error while uninstalling dev2 Controller Cluster Role\n")
		}
	}

	if err := c.K8sClientset.RbacV1().ClusterRoleBindings().Delete(context.Background(), pkg.ClusterRoleViewName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			fmt.Print("Error while uninstalling dev2 Controller Cluster Role Bindings\n")
		}
	}

	if err := c.K8sClientset.RbacV1().ClusterRoles().Delete(context.Background(), pkg.ClusterRoleManageName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			fmt.Print("Error while uninstalling dev2 Controller Cluster Role\n")
		}
	}

	if err := c.K8sClientset.RbacV1().ClusterRoleBindings().Delete(context.Background(), pkg.ClusterRoleManageName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			fmt.Print("Error while uninstalling dev2 Controller Cluster Role Bindings\n")
		}
	}

	fmt.Print("❌   dev2 Service Account ...\n")
	if err := c.K8sClientset.CoreV1().ServiceAccounts(o.Namespace).Delete(context.Background(), pkg.ServiceAccountName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("ℹ️   dev2 Controller Service Account not found ...\n")
	}

	fmt.Print("❌   dev2 Service ...\n")
	if err := c.K8sClientset.CoreV1().Services(o.Namespace).Delete(context.Background(), pkg.ServiceAccountName, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("ℹ️   dev2 Relay Service not found ...\n")
	}

	fmt.Print("❌   Offloader ConfigMap ...\n")
	if err := c.K8sClientset.CoreV1().ConfigMaps(o.Namespace).Delete(context.Background(), pkg.OffloaderConfMap, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("ℹ️   Offloader ConfigMap not found ...\n")
	}

	fmt.Print("❌   Discover ConfigMap ...\n")
	if err := c.K8sClientset.CoreV1().ConfigMaps(o.Namespace).Delete(context.Background(), pkg.DiscoverConfMap, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("ℹ️   Discover ConfigMap not found ...\n")
	}
	fmt.Print("❌   Hardening ConfigMap ...\n")
	if err := c.K8sClientset.CoreV1().ConfigMaps(o.Namespace).Delete(context.Background(), pkg.HardeningConfMap, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("ℹ️   Hardening ConfigMap not found ...\n")
	}
	fmt.Print("❌   Sumengine ConfigMap ...\n")
	if err := c.K8sClientset.CoreV1().ConfigMaps(o.Namespace).Delete(context.Background(), pkg.SumengineConfmap, metav1.DeleteOptions{}); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
		fmt.Print("ℹ️   Sumengine ConfigMap not found ...\n")
	}

	checkTerminatingPods(c)

	return nil
}
