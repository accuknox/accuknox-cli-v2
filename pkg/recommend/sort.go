package recommend

import (
	"strings"

	policyType "github.com/accuknox/dev2/hardening/pkg/types"
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
