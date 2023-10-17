// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	hardening "github.com/accuknox/dev2/hardening/pkg/types"
	"github.com/clarketm/json"
	"github.com/fatih/color"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

func mkPathFromTag(tag string) string {
	r := strings.NewReplacer(
		"/", "-",
		":", "-",
		"\\", "-",
		".", "-",
		"@", "-",
	)
	return r.Replace(tag)
}

func (img *ImageInfo) writeAdmissionControllerPolicy(policy kyvernov1.Policy) {
	policyName := strings.ReplaceAll(policy.Name, img.Name+"-", "")
	outFile := img.getPolicyFile(policyName)
	_ = os.MkdirAll(filepath.Dir(outFile), 0750)

	f, err := os.Create(filepath.Clean(outFile))
	if err != nil {
		log.WithError(err).Error(fmt.Sprintf("create file %s failed", outFile))

	}

	arr, _ := json.Marshal(policy)
	yamlArr, _ := yaml.JSONToYAML(arr)
	if _, err := f.WriteString(string(yamlArr)); err != nil {
		log.WithError(err).Error("WriteString failed")
	}
	if err := f.Sync(); err != nil {
		log.WithError(err).Error("file sync failed")
	}
	if err := f.Close(); err != nil {
		log.WithError(err).Error("file close failed")
	}
	_ = ReportAdmissionControllerRecord(outFile, string(policy.Spec.ValidationFailureAction), policy.Annotations)
	color.Green("created policy %s ...", outFile)
}

func (img *ImageInfo) writeHardeningPolicy(policy hardening.KubeArmorPolicy) {
	policyName := policy.Metadata.Name
	outFile := img.getPolicyFile(policyName)
	_ = os.MkdirAll(filepath.Dir(outFile), 0750)

	f, err := os.Create(filepath.Clean(outFile))
	if err != nil {
		log.WithError(err).Error(fmt.Sprintf("create file %s failed", outFile))
	}

	arr, _ := json.Marshal(policy)
	yamlStruct, _ := yaml.JSONToYAML(arr)
	if _, err := f.WriteString(string(yamlStruct)); err != nil {
		log.WithError(err).Error("WriteString failed for hardening policy")
	}
	if err := f.Sync(); err != nil {
		log.WithError(err).Error("file sync failed")
	}
	if err := f.Close(); err != nil {
		log.WithError(err).Error("file sync failed")
	}
	_ = ReportHardeningControllerRecord(outFile, policy.Spec.Action, policy.Spec.Severity, policy.Metadata.Annotations, policy.Spec.Tags)
	// Commenting this out due to extremely verbose logs generated making the experience cluttered
	color.Green("hardening policy created %s ...", outFile)
}
