package scraper

import (
	"context"
	"slices"
	"testing"
	"time"

	"main/database"
	"main/internal/faultdb"
	"main/jobs"
	"main/utils"
)

func TestCapNewestJobsPutsUndatedLast(t *testing.T) {
	now := time.Now()
	got := capNewestJobs([]jobs.Job{
		{Title: "undated1"}, {Title: "old", PostedAt: now.Add(-time.Hour)}, {Title: "undated2"}, {Title: "new", PostedAt: now},
	}, 3)

	titles := []string{got[0].Title, got[1].Title, got[2].Title}
	if !slices.Equal(titles, []string{"new", "old", "undated1"}) {
		t.Errorf("titles = %v", titles)
	}
}

func TestSchedulersStopWhenCancelled(t *testing.T) {
	newTestDB(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	// Every source fails at once on a dead context, so neither touches the network or database.
	RunScrapeCycle(ctx)
	StartScrapingJob(ctx)
}

func TestRunScraperSurvivesDatabaseFaults(t *testing.T) {
	newTestDB(t)
	t.Setenv("JINA_API_KEY", "")

	pool, inj, err := faultdb.Open(utils.Getenv("TEST_DATABASE_CONNECTION"))
	if err != nil {
		t.Fatal(err)
	}
	prev := database.SetDB(pool)
	t.Cleanup(func() { database.SetDB(prev); pool.Close() })

	sources := []jobs.JobSource{fakeSource{jobs: []jobs.Job{
		{Title: "Fresh", Company: "Acme", URL: "https://example.com/fresh", PostedAt: time.Now(), Description: "Go"},
	}}}

	// Write and prune failures are logged, never escape the cycle.
	faultdb.SweepEach(t, inj, nil, func() { runScraper(t.Context(), sources) }, func(*testing.T) {})
}
