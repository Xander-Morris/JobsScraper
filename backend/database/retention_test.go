package database

import (
	"context"
	"main/jobs"
	"slices"
	"strings"
	"testing"
	"time"
)

func jobTitles(t *testing.T) []string {
	t.Helper()

	rows, err := cachedDb.Query("SELECT title FROM jobs ORDER BY title")
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	defer rows.Close()

	var titles []string
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			t.Fatalf("scan title: %v", err)
		}
		titles = append(titles, title)
	}

	return titles
}

func TestDeleteExpiredJobs(t *testing.T) {
	newTestDB(t)
	ctx := context.Background()
	old := time.Now().Add(-JobMaxAge - 24*time.Hour)

	seed := []jobs.Job{
		{Title: "Fresh", Company: "Acme", PostedAt: time.Now(), URL: "https://example.com/jobs/fresh", Tags: []string{"fresh-tag"}},
		{Title: "Expired", Company: "Acme", PostedAt: old, URL: "https://example.com/jobs/expired", Tags: []string{"expired-tag"}},
		{Title: "Expired Applied", Company: "Acme", PostedAt: old, URL: "https://example.com/jobs/expired-applied"},
		{Title: "Undated", Company: "Acme", URL: "https://example.com/jobs/undated"},
	}

	if err := WriteJobsToDatabase(seed); err != nil {
		t.Fatalf("seed db: %v", err)
	}

	profileID, err := CreateProfile(&ProfileRequest{Email: "retention@example.com", Password: "securepassword123"})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	if err := MarkJobApplied(ctx, profileID, jobIDByURL(t, "https://example.com/jobs/expired-applied")); err != nil {
		t.Fatalf("mark applied: %v", err)
	}

	deleted, err := DeleteExpiredJobs(ctx)
	if err != nil {
		t.Fatalf("DeleteExpiredJobs: %v", err)
	}

	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	if got, want := jobTitles(t), []string{"Expired Applied", "Fresh", "Undated"}; !slices.Equal(got, want) {
		t.Errorf("remaining jobs = %v, want %v", got, want)
	}

	var expiredTags int
	if err := cachedDb.QueryRow("SELECT COUNT(*) FROM tags WHERE tag = 'expired-tag'").Scan(&expiredTags); err != nil {
		t.Fatalf("count tags: %v", err)
	}
	if expiredTags != 0 {
		t.Errorf("tag only used by the deleted job still exists")
	}

	result, err := SearchForJobs(ctx, &JobSearchParams{Limit: 10})
	if err != nil {
		t.Fatalf("SearchForJobs: %v", err)
	}

	var searched []string
	for _, job := range result.Jobs {
		searched = append(searched, job.Title)
	}
	slices.Sort(searched)

	if want := []string{"Fresh", "Undated"}; !slices.Equal(searched, want) {
		t.Errorf("search results = %v, want %v (kept-but-expired jobs hidden)", searched, want)
	}
}

func TestWriteJobsToDatabaseUpsert(t *testing.T) {
	newTestDB(t)
	posted := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	const url = "https://example.com/jobs/upsert"

	first := []jobs.Job{
		{Title: "Duplicate In Batch", Company: "Acme", URL: url, PostedAt: posted},
		{Title: "Engineer", Company: "Acme", URL: url, PostedAt: posted, Tags: []string{"go", "sql"}},
	}

	if err := WriteJobsToDatabase(first); err != nil {
		t.Fatalf("first write: %v", err)
	}

	rescraped := jobs.Job{Title: "Senior Engineer", Company: "Acme", URL: url, Tags: []string{"sql", "rust"}}

	if err := WriteJobsToDatabase([]jobs.Job{rescraped}); err != nil {
		t.Fatalf("second write: %v", err)
	}

	got, err := GetJobByID(context.Background(), jobIDByURL(t, url), JobDetailParams{})
	if err != nil {
		t.Fatalf("GetJobByID: %v", err)
	}

	if got.Title != "Senior Engineer" {
		t.Errorf("Title = %q, want the re-scraped title", got.Title)
	}

	if !got.PostedAt.Equal(posted) {
		t.Errorf("PostedAt = %v, want %v kept when the re-scrape has no date", got.PostedAt, posted)
	}

	tags := slices.Clone(got.Tags)
	slices.Sort(tags)

	if want := []string{"rust", "sql"}; !slices.Equal(tags, want) {
		t.Errorf("Tags = %v, want %v", tags, want)
	}
}

func TestJobEmbeddingText(t *testing.T) {
	if got := jobEmbeddingText("Engineer", "Short"); got != "Engineer\n\nShort" {
		t.Errorf("short text = %q, want title and description unchanged", got)
	}

	long := jobEmbeddingText("Engineer", strings.Repeat("é", maxEmbeddingChars*2))

	if n := len([]rune(long)); n != maxEmbeddingChars {
		t.Errorf("long text = %d runes, want %d", n, maxEmbeddingChars)
	}

	if !strings.HasPrefix(long, "Engineer\n\n") {
		t.Errorf("long text should start with the title")
	}
}
