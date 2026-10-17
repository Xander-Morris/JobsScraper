package crawler

import (
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"main/jobs"
)

var testNow = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

func mustJob(t *testing.T, script, company string) jobs.Job {
	t.Helper()

	postings := findPostings(script)
	if len(postings) != 1 {
		t.Fatalf("found %d postings, want 1", len(postings))
	}

	job, ok := postings[0].toJob("https://example.com/jobs/1", company, testNow)
	if !ok {
		t.Fatal("toJob rejected the posting")
	}

	return job
}

func TestFindPostingsShapes(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   int
	}{
		{"single object", `{"@type":"JobPosting","title":"A"}`, 1},
		{"array", `[{"@type":"Organization"},{"@type":"JobPosting","title":"A"}]`, 1},
		{"graph", `{"@context":"https://schema.org","@graph":[{"@type":"WebPage"},{"@type":"JobPosting","title":"A"}]}`, 1},
		{"type list and full iri", `[{"@type":["Thing","JobPosting"],"title":"A"},{"@type":"http://schema.org/JobPosting","title":"B"}]`, 2},
		{"raw newline in string", "{\"@type\":\"JobPosting\",\"title\":\"A\",\"description\":\"line one\nline two\"}", 1},
		{"not a posting", `{"@type":"Organization","name":"Acme"}`, 0},
		{"invalid json", `{"@type":`, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := len(findPostings(tt.script)); got != tt.want {
				t.Errorf("found %d postings, want %d", got, tt.want)
			}
		})
	}
}

func TestPostingToJob(t *testing.T) {
	job := mustJob(t, `{
		"@type": "JobPosting",
		"title": "Senior Engineer",
		"description": "<p>Build <b>things</b></p>",
		"datePosted": "2026-09-20",
		"validThrough": "2026-12-01T00:00:00Z",
		"hiringOrganization": {"@type": "Organization", "name": "2100 Acme USA"},
		"jobLocation": [
			{"@type": "Place", "address": {"@type": "PostalAddress", "addressLocality": "Austin", "addressRegion": "TX", "addressCountry": "US"}},
			{"@type": "Place", "address": {"addressCountry": {"@type": "Country", "name": "Canada"}}}
		],
		"employmentType": ["FULL_TIME", "CONTRACTOR"],
		"occupationalCategory": "Engineering",
		"baseSalary": {"@type": "MonetaryAmount", "currency": "USD",
			"value": {"@type": "QuantitativeValue", "minValue": 120000, "maxValue": "150,000", "unitText": "YEAR"}}
	}`, "Acme")

	min, max := 120000, 150000
	assertJobEqual(t, job, jobs.Job{
		Title:       "Senior Engineer",
		Company:     "Acme",
		Location:    "Austin, TX; Canada",
		Tags:        []string{"Full-time", "Contract", "Engineering"},
		SalaryMin:   &min,
		SalaryMax:   &max,
		PostedAt:    time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC),
		URL:         "https://example.com/jobs/1",
		Description: "Build things",
	})
}

func TestPostingToJobFallsBackToHiringOrganization(t *testing.T) {
	job := mustJob(t, `{"@type":"JobPosting","title":"Engineer","hiringOrganization":"Acme Inc"}`, "")

	if job.Company != "Acme Inc" {
		t.Errorf("Company = %q, want the hiringOrganization", job.Company)
	}
}

func TestPostingToJobRemote(t *testing.T) {
	job := mustJob(t, `{"@type":"JobPosting","title":"Engineer","jobLocationType":"TELECOMMUTE",
		"applicantLocationRequirements":[{"@type":"Country","name":"USA"},{"@type":"Country","name":"Canada"}]}`, "Acme")

	if job.WorkplaceType != jobs.Remote || job.Location != "Remote - USA, Canada" {
		t.Errorf("workplace, location = %v, %q, want remote, %q", job.WorkplaceType, job.Location, "Remote - USA, Canada")
	}
}

func TestPostingToJobTitleOverridesTelecommute(t *testing.T) {
	tests := []struct {
		title string
		want  jobs.WorkplaceType
	}{
		{"PCB Lead (Onsite)", jobs.InPerson},
		{"Designer - On-site", jobs.InPerson},
		{"Analyst (Hybrid)", jobs.Hybrid},
	}

	for _, tt := range tests {
		job := mustJob(t, `{"@type":"JobPosting","title":"`+tt.title+`","jobLocationType":"TELECOMMUTE"}`, "Acme")

		if job.WorkplaceType != tt.want {
			t.Errorf("%q: workplace = %v, want %v", tt.title, job.WorkplaceType, tt.want)
		}
	}
}

