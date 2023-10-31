package common

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// SetupGitHubClient sets up a GitHub client with the provided PAT, it uses oAuth2 to authenticate
func SetupGitHubClient(pat string, ctx context.Context) (*github.Client, error) {
	if pat == "" {
		return nil, os.ErrInvalid
	}

	if err := checkGithubHealth(); err != nil {
		return nil, err
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: pat},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	return client, nil
}

// GetLatestRelease returns the latest release from the GitHub API
func GetLatestRelease(client *github.Client, ctx context.Context) (*github.RepositoryRelease, error) {
	latestRelease, _, err := client.Repositories.GetLatestRelease(ctx, AccuknoxGithub, AccuknoxCLIRepo)
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
