package install

import (
	"context"
	"fmt"

	"github.com/kubearmor/kubearmor-client/k8s"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

func decodeManifest(manifest []byte, addToScheme func(*runtime.Scheme) error) (runtime.Object, error) {
	sch := runtime.NewScheme()
	if err := addToScheme(sch); err != nil {
		return nil, err
	}

	dec := serializer.NewCodecFactory(sch).UniversalDeserializer()
	obj, _, err := dec.Decode(manifest, nil, nil)
	return obj, err
}

func createCoreV1Resources(client *k8s.Client, obj runtime.Object, namespace string) error {
	switch o := obj.(type) {
	case *corev1.Namespace:
		_, err := client.K8sClientset.CoreV1().Namespaces().Create(context.Background(), o, metav1.CreateOptions{})
		if err != nil {
			if errors.IsAlreadyExists(err) {
				fmt.Printf(" [-] Namespace already exists: %s\n", o.Name)
				return nil
			}
			return err
		}
		fmt.Printf(" [✓] Namespace created %s\n", o.Name)
		return nil
	case *corev1.ConfigMap:
		_, err := client.K8sClientset.CoreV1().ConfigMaps(namespace).Create(context.Background(), o, metav1.CreateOptions{})
		if err != nil {
			if errors.IsAlreadyExists(err) {
				fmt.Printf(" [-] ConfigMap already exists: %s\n", o.Name)
				return nil
			}
			return err
		}
		fmt.Printf(" [✓] Config Map created %s\n", o.Name)
		return nil
	case *corev1.ServiceAccount:
		_, err := client.K8sClientset.CoreV1().ServiceAccounts(namespace).Create(context.Background(), o, metav1.CreateOptions{})
		if err != nil {
			if errors.IsAlreadyExists(err) {
				fmt.Printf(" [-] ServiceAccount already exists: %s\n", o.Name)
				return nil
			}
			return err
		}
		fmt.Printf(" [✓] Service Account created %s\n", o.Name)
		return nil
	case *corev1.Service:
		_, err := client.K8sClientset.CoreV1().Services(namespace).Create(context.Background(), o, metav1.CreateOptions{})
		if err != nil {
			if errors.IsAlreadyExists(err) {
				fmt.Printf(" [-] Service already exists: %s\n", o.Name)
				return nil
			}
			return err
		}
		fmt.Printf(" [✓] Service created %s\n", o.Name)
		return nil
	default:
		return fmt.Errorf("manifest is not a core/v1 k8s object")
	}
}

func createRbacV1Resources(client *k8s.Client, obj runtime.Object) error {
	switch o := obj.(type) {
	case *rbacv1.ClusterRole:
		_, err := client.K8sClientset.RbacV1().ClusterRoles().Create(context.Background(), o, metav1.CreateOptions{})
		if err != nil {
			if errors.IsAlreadyExists(err) {
				fmt.Printf(" [-] Cluster Role already exists: %s\n", o.Name)
				return nil
			}
			return err
		}
		fmt.Printf(" [✓] Cluster Role created %s\n", o.Name)
		return nil
	case *rbacv1.ClusterRoleBinding:
		_, err := client.K8sClientset.RbacV1().ClusterRoleBindings().Create(context.Background(), o, metav1.CreateOptions{})
		if err != nil {
			if errors.IsAlreadyExists(err) {
				fmt.Printf(" [-] Cluster Role Binding already exists: %s\n", o.Name)
				return err
			}
			return err
		}
		fmt.Printf(" [✓] Cluster Role Binding created %s\n", o.Name)
		return nil
	default:
		return fmt.Errorf("manifest is not a rbac/v1 k8s object")
	}
}

func createApiExtnV1Resources(client *k8s.Client, obj runtime.Object) error {
	switch o := obj.(type) {
	case *apiextensionsv1.CustomResourceDefinition:
		_, err := client.APIextClientset.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), o, metav1.CreateOptions{})
		if err != nil {
			if errors.IsAlreadyExists(err) {
				fmt.Printf(" [-] CRD already exists: %s\n", o.Name)
				return nil
			}
			return err
		}
		fmt.Printf(" [✓] CRD created %s\n", o.Name)
		return nil
	default:
		return fmt.Errorf("manifest is not an apiextension/v1 k8s object")
	}
}

func createAppsV1Resources(client *k8s.Client, obj runtime.Object, namespace string) error {
	switch o := obj.(type) {
	case *appsv1.Deployment:
		_, err := client.K8sClientset.AppsV1().Deployments(namespace).Create(context.Background(), o, metav1.CreateOptions{})
		if err != nil {
			if errors.IsAlreadyExists(err) {
				fmt.Printf(" [-] Deployment already exists: %s\n", o.Name)
				return nil
			}
			return err
		}
		fmt.Printf(" [✓] Deployment created %s\n", o.Name)
		return nil
	default:
		return fmt.Errorf("manifest is not an apps/v1 k8s object")
	}
}
