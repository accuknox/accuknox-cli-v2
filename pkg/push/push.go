package push

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/accuknox/accuknox-cli-v2/pkg/common"
	"github.com/google/go-github/github"
)

type Options struct {
	GitPATPath string
}

// WARNING: This has to be changed once Accuknox's webpage is up with a proper domain name.
// we will change the repo to Accuknox and repoName to knoxctl-website
const (
	repoOwner = "swarit-pandey"
	repoName  = "knoxctl.github.io"
)

func downloadReleaseAssets(client *github.Client, ctx context.Context, owner string, repo string, releaseTag string, tempDir string) error {
	release, _, err := client.Repositories.GetReleaseByTag(ctx, owner, repo, releaseTag)
	if err != nil {
		return fmt.Errorf("failed to get release by tag: %w", err)
	}

	for _, asset := range release.Assets {
		filePath := filepath.Join(tempDir, *asset.Name)
		err := downloadAsset(client, ctx, owner, repo, *asset.ID, filePath)
		if err != nil {
			return fmt.Errorf("failed to download asset: %w", err)
		}
	}

	return nil
}

func downloadAsset(client *github.Client, ctx context.Context, owner string, repo string, assetID int64, filePath string) error {
	reader, redURL, err := client.Repositories.DownloadReleaseAsset(ctx, owner, repo, assetID)
	if err != nil {
		return fmt.Errorf("error downloading asset: %v", err)
	}

	if redURL != "" {
		_, err := url.ParseRequestURI(redURL)
		if err != nil {
			return fmt.Errorf("error parsing redirect URL: %v", err)
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

	cleanedDestPath := filepath.Clean(filePath)
	out, err := os.Create(cleanedDestPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, reader)
	return err
}

func uploadBinaries(client *github.Client, ctx context.Context, tempDir string) error {
	files, err := os.ReadDir(tempDir)
	if err != nil {
		return fmt.Errorf("failed to read tempDir: %w", err)
	}

	var latestVersion string
	for _, file := range files {
		parts := strings.Split(file.Name(), "_") // format: accuknoxcli_<version>_<file-related-info>
		if len(parts) > 1 {
			latestVersion = strings.TrimPrefix(parts[1], ".tar.gz")
			break
		}
	}

	if latestVersion != "" {
		fmt.Printf("Release version found: [%s]\n", latestVersion)
		err = updateVersionFile(client, ctx, latestVersion)
		if err != nil {
			return err
		}
	}

	err = filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking the path: %w", err)
		}

		if !info.IsDir() && info.Size() > 0 {
			cleanedPath := filepath.Clean(path)
			file, openErr := os.Open(cleanedPath)
			if openErr != nil {
				return fmt.Errorf("failed to open file: %w", openErr)
			}
			defer file.Close()

			content, readErr := io.ReadAll(file)
			if readErr != nil {
				return fmt.Errorf("failed to read file: %w", readErr)
			}

			repoPath := "static/binaries/" + info.Name() // this should go in static/binaries to make sure the binaries are downloadable

			fileContent, _, _, err := client.Repositories.GetContents(ctx, repoOwner, repoName, repoPath, &github.RepositoryContentGetOptions{})
			var sha string
			if err == nil && fileContent != nil {
				sha = *fileContent.SHA
			}

			// if branch is write protected we might have to change this
			opts := &github.RepositoryContentFileOptions{
				Message: github.String("chore(bin): upload/update binary file"),
				Content: content,
				Branch:  github.String("main"),
				SHA:     &sha,
			}

			_, _, err = client.Repositories.CreateFile(ctx, repoOwner, repoName, repoPath, opts)
			if err != nil {
				fmt.Printf("failed to upload file: %v", err)
			} else {
				fmt.Printf("Uploaded %s to repository path %s\n", info.Name(), repoPath)
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("error in uploading file: %w", err)
	}

	t := time.Now()
	formattedTime := t.Format("2006-01-02 15:04:05")
	fmt.Printf("Artifacts uploaded [time: %v]\n", formattedTime)
	return nil
}

func handleAssetsTransfer(gitPAT string) error {
	ctx := context.Background()
	client, err := common.SetupGitHubClient(gitPAT, ctx)
	if err != nil {
		return fmt.Errorf("failed to setup GitHub client: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "knoxctl_assets")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	releaseTag, err := common.GetLatestVersion(client, ctx)
	if err != nil {
		return fmt.Errorf("failed to get latest version: %w", err)
	}

	err = downloadReleaseAssets(client, ctx, common.AccuknoxGithub, common.AccuknoxCLIRepo, releaseTag, tempDir)
	if err != nil {
		return fmt.Errorf("failed to download release assets: %w", err)
	}

	_, err = os.ReadDir(tempDir)
	if err != nil {
		return fmt.Errorf("failed to list tempDir contents: %w", err)
	}

	err = uploadBinaries(client, ctx, tempDir)
	if err != nil {
		return fmt.Errorf("failed to upload binaries: %w", err)
	}
	return nil
}

// Push will push the GitHub artifacts to knoxctl website, this
// subcommand requires a valid GitHub PAT and is only accessible
// to an internal Accuknox member.
func Push(options *Options) error {
	if options.GitPATPath == "" {
		return errors.New("empty Git PAT path")
	}

	gitPAT, err := readGitKey(options.GitPATPath)
	if err != nil {
		return err
	}

	return handleAssetsTransfer(gitPAT)
}

func readGitKey(path string) (string, error) {
	cleanedPath := filepath.Clean(path)

	keyBytes, err := os.ReadFile(cleanedPath)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(keyBytes)), nil
}

func updateVersionFile(client *github.Client, ctx context.Context, version string) error {
	path := "static/version/latest-version.txt"

	fileContent, _, _, err := client.Repositories.GetContents(ctx, repoOwner, repoName, path, &github.RepositoryContentGetOptions{})
	if err != nil {
		return fmt.Errorf("failed to fetch current file SHA: %w", err)
	}
	sha := *fileContent.SHA

	content := []byte(version)
	opts := &github.RepositoryContentFileOptions{
		Message: github.String("update(version): update version"),
		Content: content,
		SHA:     &sha,
		Branch:  github.String("main"),
	}

	_, _, err = client.Repositories.UpdateFile(ctx, repoOwner, repoName, path, opts)
	if err != nil {
		return fmt.Errorf("failed to update the file: %w", err)
	}

	fmt.Printf("Updated version in knoxctl webpage repo [%s]\n", version)
	return nil
}
