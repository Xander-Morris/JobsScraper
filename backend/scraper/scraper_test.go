package scraper

import (
	"main/jobs"
	"slices"
	"testing"
	"time"
)

func TestDropExpiredJobs(t *testing.T) {
	cutoff := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	fetched := []jobs.Job{
		{Title: "Recent", PostedAt: cutoff.Add(time.Hour)},
		{Title: "Old", PostedAt: cutoff.Add(-time.Hour)},
		{Title: "Undated"},
	}

	var got []string
	for _, job := range dropExpiredJobs(fetched, cutoff) {
		got = append(got, job.Title)
	}

	if want := []string{"Recent", "Undated"}; !slices.Equal(got, want) {
		t.Errorf("kept = %v, want %v", got, want)
	}
}
