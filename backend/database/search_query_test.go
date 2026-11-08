package database

import (
	"context"
	"main/jobs"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pgvector/pgvector-go"
)

func TestParseSearchQuery(t *testing.T) {
	tests := []struct {
		query string
		want  [][]string
	}{
		{"", nil},
		{"the and of", nil},
		{"Intern Web Dev", [][]string{internTerm.variants, webTerm.variants, devTerm.variants}},
		{"front-end developer", [][]string{frontendTerm.variants, devTerm.variants}},
		{"Full Stack", [][]string{fullstackTerm.variants}},
		{"IT support", [][]string{itTerm.variants, {"support"}}},
		{"developer devs engineer", [][]string{devTerm.variants}},
		{"python, rust & c++!", [][]string{{"python"}, {"rust"}, {"c"}}},
		{"a1 b2 c3 d4 e5 f6 g7 h8 i9", [][]string{{"a1"}, {"b2"}, {"c3"}, {"d4"}, {"e5"}, {"f6"}, {"g7"}, {"h8"}}},
	}

	for _, tt := range tests {
		got := parseSearchQuery(tt.query)

		if len(got) != len(tt.want) {
			t.Errorf("parseSearchQuery(%q) = %d terms, want %d", tt.query, len(got), len(tt.want))
			continue
		}

		for i := range got {
			if !slices.Equal(got[i].variants, tt.want[i]) {
				t.Errorf("parseSearchQuery(%q)[%d] = %v, want %v", tt.query, i, got[i].variants, tt.want[i])
			}
		}
	}
}

func TestSearchTermTsqueryText(t *testing.T) {
	term := searchTerm{variants: []string{"web", "front end"}}

	if got, want := term.tsqueryText("BD"), "(web:BD) | (front:BD <-> end:BD)"; got != want {
		t.Errorf("tsqueryText = %q, want %q", got, want)
	}
}

func seedSearchJobs(t *testing.T, seed []jobs.Job) map[string]int64 {
	t.Helper()

	if err := WriteJobsToDatabase(t.Context(), seed); err != nil {
		t.Fatalf("seed db: %v", err)
	}

	ids := map[string]int64{}

	for _, job := range seed {
		ids[job.Title] = jobIDByURL(t, job.URL)
	}

	return ids
}

func searchTitles(t *testing.T, params *JobSearchParams) []string {
	t.Helper()

	params.Limit = MaxSearchLimit
	result, err := SearchForJobs(context.Background(), params)

	if err != nil {
		t.Fatalf("SearchForJobs(%q): %v", params.SearchQuery, err)
	}

	var titles []string

	for _, job := range result.Jobs {
		titles = append(titles, job.Title)
	}

	return titles
}

func TestSearchForJobsTypedQueryRelevance(t *testing.T) {
	newTestDB(t)

	job := func(title, description string) jobs.Job {
		return jobs.Job{Title: title, Company: "Acme", PostedAt: time.Now(), Description: description,
			URL: "https://example.com/jobs/" + strings.ReplaceAll(strings.ToLower(title), " ", "-")}
	}

	seedSearchJobs(t, []jobs.Job{
		job("Staff Product Manager", "Partner with internal web dev teams on international launches."),
		job("Software Engineering Intern", "Build web apps with our team."),
		job("Senior Frontend Engineer", "Own our web platform."),
		job("Account Executive", "Close deals with accountants."),
		job("Accountant", "Reconcile ledgers."),
		job("Nurse", "Patient care."),
	})

	titles := searchTitles(t, &JobSearchParams{SearchQuery: "intern web dev"})
	if len(titles) == 0 || titles[0] != "Software Engineering Intern" {
		t.Errorf("intern web dev = %v, want the intern first", titles)
	}
	if slices.Contains(titles, "Staff Product Manager") {
		t.Errorf("intern web dev = %v, should skip \"internal\" in a PM posting", titles)
	}

	titles = searchTitles(t, &JobSearchParams{SearchQuery: "accountant"})
	if len(titles) != 2 || titles[0] != "Accountant" {
		t.Errorf("accountant = %v, want Accountant then Account Executive", titles)
	}

	if titles = searchTitles(t, &JobSearchParams{SearchQuery: "the"}); len(titles) != 6 {
		t.Errorf("stopword-only query = %v, want every job", titles)
	}

	titles = searchTitles(t, &JobSearchParams{SearchQuery: "nurse", Sort: SortDate})
	if !slices.Equal(titles, []string{"Nurse"}) {
		t.Errorf("nurse by date = %v", titles)
	}
}

func TestSearchForJobsQueryEmbedding(t *testing.T) {
	newTestDB(t)

	ids := seedSearchJobs(t, []jobs.Job{
		{Title: "Frontend Engineer", Company: "Acme", PostedAt: time.Now(), URL: "https://example.com/jobs/fe", Description: "React."},
		{Title: "Web Developer", Company: "Acme", PostedAt: time.Now(), URL: "https://example.com/jobs/wd", Description: "Sites."},
		{Title: "Nurse", Company: "Acme", PostedAt: time.Now(), URL: "https://example.com/jobs/nurse", Description: "Care."},
	})

	setJobEmbedding(t, ids["Frontend Engineer"], unitVector(0))
	setJobEmbedding(t, ids["Web Developer"], unitVector(1))
	setJobEmbedding(t, ids["Nurse"], unitVector(2))

	queryVec := pgvector.NewVector(unitVector(0))
	resumeVec := pgvector.NewVector(unitVector(1))

	// No keyword matches "zzqx", so the nearest embeddings carry the results.
	titles := searchTitles(t, &JobSearchParams{SearchQuery: "zzqx", SearchEmbedding: &queryVec})
	if len(titles) != 3 || titles[0] != "Frontend Engineer" {
		t.Errorf("semantic-only search = %v, want Frontend Engineer first", titles)
	}

	// Both titles hit "web developer" equally; the query embedding breaks the tie.
	titles = searchTitles(t, &JobSearchParams{SearchQuery: "developer", SearchEmbedding: &queryVec, ResumeEmbedding: &resumeVec})
	if len(titles) == 0 || titles[0] != "Frontend Engineer" {
		t.Errorf("hybrid search = %v, want Frontend Engineer first", titles)
	}
}

func TestTermWeightsCachedUntilJobsChange(t *testing.T) {
	newTestDB(t)
	forgetTermWeights()

	seedJobs(t, jobs.Job{Title: "Go Intern", Company: "Acme", URL: "https://example.com/w1"},
		jobs.Job{Title: "Go Engineer", Company: "Acme", URL: "https://example.com/w2"})

	text := newTextSearch(parseSearchQuery("intern go"), nil)
	if err := text.loadWeights(t.Context()); err != nil {
		t.Fatal(err)
	}

	// intern matches 1 of 2 jobs, go matches both, so intern is rarer.
	want := []float64{math.Log(1 + 2.0/2), math.Log(1 + 2.0/3)}
	if !slices.EqualFunc(text.weights, want, func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }) {
		t.Errorf("weights = %v, want %v", text.weights, want)
	}

	termWeights.Lock()
	cached := len(termWeights.entries)
	termWeights.Unlock()

	if cached != 2 {
		t.Errorf("cached weights = %d, want 2", cached)
	}

	seedJobs(t, jobs.Job{Title: "Nurse", Company: "Acme", URL: "https://example.com/w3"})

	termWeights.Lock()
	cached = len(termWeights.entries)
	termWeights.Unlock()

	if cached != 0 {
		t.Errorf("cached weights after write = %d, want 0", cached)
	}
}
