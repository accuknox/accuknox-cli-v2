// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Authors of KubeArmor

package ui

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// gitReq holds the Git-source fields accepted by the SBOM handler.
type gitReq struct {
	Provider string `json:"provider"` // "github" or "gitlab"
	Token    string `json:"token"`    // optional override; falls back to saved config
	Ref      string `json:"ref"`      // optional branch/tag/commit
}

// gitTokenFor returns the token to use for a provider: the per-request override
// when present, otherwise the value saved in the app config.
func gitTokenFor(g gitReq) string {
	if t := strings.TrimSpace(g.Token); t != "" {
		return t
	}
	cfg := loadAppConfig()
	if strings.EqualFold(g.Provider, "gitlab") {
		return cfg.Git.GitLabToken
	}
	return cfg.Git.GitHubToken
}

// authedGitURL injects credentials into an https Git URL. The username convention
// differs per provider so the same flow works for github.com, gitlab.com, and
// self-hosted instances (the host is taken from the URL itself):
//
//	github  -> https://x-access-token:<token>@host/owner/repo.git
//	gitlab  -> https://oauth2:<token>@host/owner/repo.git
//
// With no token the URL is returned unchanged (public repositories).
func authedGitURL(raw, provider, token string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if u.Host == "" {
		return "", fmt.Errorf("repository URL must be absolute (https://host/owner/repo)")
	}
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	if token != "" {
		user := "x-access-token"
		if strings.EqualFold(provider, "gitlab") {
			user = "oauth2"
		}
		u.User = url.UserPassword(user, token)
	}
	return u.String(), nil
}

// cloneGitSource shallow-clones repoURL into baseDir and returns the path of the
// working copy. Credentials are resolved from the request or saved config; the
// token is never written to the log stream.
func cloneGitSource(ctx context.Context, repoURL string, g gitReq, baseDir string, logw io.Writer) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("git is not installed or not on PATH; install Git to use the Git repository source")
	}

	token := gitTokenFor(g)
	authURL, err := authedGitURL(repoURL, g.Provider, token)
	if err != nil {
		return "", fmt.Errorf("invalid repository URL: %w", err)
	}

	dest := filepath.Join(baseDir, "repo")
	args := []string{"clone", "--depth", "1"}
	if ref := strings.TrimSpace(g.Ref); ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, authURL, dest)

	cmd := exec.CommandContext(ctx, "git", args...) // #nosec G204
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	// Redact the token from anything git prints (e.g. the URL echoed on error).
	sw := &redactWriter{w: logw, secret: token}
	cmd.Stdout = sw
	cmd.Stderr = sw

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git clone failed (check the URL, token, and that the repository exists): %w", err)
	}
	return dest, nil
}

// redactWriter masks a secret in everything written through it.
type redactWriter struct {
	w      io.Writer
	secret string
}

func (rw *redactWriter) Write(p []byte) (int, error) {
	if rw.secret == "" {
		return rw.w.Write(p)
	}
	masked := strings.ReplaceAll(string(p), rw.secret, "***")
	if _, err := rw.w.Write([]byte(masked)); err != nil {
		return 0, err
	}
	return len(p), nil
}
