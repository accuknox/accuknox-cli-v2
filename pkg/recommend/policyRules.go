// SPDX-License-Identifier: Apache-2.0
// Copyright 2022 Authors of KubeArmor

package recommend

import (
	_ "embed" // need for embedding

	pol "github.com/kubearmor/KubeArmor/pkg/KubeArmorController/api/security.kubearmor.com/v1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
)

// MatchSpec spec to match for defining policy
type MatchSpec struct {
	Name              string                  `json:"name" yaml:"name"`
	Precondition      []string                `json:"precondition" yaml:"precondition"`
	Description       Description             `json:"description" yaml:"description"`
	Yaml              string                  `json:"yaml" yaml:"yaml"`
	Spec              pol.KubeArmorPolicySpec `json:"spec,omitempty" yaml:"spec,omitempty"`
	KyvernoPolicySpec *kyvernov1.Spec         `json:"kyvernoPolicySpec,omitempty" yaml:"kyvernoPolicySpec,omitempty"`
	KyvernoPolicyTags []string                `json:"kyvernoPolicyTags,omitempty" yaml:"kyvernoPolicyTags,omitempty"`
}

// Ref for the policy rules
type Ref struct {
	Name string   `json:"name" yaml:"name"`
	URL  []string `json:"url" yaml:"url"`
}

// Description detailed description for the policy rule
type Description struct {
	Refs     []Ref  `json:"refs" yaml:"refs"`
	Tldr     string `json:"tldr" yaml:"tldr"`
	Detailed string `json:"detailed" yaml:"detailed"`
}
