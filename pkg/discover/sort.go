package discover

import (
	"strings"
	"sync"

	policyType "github.com/accuknox/dev2/discover/pkg/common"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

type KubearmorPolicyBucket struct {
	Labels  map[string][]*policyType.KubeArmorPolicy
	Actions map[string][]*policyType.KubeArmorPolicy
}

type NetworkPolicyBucket struct {
	Types     map[string][]*networkingv1.NetworkPolicy
	Protocols map[string][]*networkingv1.NetworkPolicy
}

type NamespaceBucket struct {
	KubearmorHostPolicies KubearmorPolicyBucket
	KubearmorPolicies     KubearmorPolicyBucket
	NetworkPolicies       NetworkPolicyBucket
}

type PolicyForest struct {
	sync.RWMutex
	Namespaces map[string]*NamespaceBucket
}

func NewPolicyForest() *PolicyForest {
	return &PolicyForest{
		Namespaces: map[string]*NamespaceBucket{},
	}
}

// AddKubearmorPolicy adds a KubeArmor policy to the appropriate bucket.
func (pf *PolicyForest) AddKubearmorPolicy(namespace string, policy *policyType.KubeArmorPolicy) {
	if pf.Namespaces[namespace] == nil {
		pf.Namespaces[namespace] = &NamespaceBucket{
			KubearmorPolicies: KubearmorPolicyBucket{
				Labels:  make(map[string][]*policyType.KubeArmorPolicy),
				Actions: make(map[string][]*policyType.KubeArmorPolicy),
			},
		}
	}

	if pf.Namespaces[namespace].KubearmorPolicies.Labels == nil {
		pf.Namespaces[namespace].KubearmorPolicies.Labels = make(map[string][]*policyType.KubeArmorPolicy)
	}
	if pf.Namespaces[namespace].KubearmorPolicies.Actions == nil {
		pf.Namespaces[namespace].KubearmorPolicies.Actions = make(map[string][]*policyType.KubeArmorPolicy)
	}

	labelKey := serializeLabels(policy.Spec.Selector.MatchLabels)
	pf.Namespaces[namespace].KubearmorPolicies.Labels[labelKey] = append(pf.Namespaces[namespace].KubearmorPolicies.Labels[labelKey], policy)

	actionKey := policy.Spec.Action
	pf.Namespaces[namespace].KubearmorPolicies.Actions[actionKey] = append(pf.Namespaces[namespace].KubearmorPolicies.Actions[actionKey], policy)
}

// AddNetworkPolicy adds a KnoxNetwork policy to the appropriate bucket.
func (pf *PolicyForest) AddNetworkPolicy(namespace string, policy *networkingv1.NetworkPolicy) {
	if pf.Namespaces[namespace] == nil {
		pf.Namespaces[namespace] = &NamespaceBucket{
			NetworkPolicies: NetworkPolicyBucket{
				Types:     make(map[string][]*networkingv1.NetworkPolicy),
				Protocols: make(map[string][]*networkingv1.NetworkPolicy),
			},
		}
	}

	if pf.Namespaces[namespace].NetworkPolicies.Types == nil {
		pf.Namespaces[namespace].NetworkPolicies.Types = make(map[string][]*networkingv1.NetworkPolicy)
	}

	if pf.Namespaces[namespace].NetworkPolicies.Protocols == nil {
		pf.Namespaces[namespace].NetworkPolicies.Protocols = make(map[string][]*networkingv1.NetworkPolicy)
	}

	policyExists := func(policies []*networkingv1.NetworkPolicy, name string) bool {
		for _, p := range policies {
			if p.ObjectMeta.Name == name {
				return true
			}
		}
		return false
	}

	addPolicy := func(policyType string, policy *networkingv1.NetworkPolicy) {
		if !policyExists(pf.Namespaces[namespace].NetworkPolicies.Types[policyType], policy.ObjectMeta.Name) {
			pf.Namespaces[namespace].NetworkPolicies.Types[policyType] = append(pf.Namespaces[namespace].NetworkPolicies.Types[policyType], policy)
		}
	}

	addProtocolPolicy := func(protocol string, policy *networkingv1.NetworkPolicy) {
		if !policyExists(pf.Namespaces[namespace].NetworkPolicies.Protocols[protocol], policy.ObjectMeta.Name) {
			pf.Namespaces[namespace].NetworkPolicies.Protocols[protocol] = append(pf.Namespaces[namespace].NetworkPolicies.Protocols[protocol], policy)
		}
	}

	if len(policy.Spec.Ingress) > 0 {
		addPolicy("ingress", policy)
		for _, ingress := range policy.Spec.Ingress {
			for _, port := range ingress.Ports {
				protocol := getDefaultProtocol(port.Protocol)
				addProtocolPolicy(protocol, policy)
			}
		}
	}

	if len(policy.Spec.Egress) > 0 {
		addPolicy("egress", policy)
		for _, egress := range policy.Spec.Egress {
			for _, port := range egress.Ports {
				protocol := getDefaultProtocol(port.Protocol)
				addProtocolPolicy(protocol, policy)
			}
		}
	}
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

func getDefaultProtocol(protocol *corev1.Protocol) string {
	if protocol != nil {
		return string(*protocol)
	}
	return "TCP" // default protocol
}
