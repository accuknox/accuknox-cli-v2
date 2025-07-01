package common

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/go-github/github"
)

// SetupGitHubClient sets up a GitHub client with the provided PAT, it uses oAuth2 to authenticate
func SetupGitHubClient(ctx context.Context) (*github.Client, error) {

	client := github.NewClient(nil)

	return client, nil
}

// GetLatestRelease returns the latest release from the GitHub API
func GetLatestRelease(client *github.Client, ctx context.Context) (*github.RepositoryRelease, error) {
	latestRelease, _, err := client.Repositories.GetLatestRelease(ctx, AccuknoxGithub, AccuknoxKnoxctlwebsite)
	if err != nil {
		return nil, err
	}

	return latestRelease, nil
}

// GetLatestVersion returns the latest version from the GitHub API
func GetLatestVersion(client *github.Client, ctx context.Context) (string, error) {
	latestRelease, err := GetLatestRelease(client, ctx)
	if err != nil {
		return "", err
	}

	return *latestRelease.TagName, nil
}

func checkGithubHealth() error {
	resp, err := http.Get("https://api.github.com")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("github api is not healthy, please check github status at https://www.githubstatus.com/")
	}

	return nil
}
