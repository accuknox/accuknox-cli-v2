package report

import (
	"reflect"
	"testing"
)

func TestMyersDiff(t *testing.T) {
	tests := []struct {
		name   string
		aLines []string
		bLines []string
		want   []EditAction
	}{
		{
			aLines: []string{},
			bLines: []string{},
			want:   []EditAction{},
		},
		{
			aLines: []string{},
			bLines: []string{"Hello"},
			want:   []EditAction{Insert{"Hello"}},
		},
		{
			aLines: []string{"Hello"},
			bLines: []string{"Hello"},
			want:   []EditAction{Keep{"Hello"}},
		},
		{
			aLines: []string{"Hello"},
			bLines: []string{"World"},
			want:   []EditAction{Remove{"Hello"}, Insert{"World"}},
		},
		{
			aLines: []string{"Hello", "World"},
			bLines: []string{"Hello", "Who?"},
			want:   []EditAction{Keep{"Hello"}, Remove{"World"}, Insert{"Who?"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := myersDiff(tt.aLines, tt.bLines); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("myersDiff() = %v, want %v", got, tt.want)
			}
		})
	}
}
