package update

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/version"
	selfupdate "github.com/creativeprojects/go-selfupdate"
	"github.com/fatih/color"
	"github.com/google/go-github/github"
)

const maxSize = 128 * 10e6 // 128 MB

// SelfUpdate() checks if an update is available for the knoxctl CLI tool.
func SelfUpdate() error {

	ctx := context.Background()
	client, err := common.SetupGitHubClient(ctx)
	if err != nil {
		return fmt.Errorf("error setting up GitHub client: %v", err)
	}

	release, err := common.GetLatestRelease(client, ctx)
	if err != nil {
		return fmt.Errorf("error getting latest release: %v", err)
	}

	currentVersion := strings.TrimLeft(version.GitSummary, "v")
	latestVersion := strings.TrimLeft(*release.TagName, "v")
	if latestVersion == currentVersion {
		fmt.Println("Knoxctl is already running the latest version. ", *release.TagName)
		return nil
	}

	fmt.Printf("An update is available for knoxctl: latest version %s, current version %s. Do you want to update? (y/n): ", latestVersion, currentVersion)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("error reading response: %v", err)
	}
	if strings.ToLower(strings.TrimSpace(response)) != "y" {
		fmt.Println("Update cancelled.")
		return nil
	}

	opSys := runtime.GOOS  // "linux" or "darwin"
	arch := runtime.GOARCH // "amd64" or "arm64"
	assetIdentifier := fmt.Sprintf("%s_%s", opSys, arch)
	fmt.Println("System identified: ", assetIdentifier)

	var asset *github.ReleaseAsset
	for _, a := range release.Assets {
		if strings.Contains(*a.Name, assetIdentifier) && strings.HasSuffix(*a.Name, ".tar.gz") {
			assetCopy := a
			asset = &assetCopy
			break
		}
	}

	if asset == nil {
		return fmt.Errorf("no suitable asset found for %s/%s in the release", opSys, arch)
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Println("Could not locate executable path")
		return errors.New("could not locate exec path")
	}
	if err := selfupdate.UpdateTo(ctx, *asset.BrowserDownloadURL, *asset.Name, exe); err != nil {
		if strings.Contains(err.Error(), "permission denied") {
			color.Red("use [sudo knoxctl selfupdate]")
		}
		return err
	}
	fmt.Println("Update successful. [" + latestVersion + "]")
	return nil
}
