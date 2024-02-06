package install

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/accuknox/dev2/common/deploy"
	"github.com/kubearmor/kubearmor-client/k8s"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Options struct {
	Tag      string
	ListTags bool
	Debug    bool
}

// DiscoveryEngine installs all the necessary k8s objects and resources
// that will get Discovery Engine deployed, up and running.
func DiscoveryEngine(client *k8s.Client, o Options) error {
	if o.ListTags {
		tags, err := deploy.GetReleaseTags()
		if err != nil {
			return err
		}

		for _, tag := range tags {
			fmt.Printf("Tag: %s, Release date: %s\n", tag.Tag, tag.Date)
		}

		return nil
	}

	err := processTags(&o)
	if err != nil {
		return err
	}

	err = setupEverything(client, o)
	if err != nil {
		fmt.Printf("Failed to setup one of kubernetes objects\n")
		if !o.Debug {
			fmt.Printf("Rolling back deployemnt and cleaning up resources...\n")
			rollbackDeployment(client)
		}
		return err
	}

	countdownDuration := 7 * time.Minute

	doneChan := make(chan error)
	allGoodChan := make(chan bool)
	go startCountdown(countdownDuration)

	go func() {
		good, err := allGood(client, "accuknox-agents", 7*time.Minute, 5, 10*time.Second)
		doneChan <- err
		allGoodChan <- good // is this channel needed?
	}()

	select {
	case err := <-doneChan:
		if err != nil {
			fmt.Printf("\nDeployment failed\n")
			if !o.Debug {
				fmt.Printf("Rolling back deployment and cleaning up resources...\n")
				rollbackDeployment(client)
			}
			return err
		}
		fmt.Println("\nDeployment succuessful! Discovery Engine is up and running.")
		return nil
	case good := <-allGoodChan:
		if good {
			fmt.Println("\nDeployment successful! Discovery Engine is up and running.")
		}
		return nil
	case <-time.After(countdownDuration):
		fmt.Printf("Deployment timeout reached\n")
		if !o.Debug {
			fmt.Println("Health check timeout reached.")
		} else {
			fmt.Println("Health check timeout reached.")
		}
		return nil
	}
}

func setupEverything(client *k8s.Client, o Options) error {
	// 1. Setup Namespace
	Namespace, err := setupNamespace(client, o.Tag)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf(" [⨯] Failed to set up Namespace: %v\n", errors.ReasonForError(err))
	}

	// 2. Setup ConfigMap
	_, err = setupConfigmaps(client, o.Tag, Namespace)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf(" [⨯] Failed to set up configmap: %v", errors.ReasonForError(err))
	}

	// 3. Setup CRD
	_, err = setupCRD(client, o.Tag)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf(" [⨯] Failed to set up CRD: %v", errors.ReasonForError(err))
	}

	// 4. Setup Service Account
	_, err = setupServiceAccount(client, o.Tag, Namespace)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf(" [⨯] Failed to set up service account: %v", errors.ReasonForError(err))
	}

	// 5. Setup Cluster Role
	_, err = setupClusterRole(client, o.Tag)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf(" [⨯] Failed to set up cluster role: %v", errors.ReasonForError(err))
	}

	// 6. Setup Cluster Role Binding
	_, err = setupClusterRoleBinding(client, o.Tag)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf(" [⨯] Failed to set up cluster role binding: %v", errors.ReasonForError(err))
	}

	// 7. Setup Deployment
	_, err = setupDeployment(client, o.Tag, Namespace)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf(" [⨯] Failed to set up deployment: %v", errors.ReasonForError(err))
	}

	// 8. Setup Service
	_, err = setupService(client, o.Tag, Namespace)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf(" [⨯] Failed to set up service: %v", errors.ReasonForError(err))
	}

	fmt.Printf("Discovery Engine objects created successfully!\n")
	return nil
}

