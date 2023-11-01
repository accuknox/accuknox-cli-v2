package discover

import (
	"reflect"
	"testing"

	"github.com/kubearmor/kubearmor-client/k8s"
)

var mockPolicyHandler = policyHandler{fn: func(c *k8s.Client, p *Options) ([]string, error) {
	return nil, nil
}}

func mockPolicies() map[string]policyHandler {
	return map[string]policyHandler{
		KindK8sNetworkPolicy:    mockPolicyHandler,
		KindKubeArmorPolicy:     mockPolicyHandler,
		KindKubeArmorHostPolicy: mockPolicyHandler,
	}
}

func TestDeterminePoliciesToProcess(t *testing.T) {
	tests := []struct {
		name        string
		parsedArgs  *Options
		want        map[string]bool
		expectError bool
	}{
		{
			name: "default kind when none specified",
			parsedArgs: &Options{
				Kind: []string{},
			},
			want: map[string]bool{
				KindK8sNetworkPolicy:    false,
				KindKubeArmorPolicy:     true,
				KindKubeArmorHostPolicy: false,
			},
			expectError: false,
		},
		{
			name: "specified kind is supported",
			parsedArgs: &Options{
				Kind: []string{KindK8sNetworkPolicy},
			},
			want: map[string]bool{
				KindK8sNetworkPolicy:    true,
				KindKubeArmorPolicy:     false,
				KindKubeArmorHostPolicy: false,
			},
			expectError: false,
		},
		{
			name: "specified kind is not supported",
			parsedArgs: &Options{
				Kind: []string{"UnsupportedPolicy"},
			},
			want:        nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := determinePoliciesToProcess(tt.parsedArgs, mockPolicies())
			if (err != nil) != tt.expectError {
				t.Errorf("determinePoliciesToProcess() error = %v, expectError %v", err, tt.expectError)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("determinePoliciesToProcess() got = %v, want %v", got, tt.want)
			}
		})
	}
}
