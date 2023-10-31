package version

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/fatih/color"
	"github.com/kubearmor/kubearmor-client/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	GitSummary = ""
	BuildDate  = ""
)

type Option struct {
	GitPATPath string
}

// PrintVersion displays the current version and checks for updates
func PrintVersion(c *k8s.Client, pat string) error {
	if pat == "" {
		fmt.Println("please provide an absolute path to your GitHub Personal Access Token (PAT)")
		return nil
	}

	gitKey, err := readGitKey(pat)
	if err != nil {
		return fmt.Errorf("error reading git PAT: %v", err)
	}

	ctx := context.Background()
	client, err := common.SetupGitHubClient(gitKey, ctx)
	if err != nil {
		return fmt.Errorf("error setting up GitHub client: %v", err)
	}

	latestVer, err := common.GetLatestVersion(client, ctx)
	if err != nil {
		return fmt.Errorf("error checking latest version: %v", err)
	}

	if latestVer != GitSummary {
		color.HiMagenta("Update available: version " + latestVer)
	} else {
		fmt.Println("You are using the latest version of accuknoxcli.")
	}

	kubearmorVersion, err := getKubeArmorVersion(c)
	if err != nil {
		return nil
	}
	if kubearmorVersion == "" {
		fmt.Printf("kubearmor not running\n")
		return nil
	}
	fmt.Printf("kubearmor image (running) version %s\n", kubearmorVersion)
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

func readGitKey(path string) (string, error) {
	cleanedPath := filepath.Clean(path)

	keyBytes, err := os.ReadFile(cleanedPath)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(keyBytes)), nil
}

func ShouldUpdate(currentVersion, pat string) (bool, error) {
	if pat == "" {
		return false, fmt.Errorf("GitHub Personal Access Token (PAT) is required")
	}

	client, err := common.SetupGitHubClient(pat, context.Background())
	if err != nil {
		return false, fmt.Errorf("error setting up GitHub client: %v", err)
	}

	latestVersion, err := common.GetLatestVersion(client, context.Background())
	if err != nil {
		return false, fmt.Errorf("error checking latest version: %v", err)
	}

	return latestVersion != currentVersion, nil
}