func setupNamespace(client *k8s.Client, tag string) (string, error) {
	manifest, err := deploy.Namespace(tag)
	if err != nil {
		return "", err
	}

	obj, err := decodeManifest([]byte(manifest), corev1.AddToScheme)
	if err != nil {
		return "", err
	}

	Namespace, ok := obj.(*corev1.Namespace)
	if !ok {
		return "", fmt.Errorf("manifest does not represents a Namespace")
	}

	err = createCoreV1Resources(client, obj, "")
	if err != nil {
		return "", err
	}

	return Namespace.Name, nil
}

func setupConfigmaps(client *k8s.Client, tag string, Namespace string) ([]string, error) {
	configMapFuncs := []func(string) (string, error){
		deploy.ConfigMapSumE,      // Summary Engine configmap
		deploy.ConfigMapOffloader, // Offloader configmap
		deploy.ConfigMapDiscover,  // Discover module configmap
		deploy.ConfigMapHardening, // Hardening module configmap
	}

	var cmNames []string
	for _, getConfigMap := range configMapFuncs {
		manifest, err := getConfigMap(tag)
		if err != nil {
			return nil, err
		}

		obj, err := decodeManifest([]byte(manifest), corev1.AddToScheme)
		if err != nil {
			return nil, err
		}

		configmap, ok := obj.(*corev1.ConfigMap)
		if !ok {
			return nil, fmt.Errorf("manifest does not represent a configmap, please check again")
		}

		err = createCoreV1Resources(client, obj, Namespace)
		if err != nil {
			return nil, err
		}

		cmNames = append(cmNames, configmap.Name)
	}

	return cmNames, nil
}

func setupCRD(client *k8s.Client, tag string) (string, error) {
	manifest, err := deploy.CRD(tag)
	if err != nil {
		return "", err
	}

	obj, err := decodeManifest([]byte(manifest), apiextensionsv1.AddToScheme)
	if err != nil {
		return "", err
	}

	crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition)
	if !ok {
		return "", fmt.Errorf("manifest does not represent a crd, please check again")
	}

	err = createApiExtnV1Resources(client, obj)
	if err != nil {
		return "", err
	}

	return crd.Name, nil
}

func setupServiceAccount(client *k8s.Client, tag string, Namespace string) (string, error) {
	manifest, err := deploy.ServiceAccount(tag)
	if err != nil {
		return "", err
	}

	obj, err := decodeManifest([]byte(manifest), corev1.AddToScheme)
	if err != nil {
		return "", err
	}

	svcaccount, ok := obj.(*corev1.ServiceAccount)
	if !ok {
		return "", fmt.Errorf("manifest does not represent a service account, please check again")
	}

	err = createCoreV1Resources(client, obj, Namespace)
	if err != nil {
		return "", err
	}

	return svcaccount.Name, nil
}

func setupDeployment(client *k8s.Client, tag string, Namespace string) (string, error) {
	manifest, err := deploy.Deployments(tag)
	if err != nil {
		return "", err
	}

	obj, err := decodeManifest([]byte(manifest), appsv1.AddToScheme)
	if err != nil {
		return "", err
	}

	deployment, ok := obj.(*appsv1.Deployment)
	if !ok {
		return "", fmt.Errorf("manifest does not represents a deployment, please check again")
	}

	err = createAppsV1Resources(client, obj, Namespace)
	if err != nil {
		return "", err
	}

	return deployment.Name, nil
}

func setupClusterRoleBinding(client *k8s.Client, tag string) (string, error) {
	manifest, err := deploy.ClusterRoleBinding(tag)
	if err != nil {
		return "", err
	}

	obj, err := decodeManifest([]byte(manifest), rbacv1.AddToScheme)
	if err != nil {
		return "", err
	}

	clusterRoleBinding, ok := obj.(*rbacv1.ClusterRoleBinding)
	if !ok {
		return "", fmt.Errorf("manifest does not represents a cluster role binding, please check again")
	}

	err = createRbacV1Resources(client, obj)
	if err != nil {
		return "", err
	}

	return clusterRoleBinding.Name, nil
}

