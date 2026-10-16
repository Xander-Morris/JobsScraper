package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"main/database"
	"main/jobs"
	"main/schedule"
	"slices"
	"sync"
	"time"
)

// embedJobsTimeout caps how long one cycle spends embedding new jobs before
// giving up and letting the next cycle pick it back up.
const embedJobsTimeout = 10 * time.Minute

// dropExpiredJobs skips postings already past retention, so they aren't written and embedded only to be deleted.
func dropExpiredJobs(fetched []jobs.Job, cutoff time.Time) []jobs.Job {
	kept := make([]jobs.Job, 0, len(fetched))

	for _, job := range fetched {
		if job.PostedAt.IsZero() || !job.PostedAt.Before(cutoff) {
			kept = append(kept, job)
		}
	}

	return kept
}

// capNewestJobs keeps the newest max jobs, undated ones last, so a big fetch can't overfill the database.
func capNewestJobs(fetched []jobs.Job, max int) []jobs.Job {
	if len(fetched) <= max {
		return fetched
	}

	slices.SortStableFunc(fetched, func(a, b jobs.Job) int {
		if a.PostedAt.IsZero() != b.PostedAt.IsZero() {
			if a.PostedAt.IsZero() {
				return 1
			}
			return -1
		}
		return b.PostedAt.Compare(a.PostedAt)
	})

	return fetched[:max]
}

func deleteExpiredJobs(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	if deleted, err := database.DeleteExpiredJobs(ctx); err != nil {
		slog.Error("scraper: delete expired jobs", "error", err)
	} else {
		slog.Info("scraper: deleted expired jobs", "count", deleted)
	}
}

// dedupeKey keeps one company's same-titled openings in different cities or months apart.
type dedupeKey struct {
	company, title, location string
	year                     int
	month                    time.Month
}

func keyFor(job jobs.Job) dedupeKey {
	return dedupeKey{job.Company, job.Title, job.Location, job.PostedAt.Year(), job.PostedAt.Month()}
}

func fetchAll(ctx context.Context, sources []jobs.JobSource) []jobs.Job {
	var wg sync.WaitGroup
	ch := make(chan []jobs.Job, len(sources))

	for _, source := range sources {
		wg.Go(func() {
			sourceName := fmt.Sprintf("%T", source)

			defer func() {
				if r := recover(); r != nil {
					slog.Error("scraper: panic fetching jobs", "source", sourceName, "panic", r)
				}
			}()

			sourceJobs, err := source.FetchJobs(ctx)

			if err != nil {
				slog.Error("scraper: fetch failed", "source", sourceName, "error", err)
				return
			}

			slog.Info("scraper: fetched jobs", "source", sourceName, "count", len(sourceJobs))
			ch <- sourceJobs
		})
	}

	wg.Wait()
	close(ch)

	var fetchedJobs []jobs.Job
	seen := make(map[dedupeKey]bool)

	for sourceJobs := range ch {
		for _, job := range sourceJobs {
			if key := keyFor(job); !seen[key] {
				seen[key] = true
				fetchedJobs = append(fetchedJobs, job)
			}
		}
	}

	return fetchedJobs
}

func runScraper(ctx context.Context, sources []jobs.JobSource) {
	slog.Info("scraper: starting fetch cycle")

	fetchedJobs := fetchAll(ctx, sources)

	if ctx.Err() != nil {
		slog.Info("scraper: cancelled before write")
		return
	}

	fetchedCount := len(fetchedJobs)
	fetchedJobs = dropExpiredJobs(fetchedJobs, time.Now().Add(-database.JobMaxAge))
	unexpiredCount := len(fetchedJobs)
	fetchedJobs = capNewestJobs(fetchedJobs, database.MaxJobs)
	slog.Info("scraper: writing jobs", "count", len(fetchedJobs),
		"skipped_expired", fetchedCount-unexpiredCount, "skipped_over_cap", unexpiredCount-len(fetchedJobs))

	// Prune first so space frees up even if the write fails.
	deleteExpiredJobs(ctx)

	if err := database.WriteJobsToDatabase(ctx, fetchedJobs); err != nil {
		slog.Error("scraper: write jobs to database", "error", err)
		return
	}

	deleteExpiredJobs(ctx)

	embedCtx, cancel := context.WithTimeout(ctx, embedJobsTimeout)
	defer cancel()

	if err := database.EmbedPendingJobs(embedCtx); err != nil {
		slog.Error("scraper: embed pending jobs", "error", err)
	}
}

// userAgent identifies the scraper to job boards, with a link to who runs it.
const userAgent = "JobsScraper/1.0 (+https://github.com/Xander-Morris/JobsScraper)"

func allSources() []jobs.JobSource {
	return []jobs.JobSource{
		jobs.NewRemoteOK(userAgent),
		jobs.NewRemotive(userAgent),
		jobs.NewArbeitnow(userAgent),
		jobs.NewJobicy(userAgent),
		jobs.NewHimalayas(userAgent),
		jobs.NewWeWorkRemotely(userAgent),
		jobs.NewGreenhouse(userAgent),
		jobs.NewLever(userAgent),
		jobs.NewAshby(userAgent),
	}
}

const jobName = "scrape"

const scrapeInterval = 10 * time.Minute

// RunScrapeCycle runs one fetch-all-sources-and-write pass, for cmd/cron.
func RunScrapeCycle(ctx context.Context) {
	sources := allSources()

	schedule.Once(ctx, jobName, func(ctx context.Context) { runScraper(ctx, sources) })
}

// StartScrapingJob fetches immediately, then every scrapeInterval, until ctx is
// cancelled. Run it in its own goroutine.
func StartScrapingJob(ctx context.Context) {
	sources := allSources()

	schedule.Every(ctx, jobName, scrapeInterval, func(ctx context.Context) { runScraper(ctx, sources) })
}
