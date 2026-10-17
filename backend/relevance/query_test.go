package relevance

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"main/database"
	"main/llm"
)

func TestSplitTerms(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"Go", []string{"Go"}},
		{"Go, Python; Rust", []string{"Go", "Python", "Rust"}},
		{"CI/CD | Docker & Kubernetes", []string{"CI", "CD", "Docker", "Kubernetes"}},
		{"React and Redux", []string{"React", "Redux"}},
		{"Android", []string{"Android"}},
		{" ,, ;; ", nil},
		{"Node.js: Express", []string{"Node.js", "Express"}},
	}

	for _, tt := range tests {
		if got := splitTerms(tt.input); !slices.Equal(got, tt.want) {
			t.Errorf("splitTerms(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSearchQuery(t *testing.T) {
	extraction := database.ResumeExtraction{
		ExtractedResume: llm.ExtractedResume{
			Skills: []string{"Go, Python", "go", "SQL"},
			Education: []llm.EducationEntry{
				{Major: "Computer Science"},
				{Major: ""},
			},
			WorkExperience: []llm.WorkExperienceEntry{
				{JobTitle: "Backend Engineer"},
				{JobTitle: ""},
			},
			Projects: []llm.ProjectEntry{
				{Technologies: []string{"React", "python", "Docker/Kubernetes"}},
			},
		},
	}

	got := SearchQuery(extraction)
	want := "Go OR Python OR SQL OR Computer Science OR Backend Engineer OR React OR Docker OR Kubernetes"

	if got != want {
		t.Errorf("SearchQuery() = %q, want %q", got, want)
	}
}

func TestSearchQueryEmpty(t *testing.T) {
	if got := SearchQuery(database.ResumeExtraction{}); got != "" {
		t.Errorf("SearchQuery(empty) = %q, want empty", got)
	}
}

func TestSearchQueryCapsTerms(t *testing.T) {
	var skills []string
	for i := range maxResumeQueryTerms + 10 {
		skills = append(skills, fmt.Sprintf("skill%d", i))
	}

	got := SearchQuery(database.ResumeExtraction{ExtractedResume: llm.ExtractedResume{Skills: skills}})
	terms := strings.Split(got, " OR ")

	if len(terms) != maxResumeQueryTerms {
		t.Fatalf("got %d terms, want %d", len(terms), maxResumeQueryTerms)
	}

	if terms[0] != "skill0" || terms[len(terms)-1] != fmt.Sprintf("skill%d", maxResumeQueryTerms-1) {
		t.Errorf("terms not kept in order: first %q, last %q", terms[0], terms[len(terms)-1])
	}
}
