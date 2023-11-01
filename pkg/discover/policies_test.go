package discover

import (
	"regexp"
	"testing"

	policyType "github.com/accuknox/dev2/discover/pkg/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Mock policies and Options for testing
func mockKubeArmorPolicy(namespace string, labels map[string]string, fromSourcePaths []string, includeNetwork bool) policyType.KubeArmorPolicy {
	matchSources := make([]policyType.KnoxFromSource, len(fromSourcePaths))
	for i, path := range fromSourcePaths {
		matchSources[i] = policyType.KnoxFromSource{Path: path}
	}

	return policyType.KubeArmorPolicy{
		APIVersion: "v1",
		Kind:       "KAPolicy",
		Metadata: metav1.ObjectMeta{
			Namespace: namespace,
		},
		Spec: policyType.KnoxSystemSpec{
			Selector: policyType.Selector{
				MatchLabels: labels,
			},
			Process: policyType.KnoxSys{
				MatchPaths: []policyType.KnoxMatchPaths{
					{
						FromSource: matchSources,
					},
				},
			},
			Network: policyType.NetworkRule{
				MatchProtocols: []policyType.KnoxMatchProtocols{
					{
						Protocol: "TCP",
					},
				},
			},
		},
	}
}

// TODO: Tests can be improved
func TestKaPolicyFilter(t *testing.T) {
	tests := []struct {
		name       string
		policy     policyType.KubeArmorPolicy
		Options    Options
		wantFilter bool
	}{
		{
			name:       "no filter should return true",
			policy:     mockKubeArmorPolicy("", nil, nil, false),
			Options:    Options{},
			wantFilter: true,
		},
		{
			name:       "namespace filter matches should return true",
			policy:     mockKubeArmorPolicy("default", nil, nil, false),
			Options:    Options{Namespace: []string{"default"}},
			wantFilter: true,
		},
		{
			name:       "label filter matches should return true",
			policy:     mockKubeArmorPolicy("default", map[string]string{"app": "web"}, nil, false),
			Options:    Options{Labels: []string{"app=web"}},
			wantFilter: true,
		},
		{
			name:       "source path filter matches should return true",
			policy:     mockKubeArmorPolicy("default", nil, []string{"/usr/bin/curl"}, false),
			Options:    Options{Source: []string{"/usr/bin/curl"}},
			wantFilter: true,
		},
		{
			name:       "include network filter matches should return true",
			policy:     mockKubeArmorPolicy("default", nil, nil, true),
			Options:    Options{IncludeNetwork: true},
			wantFilter: true,
		},
		{
			name:       "namespace regex filter matches should return true",
			policy:     mockKubeArmorPolicy("development", nil, nil, false),
			Options:    Options{NamespaceRegex: []*regexp.Regexp{regexp.MustCompile(`^dev.*`)}},
			wantFilter: true,
		},
		{
			name:       "label regex filter matches should return true",
			policy:     mockKubeArmorPolicy("", map[string]string{"role": "frontend"}, nil, false),
			Options:    Options{LabelsRegex: []*regexp.Regexp{regexp.MustCompile(`role=front.*`)}},
			wantFilter: true,
		},
		{
			name:       "multiple label filters match should return true",
			policy:     mockKubeArmorPolicy("", map[string]string{"role": "db", "tier": "backend"}, nil, false),
			Options:    Options{Labels: []string{"role=db", "tier=backend"}},
			wantFilter: true,
		},
		{
			name:       "label filter does not match should return false",
			policy:     mockKubeArmorPolicy("", map[string]string{"role": "db"}, nil, false),
			Options:    Options{Labels: []string{"role=frontend"}},
			wantFilter: false,
		},
		{
			name:       "label regex filter does not match should return false",
			policy:     mockKubeArmorPolicy("", map[string]string{"role": "backend"}, nil, false),
			Options:    Options{LabelsRegex: []*regexp.Regexp{regexp.MustCompile(`role=front.*`)}},
			wantFilter: false,
		},
		{
			name:       "namespace filter does not match should return false",
			policy:     mockKubeArmorPolicy("production", nil, nil, false),
			Options:    Options{Namespace: []string{"development"}},
			wantFilter: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := kaPolicyFilter(tt.policy, &tt.Options); got != tt.wantFilter {
				t.Errorf("kaPolicyFilter() = %v, want %v", got, tt.wantFilter)
			}
		})
	}
}