func setupClusterRole(client *k8s.Client, tag string) (string, error) {
	manifest, err := deploy.ClusterRole(tag)
	if err != nil {
		return "", err
	}

	obj, err := decodeManifest([]byte(manifest), rbacv1.AddToScheme)
	if err != nil {
		return "", err
	}

	clusterRole, ok := obj.(*rbacv1.ClusterRole)
	if !ok {
		return "", fmt.Errorf("manifest does not represent a cluster role, please check again")
	}

	err = createRbacV1Resources(client, obj)
	if err != nil {
		return "", err
	}

	return clusterRole.Name, nil
}

func setupService(client *k8s.Client, tag string, Namespace string) (string, error) {
	manifest, err := deploy.Service(tag)
	if err != nil {
		return "", err
	}

	obj, err := decodeManifest([]byte(manifest), corev1.AddToScheme)
	if err != nil {
		return "", err
	}

	service, ok := obj.(*corev1.Service)
	if !ok {
		return "", fmt.Errorf("manifest does not represent a service, please check again")
	}

	err = createCoreV1Resources(client, obj, Namespace)
	if err != nil {
		return "", err
	}

	return service.Name, nil
}

func allGood(client *k8s.Client, Namespace string, timeout time.Duration, restartThreshold int32, checkInterval time.Duration) (bool, error) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	timeoutChan := time.After(timeout)

	for {
		select {
		case <-ticker.C:
			pods, err := client.K8sClientset.CoreV1().Pods(Namespace).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				return false, err
			}

			allRunning := true
			for _, pod := range pods.Items {
				if pod.Status.Phase != corev1.PodRunning {
					allRunning = false
					break
				}

				for _, containerStatus := range pod.Status.ContainerStatuses {
					if !containerStatus.Ready || containerStatus.RestartCount > restartThreshold {
						allRunning = false
						break
					}
				}

				if !allRunning {
					break
				}
			}

			if allRunning {
				return true, nil
			}
		case <-timeoutChan:
			return false, fmt.Errorf("timeout reached while waiting for pods to be in a running state")
		}
	}
}

func startCountdown(duration time.Duration) {
	fmt.Printf("Please wait for few minutes, we are doing a quick health check... [ctrl+c to skip]\n")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	endTime := time.Now().Add(duration)
	for now := range ticker.C {
		remaining := endTime.Sub(now)
		if remaining <= 0 {
			fmt.Printf("\rTime left: 00\n")
			break
		}
		fmt.Printf("\rTime left: %s", remaining.Truncate(time.Second))
	}
}

func processTags(o *Options) error {
	tags, err := deploy.GetReleaseTags()
	if err != nil {
		return err
	}

	var semtags []*semver.Version
	for _, tag := range tags {
		v, err := semver.NewVersion(tag.Tag)
		if err != nil {
			fmt.Printf("skipping invalid tag: %s\n", tag.Tag)
			continue
		}
		semtags = append(semtags, v)
	}

	sort.Sort(semver.Collection(semtags))

	if len(semtags) == 0 {
		return fmt.Errorf("no valid release tags found for Discovery Engine")
	}

	if o.Tag == "" {
		latestTag := semtags[len(semtags)-1]
		o.Tag = "v" + latestTag.String()
		fmt.Printf("Installing Discovery Engine with latest release tag: %v\n", o.Tag)
	} else {
		tagExists := false
		for _, v := range semtags {
			if "v"+v.String() == o.Tag {
				tagExists = true
				break
			}
		}

		if !tagExists {
			return fmt.Errorf("specified release tag %s does not exists or is invalid.", o.Tag)
		}

		fmt.Printf("Installing Discover Engine with release tag: %v", o.Tag)
	}
	return nil
}
