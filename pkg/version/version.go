package version

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/onboard"
	"github.com/accuknox/accuknox-cli-v2/pkg/privilege"
	"github.com/kubearmor/kubearmor-client/k8s"
	"github.com/moby/moby/client"
)

var (
	GitSummary = ""
	BuildDate  = ""
)

const (
	releaseVersionPage = "https://knoxctl.accuknox.com/version/latest_version.txt"
)

type Option struct {
	Long          bool
	VMMode        string
	GitPATPath    string
	LatestRelease bool
}

// PrintVersion displays the current version and checks for updates
func PrintVersion(c *k8s.Client, o Option) error {
	if o.VMMode != "" && o.VMMode != "auto" && o.VMMode != "docker" && o.VMMode != "systemd" {
		return fmt.Errorf("--vm-mode must be auto, docker, or systemd")
	}
	if o.Long {
		fmt.Printf("knoxctl's version: %s (Built on %s)\n", GitSummary, BuildDate)
	} else {
		fmt.Printf("knoxctl's version: %s\n", GitSummary)
	}
	fmt.Println()

	if o.LatestRelease {
		releaseVer, err := fetchReleaseVersion()
		if err != nil {
			return fmt.Errorf("error fetching latest version: %v", err)
		}
		fmt.Printf("knoxctl release version: [%v]\n", releaseVer)
	}

	if o.VMMode != "systemd" {
		found, err := onboard.PrintDockerAgentVersions(context.Background(), os.Stdout, o.Long)
		if o.VMMode == "docker" {
			if err != nil {
				return err
			}
			if !found {
				fmt.Println("No Docker agents found.")
			}
			return nil
		}
		if err != nil && (errors.Is(err, os.ErrPermission) || !client.IsErrConnectionFailed(err)) {
			if privilege.IsPermissionError(err) {
				return err
			}
			fmt.Fprintf(os.Stderr, "Warning: Docker agent versions unavailable: %v\n", err)
		}
	}
	return onboard.DetermineAgentVersions(o.Long)

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
