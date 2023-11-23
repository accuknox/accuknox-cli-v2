package recommend

import (
	"regexp"
	"strings"

	policyType "github.com/accuknox/dev2/hardening/pkg/types"
	"github.com/kubearmor/kubearmor-client/k8s"
)

// Sort will essentially "bucket sort" all the policies into buckets,
// this is not a lexicographic sort. This is just to group a set of
// policies together based on specific attributes. This helps in two ways
// faster filtering and targeted visualization. The design is based on
// a forest like data structure where namespaces are roots and attributes are
// first level child nodes with pointer to policies are the leafs, creating n number
// of trees.

type Range int
type Action string
type Tag string
type Label string

type AttributeBucket struct {
	Severties map[Range][]*policyType.KubeArmorPolicy
	Actions   map[Action][]*policyType.KubeArmorPolicy
	Tags      map[Tag][]*policyType.KubeArmorPolicy
	Labels    map[Label][]*policyType.KubeArmorPolicy
}

type PolicyBucket struct {
	Namespaces map[string]*AttributeBucket
}

func NewPolicyBucket() *PolicyBucket {
	return &PolicyBucket{
		Namespaces: make(map[string]*AttributeBucket),
	}
}

func (pb *PolicyBucket) getOrCreate(namespace string) *AttributeBucket {
	if _, ok := pb.Namespaces[namespace]; !ok {
		pb.Namespaces[namespace] = &AttributeBucket{
			Severties: make(map[Range][]*policyType.KubeArmorPolicy),
			Actions:   make(map[Action][]*policyType.KubeArmorPolicy),
			Tags:      make(map[Tag][]*policyType.KubeArmorPolicy),
			Labels:    make(map[Label][]*policyType.KubeArmorPolicy),
		}
	}

	return pb.Namespaces[namespace]
}

func (pb *PolicyBucket) AddPolicy(namespace string, policy *policyType.KubeArmorPolicy) {
	ab := pb.getOrCreate(namespace)

	ab.Severties[Range(policy.Spec.Severity)] = append(ab.Severties[Range(policy.Spec.Severity)], policy)
	ab.Actions[Action(policy.Spec.Action)] = append(ab.Actions[Action(policy.Spec.Action)], policy)

	for _, tag := range policy.Spec.Tags {
		ab.Tags[Tag(tag)] = append(ab.Tags[Tag(tag)], policy)
	}

	serializedLabels := serializeLabels(policy.Spec.Selector.MatchLabels)
	if serializedLabels != "" {
		ab.Labels[Label(serializedLabels)] = append(ab.Labels[Label(serializedLabels)], policy)
	}
}

func (pb *PolicyBucket) RetrievePolicies(c *k8s.Client, o *Options) ([]*policyType.KubeArmorPolicy, error) {
	var retrievedPolicies []*policyType.KubeArmorPolicy

	validNamespaces, err := pb.getValidNamespaces(c, o)
	if err != nil {
		return nil, err
	}

	for _, ns := range validNamespaces {
		ab := pb.getOrCreate(ns)
		retrievedPolicies = append(retrievedPolicies, filtration(ab, o)...)
	}

	return retrievedPolicies, nil
}

func (pb *PolicyBucket) getValidNamespaces(c *k8s.Client, o *Options) ([]string, error) {
	if len(o.Namespace) == 0 && len(o.NamespaceRegex) == 0 {
		var allNamespaces []string
		for ns := range pb.Namespaces {
			allNamespaces = append(allNamespaces, ns)
		}
		return allNamespaces, nil
	}

	validNamespaces := o.Namespace

	if len(o.NamespaceRegex) > 0 {
		allNamespaces, err := getAllNamespaces(c)
		if err != nil {
			return nil, err
		}

		for _, regex := range o.NamespaceRegex {
			for _, ns := range allNamespaces {
				if regex.MatchString(ns) {
					validNamespaces = append(validNamespaces, ns)
				}
			}
		}
	}

	return validNamespaces, nil
}

func getAllPoliciesInBucket(ab *AttributeBucket) []*policyType.KubeArmorPolicy {
	seen := make(map[*policyType.KubeArmorPolicy]bool)
	var allPolicies []*policyType.KubeArmorPolicy

	addUniquePolicies := func(policies []*policyType.KubeArmorPolicy) {
		for _, policy := range policies {
			if !seen[policy] {
				seen[policy] = true
				allPolicies = append(allPolicies, policy)
			}
		}
	}

	for _, policies := range ab.Actions {
		addUniquePolicies(policies)
	}
	for _, policies := range ab.Severties {
		addUniquePolicies(policies)
	}
	for _, policies := range ab.Tags {
		addUniquePolicies(policies)
	}
	for _, policies := range ab.Labels {
		addUniquePolicies(policies)
	}

	return allPolicies
}

func severityMatches(policy *policyType.KubeArmorPolicy, severities []int) bool {
	for _, sev := range severities {
		if policy.Spec.Severity == sev {
			return true
		}
	}
	return false
}

func tagsMatch(policy *policyType.KubeArmorPolicy, tags []string, tagsRegex []*regexp.Regexp) bool {
	for _, tag := range policy.Spec.Tags {
		if contains(tags, tag) || regexMatch(tagsRegex, tag) {
			return true
		}
	}
	return false
}

func labelsMatch(policy *policyType.KubeArmorPolicy, labels []string, labelsRegex []*regexp.Regexp) bool {
	serializedLabels := serializeLabels(policy.Metadata.Labels)
	for _, label := range labels {
		if strings.Contains(serializedLabels, label) {
			return true
		}
	}
	for _, regex := range labelsRegex {
		if regex.MatchString(serializedLabels) {
			return true
		}
	}
	return false
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func regexMatch(regexes []*regexp.Regexp, item string) bool {
	for _, regex := range regexes {
		if regex.MatchString(item) {
			return true
		}
	}
	return false
}

func serializeLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	var keys []string
	for k := range labels {
		keys = append(keys, k)
	}

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(labels[k])
		sb.WriteString(";")
	}

	res := sb.String()
	return res
}

func filtration(ab *AttributeBucket, o *Options) []*policyType.KubeArmorPolicy {
	var filteredPolicies []*policyType.KubeArmorPolicy

	for _, policy := range getAllPoliciesInBucket(ab) {
		if !matchesFilters(policy, o) {
			continue
		}

		filteredPolicies = append(filteredPolicies, policy)
	}

	return filteredPolicies
}

func matchesFilters(policy *policyType.KubeArmorPolicy, o *Options) bool {
	if len(o.Namespace) > 0 && !contains(o.Namespace, policy.Metadata.Namespace) {
		return false
	}
	if len(o.SeveritySlice) > 0 && !severityMatches(policy, o.SeveritySlice) {
		return false
	}
	if len(o.Tags) > 0 && !tagsMatch(policy, o.Tags, o.TagsRegex) {
		return false
	}
	if len(o.Labels) > 0 && !labelsMatch(policy, o.Labels, o.LabelsRegex) {
		return false
	}

	return true
}
