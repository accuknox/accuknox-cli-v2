// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend

import (
	_ "embed" // need for embedding
	"fmt"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/kubearmor/kubearmor-client/k8s"
	log "github.com/sirupsen/logrus"
)

var tempDir string // temporary directory used by accuknox-cli to save image etc

// ImageInfo contains image information
type ImageInfo struct {
	Name       string
	RepoTags   []string
	Arch       string
	Distro     string
	OS         string
	FileList   []string
	DirList    []string
	Namespace  string
	Deployment string
	Labels     LabelMap
}

// AuthConfigurations contains the configuration information's
type AuthConfigurations struct {
	Configs map[string]types.AuthConfig `json:"configs"`
}

func (img *ImageInfo) getPolicyDir() string {
	var policyDir string

	if img.Deployment == "" {
		// policy recommendation for container images
		if img.Namespace == "" {
			policyDir = mkPathFromTag(img.RepoTags[0])
		} else {
			policyDir = fmt.Sprintf("%s-%s", img.Namespace, mkPathFromTag(img.RepoTags[0]))
		}
	} else {
		// policy recommendation based on k8s manifest
		policyDir = fmt.Sprintf("%s-%s", img.Namespace, img.Deployment)
	}
	return filepath.Join(options.OutDir, policyDir)
}

func (img *ImageInfo) getPolicyFile(spec string) string {
	var policyFile string

	if img.Deployment != "" {
		// policy recommendation based on k8s manifest
		policyFile = fmt.Sprintf("%s-%s.yaml", mkPathFromTag(img.RepoTags[0]), spec)
	} else {
		policyFile = fmt.Sprintf("%s.yaml", spec)
	}

	return filepath.Join(img.getPolicyDir(), policyFile)
}

func imageHandler(namespace, deployment string, labels LabelMap, imageName string, c *k8s.Client) error {
	img := ImageInfo{
		Name:       imageName,
		Namespace:  namespace,
		Deployment: deployment,
		Labels:     labels,
	}

	if len(options.Policy) == 0 {
		return fmt.Errorf("no policy specified, specify at least one policy to be recommended")
	}

	policiesToBeRecommendedSet := make(map[string]bool)
	for _, policy := range options.Policy {
		policiesToBeRecommendedSet[policy] = true
	}

	_, containsKubeArmorPolicy := policiesToBeRecommendedSet[KubeArmorPolicy]
	_, containsKyvernoPolicy := policiesToBeRecommendedSet[KyvernoPolicy]

	// Admission Controller Policies are not recommended based on an image
	if len(options.Images) == 0 && (containsKyvernoPolicy || containsKubeArmorPolicy) {
		if err := ReportStart(&img); err != nil {
			log.WithError(err).Error("report start failed")
			return err
		}

		_ = ReportSectEnd(&img)

		if len(img.RepoTags) == 0 {
			img.RepoTags = append(img.RepoTags, img.Name)
		}
		if !containsKubeArmorPolicy {
			if err := ReportStart(&img); err != nil {
				log.WithError(err).Error("report start failed")
				return err
			}
		}

		err := initClientConnection(c)
		if err != nil {
			log.WithError(err).Error("failed to initialize client connection.")
			return err
		}
		err = recommendHardeningPolicy(img)
		if err != nil {
			log.WithError(err).Error("failed to recommend hardening policy")
			return err
		}

		// Commenting this out for now, since we don't have admission control policies
		// supported currently from the discovery enigne; mainly to avoid unnecessary calls
		// made to connect to dev2.
		/* err = recommendAdmissionControllerPolicies(img)
		if err != nil {
			log.WithError(err).Error("failed to recommend admission controller policies.")
			return err
		} */
	}

	if !containsKyvernoPolicy && !containsKubeArmorPolicy {
		return fmt.Errorf("policy type not supported: %v", options.Policy)
	}

	return nil
}
