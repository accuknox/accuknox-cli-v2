package update

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/accuknox/accuknox-cli-v2/pkg/version"
	"github.com/google/go-github/github"
	"github.com/kubearmor/kubearmor-client/k8s"
)

const maxSize = 128 * 10e6 // 128 MB

type Option struct {
	GitPATPath string
	DoUpdate   bool
}

func decompressAndExtractTarGz(src, destDir string) error {
	srcPath := filepath.Clean(src)
	gzFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer gzFile.Close()

	stat, err := gzFile.Stat()
	if err != nil {
		return err
	}
	if stat.Size() > maxSize {
		return fmt.Errorf("file size too large")
	}

	gzReader, err := gzip.NewReader(gzFile)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if header.Typeflag == tar.TypeReg {
			if strings.Contains(header.Name, "..") {
				return fmt.Errorf("invalid file path")
			}

			cleanedDestPath, err := sanitizeArchivePath(destDir, header.Name)
			if err != nil {
				return err
			}
			outFile, err := os.Create(filepath.Clean(cleanedDestPath))
			if err != nil {
				return err
			}

			limitedReader := &io.LimitedReader{R: tarReader, N: maxSize}
			if _, err := io.Copy(outFile, limitedReader); err != nil {
				if limitedReader.N <= 0 {
					return fmt.Errorf("decompressed file size exceeds limit")
				}
				return err
			}

			err = outFile.Close()
			if err != nil {
				return err
			}

			err = os.Chmod(cleanedDestPath, 0600)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func downloadFile(client *github.Client, ctx context.Context, assetID int64, destPath string) error {
	reader, redURL, err := client.Repositories.DownloadReleaseAsset(ctx, "accuknox", common.AccuknoxKnoxctlwebsite, assetID)
	if err != nil {
		fmt.Printf("error downloading asset: %v", err)
		return err
	}

	if redURL != "" {
		_, err := url.ParseRequestURI(redURL)
		if err != nil {
			return fmt.Errorf("error downloading asset: %v", err)
		}

		httpClient := &http.Client{
			Timeout: time.Second * 30,
		}

		resp, err := httpClient.Get(redURL)
		if err != nil {
			return fmt.Errorf("error downloading asset: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("error downloading asset: %v", resp.Status)
		}

		reader = resp.Body
	} else if reader == nil {
		return fmt.Errorf("error downloading asset: reader is nil")
	}
	cleanedDestPath := filepath.Clean(destPath)
	out, err := os.Create(cleanedDestPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, reader)
	return err
}

func doUpdate(doUpdate bool) error {
	ctx := context.Background()
	client, err := common.SetupGitHubClient(ctx)
	if err != nil {
		return fmt.Errorf("error setting up GitHub client: %v", err)
	}

	release, err := common.GetLatestRelease(client, ctx)
	if err != nil {
		return fmt.Errorf("error getting latest release: %v", err)
	}

	shouldUpdate, latestVersion, currentVersion, err := version.ShouldUpdate(version.GitSummary)
	if err != nil {
		return fmt.Errorf("error checking if update is needed: %v", err)
	}

	if !shouldUpdate {
		fmt.Println("You are already using the latest version. ", latestVersion)
		return nil
	}

	if !doUpdate {
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

	tempDir, err := os.MkdirTemp("", "knoxctl-update-")
	if err != nil {
		return fmt.Errorf("failed to create a temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tarGzFilePath := filepath.Join(tempDir, *asset.Name)
	if err := downloadFile(client, ctx, *asset.ID, tarGzFilePath); err != nil {
		return err
	}
	if err := decompressAndExtractTarGz(tarGzFilePath, tempDir); err != nil {
		return err
	}

	binaryFilePath := filepath.Join(tempDir, "knoxctl") // need to change this after a new release
	cleanedBinaryFilePath := filepath.Clean(binaryFilePath)

	scriptPath := "./scripts/move.sh"
	cleanedScriptPath := filepath.Clean(scriptPath)

	cmd := exec.Command(cleanedScriptPath, cleanedBinaryFilePath) // #nosec (false positive, ref: https://stackoverflow.com/a/58861277)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run update script: %v", err)
	}

	latestVer, _ := common.GetLatestVersion(client, ctx)

	fmt.Println("Update successful. [" + latestVer + "]")
	return nil
}

func SelfUpdate(c *k8s.Client, options *Option) error {

	return doUpdate(options.DoUpdate)
}

func readGitKey(path string) (string, error) {
	cleanedPath := filepath.Clean(path)

	keyBytes, err := os.ReadFile(cleanedPath)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(keyBytes)), nil
}

func sanitizeArchivePath(d, t string) (v string, err error) {
	v = filepath.Join(d, t)
	if strings.HasPrefix(v, filepath.Clean(d)) {
		return v, nil
	}

	return "", fmt.Errorf("%s: %s", "content filepath is tainted", t)
}
