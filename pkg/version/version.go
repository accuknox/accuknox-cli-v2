package version

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/kubearmor/kubearmor-client/k8s"
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
	fmt.Println()

	if o.LatestRelease {
		releaseVer, err := fetchReleaseVersion()
		if err != nil {
			return fmt.Errorf("error fetching latest version: %v", err)
		}
		fmt.Printf("knoxctl release version: [%v]\n", releaseVer)
	}

	return onboard.DetermineAgentVersions()

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
