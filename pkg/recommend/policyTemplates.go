// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend

import (
	"context"
	"os"
	"runtime"

	"github.com/google/go-github/github"
	log "github.com/sirupsen/logrus"
)

const (
	org   = "kubearmor"
	repo  = "policy-templates"
	url   = "https://github.com/kubearmor/policy-templates/archive/refs/tags/"
	cache = ".cache/accuknox-cli/"
)

// CurrentVersion stores the current version of policy-template
var CurrentVersion string

// LatestVersion stores the latest version of policy-template
var LatestVersion string

// UserHome function returns users home directory
func UserHome() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}

func latestRelease() string {
	latestRelease, _, err := github.NewClient(nil).Repositories.GetLatestRelease(context.Background(), org, repo)
	if err != nil {
		log.WithError(err)
		return ""
	}
	return *latestRelease.TagName
}

func isLatest() bool {
	LatestVersion = latestRelease()

	if LatestVersion == "" {
		// error while fetching latest release tag
		// assume the current release is the latest one
		return true
	}
	return (CurrentVersion == LatestVersion)
}
