package llm

import (
	"slices"
	"strings"
	"testing"
)

func testExtractedResume() ExtractedResume {
	return ExtractedResume{
		FullName: "Jane Doe",
		Email:    "jane@example.com",
		Summary:  "Backend engineer",
		Skills:   []string{"Go", "Postgres", "React"},
		WorkExperience: []WorkExperienceEntry{
			{Company: "Acme", JobTitle: "Engineer", StartDate: "2022-01-01", Bullets: []string{"Built APIs", "Ran on-call"}},
			{Company: "Globex", JobTitle: "Intern", StartDate: "2021-06-01", EndDate: "2021-08-01", Bullets: []string{"Wrote tests"}},
		},
		Projects: []ProjectEntry{
			{Name: "Crawler", Technologies: []string{"Go"}, Bullets: []string{"Indexed jobs"}},
			{Name: "Game", Bullets: []string{"Made a game"}},
		},
	}
}

func TestParseSourceID(t *testing.T) {
	tests := []struct {
		id, prefix      string
		section, bullet int
		ok              bool
	}{
		{"W0", "W", 0, -1, true},
		{"W3.B1", "W", 3, 1, true},
		{"[P2.B0]", "P", 2, 0, true},
		{"P1", "W", 0, 0, false},
		{"W", "W", 0, 0, false},
		{"W-1", "W", 0, 0, false},
		{"W1.X2", "W", 0, 0, false},
		{"", "W", 0, 0, false},
	}

	for _, tt := range tests {
		section, bullet, ok := parseSourceID(tt.id, tt.prefix)
		if ok != tt.ok || (ok && (section != tt.section || bullet != tt.bullet)) {
			t.Errorf("parseSourceID(%q, %q) = (%d, %d, %v), want (%d, %d, %v)",
				tt.id, tt.prefix, section, bullet, ok, tt.section, tt.bullet, tt.ok)
		}
	}
}

func TestFormatIndexedResumeLabelsEveryItem(t *testing.T) {
	text := formatIndexedResume(testExtractedResume())

	for _, want := range []string{"[W0] Role: Engineer at Acme", "[W0.B1] Ran on-call", "[W1.B0] Wrote tests", "[P0] Project: Crawler (Go)", "[P1.B0] Made a game"} {
		if !strings.Contains(text, want) {
			t.Errorf("formatIndexedResume() missing %q, got:\n%s", want, text)
		}
	}
}

func TestHydrateTailoredResumeKeepsOnlyValidSources(t *testing.T) {
	result := TailoringResult{
		Summary: "Go engineer for your team",
		Skills:  []string{"postgres", "Kubernetes", "Go", "GO"},
		WorkExperience: []TailoredSection{
			{Source: "W0", Bullets: []TailoredBullet{
				{Text: "Ran on-call for Go services", Source: "W0.B1"},
				{Text: "Invented bullet", Source: "W0.B9"},
				{Text: "Borrowed from another role", Source: "W1.B0"},
				{Text: "  ", Source: "W0.B0"},
			}},
			{Source: "W9", Bullets: []TailoredBullet{{Text: "Fake role", Source: "W9.B0"}}},
		},
		Projects: []TailoredSection{
			{Source: "P1", Bullets: []TailoredBullet{{Text: "Made a game in Go", Source: "P1.B0"}}},
			{Source: "P7"},
		},
	}

	got := hydrateTailoredResume(testExtractedResume(), result)

	if got.FullName != "Jane Doe" || got.Email != "jane@example.com" {
		t.Errorf("contact = %q/%q, want copied from source", got.FullName, got.Email)
	}
	if got.Summary != "Go engineer for your team" {
		t.Errorf("Summary = %q", got.Summary)
	}
	if want := []string{"Postgres", "Go"}; !slices.Equal(got.Skills, want) {
		t.Errorf("Skills = %v, want %v", got.Skills, want)
	}

	if len(got.WorkExperience) != 2 {
		t.Fatalf("len(WorkExperience) = %d, want 2", len(got.WorkExperience))
	}

	acme := got.WorkExperience[0]
	if acme.Company != "Acme" || acme.JobTitle != "Engineer" || acme.StartDate != "2022-01-01" {
		t.Errorf("role facts = %+v, want copied from source", acme)
	}
	if want := []TailoredBullet{{Text: "Ran on-call for Go services", Source: "W0.B1"}}; !slices.Equal(acme.Bullets, want) {
		t.Errorf("Acme bullets = %v, want %v", acme.Bullets, want)
	}

	if want := []TailoredBullet{{Text: "Wrote tests", Source: "W1.B0"}}; !slices.Equal(got.WorkExperience[1].Bullets, want) {
		t.Errorf("omitted role bullets = %v, want originals %v", got.WorkExperience[1].Bullets, want)
	}

	if len(got.Projects) != 1 || got.Projects[0].Name != "Game" {
		t.Fatalf("Projects = %+v, want only Game", got.Projects)
	}
}

func TestHydrateTailoredResumeFallsBackOnEmptyResult(t *testing.T) {
	source := testExtractedResume()
	got := hydrateTailoredResume(source, TailoringResult{})

	if got.Summary != source.Summary {
		t.Errorf("Summary = %q, want %q", got.Summary, source.Summary)
	}
	if !slices.Equal(got.Skills, source.Skills) {
		t.Errorf("Skills = %v, want %v", got.Skills, source.Skills)
	}
	if len(got.WorkExperience) != 2 || len(got.WorkExperience[0].Bullets) != 2 {
		t.Errorf("WorkExperience = %+v, want both roles with original bullets", got.WorkExperience)
	}
	if got.Projects == nil || len(got.Projects) != 0 {
		t.Errorf("Projects = %v, want empty non-nil", got.Projects)
	}
	if got.Education == nil {
		t.Error("Education is nil, want empty slice")
	}
}

func TestTailoringSchemaRequiresAllFields(t *testing.T) {
	required, ok := tailoringSchema()["required"].([]string)
	if !ok {
		t.Fatal("schema[required] is not []string")
	}

	for _, field := range []string{"summary", "skills", "work_experience", "projects"} {
		if !slices.Contains(required, field) {
			t.Errorf("tailoringSchema() required missing %q", field)
		}
	}
}
