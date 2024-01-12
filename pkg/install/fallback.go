package install

import (
	"context"
	"fmt"

	"github.com/kubearmor/kubearmor-client/k8s"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func rollbackDeployment(client *k8s.Client) {
	namespace := "accuknox-agents"
	labelSelector := "app=discovery-engine"

	deleteService(client, namespace, labelSelector)
	deleteDeployment(client, namespace, labelSelector)
	deleteClusterRoleBinding(client, labelSelector)
	deleteClusterRole(client, labelSelector)
	deleteServiceAccount(client, namespace, labelSelector)
	deleteCRD(client, labelSelector)
	deleteCM(client, namespace, labelSelector)
	deleteNamespace(client, namespace, labelSelector)
}

func deleteNamespace(client *k8s.Client, namespace, labelSelector string) {
	err := client.K8sClientset.CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{})
	if err != nil {
		fmt.Printf(" [⨯] Failed to delete namespaces: %v\n", errors.ReasonForError(err))
	} else {
		fmt.Printf(" [✓] Deleted namespace: %v\n", namespace)
	}
}

func deleteCM(client *k8s.Client, namespace, labelSelector string) {
	cm, err := client.K8sClientset.CoreV1().ConfigMaps(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		fmt.Printf(" [⨯] Failed to list config maps: %v\n", errors.ReasonForError(err))
	}

	for _, cms := range cm.Items {
		err := client.K8sClientset.CoreV1().ConfigMaps(namespace).Delete(context.Background(), cms.Name, metav1.DeleteOptions{})
		if err != nil {
			fmt.Printf(" [⨯] Failed to delete config map %s: %v\n", cms.Name, errors.ReasonForError(err))
		} else {
			fmt.Printf(" [✓] Deleted configmap: %s\n", cms.Name)
		}
	}
}

func deleteCRD(client *k8s.Client, labelSelector string) {
	crds, err := client.APIextClientset.ApiextensionsV1().CustomResourceDefinitions().List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		fmt.Printf(" [⨯] Failed to list custom resource definition: %v\n", errors.ReasonForError(err))
	}

	for _, crd := range crds.Items {
		err := client.APIextClientset.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), crd.Name, metav1.DeleteOptions{})
		if err != nil {
			fmt.Printf(" [⨯] Failed to delete custom resource definition %s: %v\n", crd.Name, errors.ReasonForError(err))
		} else {
			fmt.Printf(" [✓] Deleted custom resource definition: %v\n", crd.Name)
		}

	}
}

func deleteService(client *k8s.Client, namespace, labelSelector string) {
	services, err := client.K8sClientset.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		fmt.Printf(" [⨯] Failed to list services: %v\n", errors.ReasonForError(err))
	}

	for _, service := range services.Items {
		err := client.K8sClientset.CoreV1().Services(namespace).Delete(context.Background(), service.Name, metav1.DeleteOptions{})
		if err != nil {
			fmt.Printf(" [⨯] Failed to delete service %s: %v\n", service.Name, errors.ReasonForError(err))
		} else {
			fmt.Printf(" [✓] Deleted service: %s\n", service.Name)
		}
	}
}

func deleteServiceAccount(client *k8s.Client, namespace, labelSelector string) {
	serviceAccounts, err := client.K8sClientset.CoreV1().ServiceAccounts(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		fmt.Printf(" [⨯] Failed to list service accounts: %v\n", errors.ReasonForError(err))
	}

	for _, serviceAccount := range serviceAccounts.Items {
		err := client.K8sClientset.CoreV1().ServiceAccounts(namespace).Delete(context.Background(), serviceAccount.Name, metav1.DeleteOptions{})
		if err != nil {
			fmt.Printf(" [⨯] Failed to delete service account %s: %v\n", serviceAccount.Name, errors.ReasonForError(err))
		} else {
			fmt.Printf(" [✓] Deleted service account: %s\n", serviceAccount.Name)
		}
	}
}

func deleteClusterRole(client *k8s.Client, labelSelector string) {
	clusterRoles, err := client.K8sClientset.RbacV1().ClusterRoles().List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		fmt.Printf(" [⨯] Failed to list cluster roles: %v\n", errors.ReasonForError(err))
	}

	for _, clusterRole := range clusterRoles.Items {
		err := client.K8sClientset.RbacV1().ClusterRoles().Delete(context.Background(), clusterRole.Name, metav1.DeleteOptions{})
		if err != nil {
			fmt.Printf(" [⨯] Failed to delete cluster role %s: %v\n", clusterRole.Name, errors.ReasonForError(err))
		} else {
			fmt.Printf(" [✓] Deleted cluster role: %v\n", clusterRole.Name)
		}
	}
}

func deleteClusterRoleBinding(client *k8s.Client, labelSelector string) {
	clusterRoleBindings, err := client.K8sClientset.RbacV1().ClusterRoleBindings().List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		fmt.Printf(" [⨯] Failed to list cluster role bindings: %v\n", errors.ReasonForError(err))
	}

	for _, crb := range clusterRoleBindings.Items {
		err := client.K8sClientset.RbacV1().ClusterRoleBindings().Delete(context.Background(), crb.Name, metav1.DeleteOptions{})
		if err != nil {
			fmt.Printf(" [⨯] Failed to delete cluster role binding %s: %v\n", crb.Name, errors.ReasonForError(err))
		} else {
			fmt.Printf(" [✓] Deleted cluster role binding: %v\n", crb.Name)
		}
	}
}

func deleteDeployment(client *k8s.Client, namespace, labelSelector string) {
	deployments, err := client.K8sClientset.AppsV1().Deployments(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		fmt.Printf(" [⨯] Failed to list deployments: %v\n", errors.ReasonForError(err))
	}

	for _, deployment := range deployments.Items {
		err := client.K8sClientset.AppsV1().Deployments(namespace).Delete(context.Background(), deployment.Name, metav1.DeleteOptions{})
		if err != nil {
			fmt.Printf(" [⨯] Failed to delete deployment %s: %v\n", deployment.Name, errors.ReasonForError(err))
		} else {
			fmt.Printf(" [✓] Deleted deployment: %s\n", deployment.Name)
		}
	}
}
