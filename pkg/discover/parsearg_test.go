package discover

import (
	"reflect"
	"regexp"
	"testing"
)

// This purely tests the functionality of parsing ability of the regex library
// so we may see absurd test cases which dont make sense (need to add more cases)
func TestProcessArgs(t *testing.T) {
	tests := []struct {
		name     string
		rawArgs  string
		expected *Options
		isError  bool
	}{
		{
			name:    "Valid gRPC and Format Flags",
			rawArgs: "--gRPC localhost:50051 --format json",
			expected: &Options{
				GRPC:   "localhost:50051",
				Format: "json",
			},
		},
		{
			name:    "Invalid Format Flag",
			rawArgs: "--format invalid",
			isError: true,
		},
		{
			name:    "Valid Namespace and Labels Flags",
			rawArgs: "--namespace default --labels app=web",
			expected: &Options{
				Namespace: []string{"default"},
				Labels:    []string{"app=web"},
			},
		},
		{
			name:    "Valid Namespace and Source Regex Flags",
			rawArgs: "--namespace kube_* --source log_* --labels app=web,fly=high,name=what",
			expected: &Options{
				NamespaceRegex: []*regexp.Regexp{regexp.MustCompile("kube_*")},
				SourceRegex:    []*regexp.Regexp{regexp.MustCompile("log_*")},
				Labels:         []string{"app=web", "fly=high", "name=what"},
			},
		},
		{
			name:    "Valid Include Network Flag",
			rawArgs: "--includenet",
			expected: &Options{
				IncludeNetwork: true,
			},
		},
		{
			name:    "Valid Shorthand Flags",
			rawArgs: "-g localhost:50051 -f json -n default -l app=web --includenet",
			expected: &Options{
				GRPC:           "localhost:50051",
				Format:         "json",
				Namespace:      []string{"default"},
				Labels:         []string{"app=web"},
				IncludeNetwork: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := ProcessArgs(tt.rawArgs)
			if tt.isError {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if !compareOptions(tt.expected, actual) {
					t.Errorf("Expected: %+v, Actual: %+v", tt.expected, actual)
				}
			}
		})
	}
}

func compareOptions(expected, actual *Options) bool {
	if expected.GRPC != actual.GRPC ||
		expected.Format != actual.Format ||
		expected.IncludeNetwork != actual.IncludeNetwork ||
		!reflect.DeepEqual(expected.Kind, actual.Kind) ||
		!reflect.DeepEqual(expected.Namespace, actual.Namespace) ||
		!reflect.DeepEqual(expected.Labels, actual.Labels) ||
		!reflect.DeepEqual(expected.Source, actual.Source) {
		return false
	}

	if !compareRegexSlices(expected.NamespaceRegex, actual.NamespaceRegex) ||
		!compareRegexSlices(expected.LabelsRegex, actual.LabelsRegex) ||
		!compareRegexSlices(expected.SourceRegex, actual.SourceRegex) {
		return false
	}

	return true
}

func compareRegexSlices(expected, actual []*regexp.Regexp) bool {
	if len(expected) != len(actual) {
		return false
	}
	for i := range expected {
		if expected[i].String() != actual[i].String() {
			return false
		}
	}
	return true
}
