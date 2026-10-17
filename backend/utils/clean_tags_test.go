package utils

import (
	"slices"
	"testing"
)

func TestCleanTags(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{"nil", nil, []string{}},
		{"trims and drops empty", []string{"  go ", "", "   "}, []string{"go"}},
		{"html entities", []string{"C&amp;C++", "R&amp;D"}, []string{"C&C++", "R&D"}},
		{"unicode escapes", []string{`café`}, []string{"café"}},
		{"keeps order", []string{"b", "a"}, []string{"b", "a"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CleanTags(tt.input); !slices.Equal(got, tt.want) {
				t.Errorf("CleanTags(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
