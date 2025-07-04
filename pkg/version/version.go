package version

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/kubearmor/kubearmor-client/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	GitSummary = ""
	BuildDate  = ""
)

const (
	releaseVersionPage = "https://knoxctl.accuknox.com/version/latest_version.txt"
)

type Option struct {
	GitPATPath    string
	LatestRelease bool
}

// PrintVersion displays the current version and checks for updates
func PrintVersion(c *k8s.Client, o Option) error {
	fmt.Printf("knoxctl's version: %s (Built on %s)\n", GitSummary, BuildDate)

	if o.LatestRelease {
		releaseVer, err := fetchReleaseVersion()
		if err != nil {
			return fmt.Errorf("error fetching latest version: %v", err)
		}
		fmt.Printf("knoxctl release version: [%v]\n", releaseVer)
	}

	systemdVersions, err := onboard.DetermineAgentVersions()
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	for agent, version := range systemdVersions {
		fmt.Printf("%s: %s\n", agent, version)
	}

	/*
		// knoxctl based kubernetes installation is not done right now
		kubearmorVersion, err := getKubeArmorVersion(c)
		if err != nil {
			return nil
		}
		if kubearmorVersion == "" {
			fmt.Printf("kubearmor not running\n")
			return nil
		}

		fmt.Printf("kubearmor image (running) version: [%s]\n", kubearmorVersion)
	*/
	return nil
}

func getKubeArmorVersion(c *k8s.Client) (string, error) {
	deployments, err := c.K8sClientset.AppsV1().Deployments("").List(context.Background(), metav1.ListOptions{LabelSelector: "kubearmor-app=kubearmor"})
	if err != nil {
		return "", err
	}

	if len(deployments.Items) > 0 {
		image := deployments.Items[0].Spec.Template.Spec.Containers[0].Image
		return image, nil
	}

	return "", nil
}

func fetchReleaseVersion() (string, error) {
	resp, err := http.Get(releaseVersionPage)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(body)), nil
}
