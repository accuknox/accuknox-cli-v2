package report

import (
	"reflect"
	"testing"
)

func TestParsePathInfo(t *testing.T) {
	tests := []struct {
		name string
		path string
		want map[K]V
	}{
		{
			path: "",
			want: map[K]V{},
		},
		{
			path: "/",
			want: map[K]V{},
		},
		{
			path: "/key1/value1",
			want: map[K]V{"key1": "value1"},
		},
		{
			name: "Test 4: Path with multiple key-value pairs",
			path: "/key1/value1/key2/value2",
			want: map[K]V{"key1": "value1", "key2": "value2"},
		},
		{
			path: "/key1/value1/key2",
			want: map[K]V{"key1": "value1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parsePathInfo(tt.path); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parsePathInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}
