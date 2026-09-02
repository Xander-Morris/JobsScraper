package llm

import (
	"slices"
	"strings"
	"testing"
)

func TestFormatResumeProfileIncludesOnlyGivenInfo(t *testing.T) {
	profile := ResumeProfile{
		FullName: "Jane Doe",
		Summary:  "Backend engineer",
		Skills:   []string{"Go", "Postgres"},
		WorkExperience: []WorkExperienceEntry{
			{Company: "Acme", JobTitle: "Engineer", StartDate: "2020-01-01", Bullets: []string{"Shipped the thing"}},
		},
	}

	text := formatResumeProfile(profile)

	for _, want := range []string{"Jane Doe", "Backend engineer", "Go, Postgres", "Acme", "Engineer", "Shipped the thing", "present"} {
		if !strings.Contains(text, want) {
			t.Errorf("formatResumeProfile() missing %q, got:\n%s", want, text)
		}
	}
}

func TestFormatJobPosting(t *testing.T) {
	text := formatJobPosting(JobPosting{Title: "Backend Engineer", Company: "Acme", Description: "Build things"})

	for _, want := range []string{"Backend Engineer", "Acme", "Build things"} {
		if !strings.Contains(text, want) {
			t.Errorf("formatJobPosting() missing %q, got:\n%s", want, text)
		}
	}
}

func TestGenerationSchemaRequiresBothFields(t *testing.T) {
	schema := generationSchema()
	required, ok := schema["required"].([]string)

	if !ok {
		t.Fatalf("schema[required] is not []string: %v", schema["required"])
	}

	for _, field := range []string{"cover_letter", "tailored_bullets"} {
		if !slices.Contains(required, field) {
			t.Errorf("generationSchema() required missing %q", field)
		}
	}
}