func TestPostingToJobSalary(t *testing.T) {
	tests := []struct {
		name     string
		salary   string
		min, max *int
	}{
		{"hourly rate becomes yearly", `{"currency":"USD","value":{"value":50,"unitText":"HOUR"}}`, intPtr(104000), intPtr(104000)},
		{"plain number", `{"currency":"USD","value":90000}`, intPtr(90000), intPtr(90000)},
		{"other currency skipped", `{"currency":"EUR","value":{"minValue":60000,"maxValue":80000}}`, nil, nil},
		{"unknown unit skipped", `{"value":{"value":5,"unitText":"PIECE"}}`, nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := mustJob(t, `{"@type":"JobPosting","title":"Engineer","baseSalary":`+tt.salary+`}`, "Acme")

			if !equalIntPtr(job.SalaryMin, tt.min) || !equalIntPtr(job.SalaryMax, tt.max) {
				t.Errorf("salary = %v-%v, want %v-%v", deref(job.SalaryMin), deref(job.SalaryMax), deref(tt.min), deref(tt.max))
			}
		})
	}
}

func TestPostingToJobRejects(t *testing.T) {
	tests := []struct {
		name   string
		script string
	}{
		{"no title", `{"@type":"JobPosting","description":"x"}`},
		{"expired", `{"@type":"JobPosting","title":"Engineer","validThrough":"2026-09-01"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := findPostings(tt.script)[0].toJob("https://example.com/jobs/1", "Acme", testNow); ok {
				t.Error("toJob accepted the posting, want it rejected")
			}
		})
	}
}

func TestPostingToJobTruncatesDescription(t *testing.T) {
	job := mustJob(t, `{"@type":"JobPosting","title":"Engineer","description":"`+strings.Repeat("a", maxDescriptionRunes+10)+`"}`, "Acme")

	if got := len([]rune(job.Description)); got != maxDescriptionRunes+1 {
		t.Errorf("description has %d runes, want %d plus an ellipsis", got, maxDescriptionRunes)
	}
}

func TestParsePage(t *testing.T) {
	base, _ := url.Parse("https://example.com/careers/")

	parsed, err := parsePage(strings.NewReader(`<html><head>
		<script type="application/ld+json">{"@type":"JobPosting","title":"Engineer"}</script>
		<script type="text/javascript">var x = {"@type":"JobPosting"};</script>
	</head><body>
		<a href="jobs/1#apply">One</a>
		<a href="/careers/jobs/2">Two</a>
		<a href="#top">Top</a>
		<a href="mailto:jobs@example.com">Mail</a>
		<a href="https://other.com/x">Other</a>
	</body></html>`), base)
	if err != nil {
		t.Fatalf("parsePage: %v", err)
	}

	if len(parsed.postings) != 1 {
		t.Errorf("found %d postings, want 1 from the ld+json script only", len(parsed.postings))
	}

	want := []string{"https://example.com/careers/jobs/1", "https://example.com/careers/jobs/2", "https://other.com/x"}
	if !slices.Equal(parsed.links, want) {
		t.Errorf("links = %v, want %v", parsed.links, want)
	}
}

func intPtr(n int) *int { return &n }

func equalIntPtr(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}

func deref(p *int) any {
	if p == nil {
		return nil
	}

	return *p
}

func assertJobEqual(t *testing.T, got, want jobs.Job) {
	t.Helper()

	if got.Title != want.Title || got.Company != want.Company || got.Location != want.Location ||
		got.WorkplaceType != want.WorkplaceType || got.URL != want.URL || got.Description != want.Description {
		t.Errorf("job = %+v, want %+v", got, want)
	}

	if !slices.Equal(got.Tags, want.Tags) {
		t.Errorf("Tags = %v, want %v", got.Tags, want.Tags)
	}

	if !got.PostedAt.Equal(want.PostedAt) {
		t.Errorf("PostedAt = %v, want %v", got.PostedAt, want.PostedAt)
	}

	if !equalIntPtr(got.SalaryMin, want.SalaryMin) || !equalIntPtr(got.SalaryMax, want.SalaryMax) {
		t.Errorf("salary = %v-%v, want %v-%v", deref(got.SalaryMin), deref(got.SalaryMax), deref(want.SalaryMin), deref(want.SalaryMax))
	}
}
