package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"main/database"
	"main/jobs"
	"sync"
	"time"
)

// embedJobsTimeout caps how long one cycle spends embedding new jobs before
// giving up and letting the next cycle pick it back up.
const embedJobsTimeout = 2 * time.Minute

func runScraper(sources []jobs.JobSource) {
	slog.Info("scraper: starting fetch cycle")

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

			sourceJobs, err := source.FetchJobs()

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
	seen := make(map[string]bool)

	for sourceJobs := range ch {
		var filtered []jobs.Job

		for _, job := range sourceJobs {
			key := job.Company + job.Title + job.PostedAt.Month().String()
			exists, _ := seen[key]

			if !exists {
				filtered = append(filtered, job)
				seen[key] = true
			}
		}

		slog.Debug("scraper: filtered jobs", "count", len(filtered))
		fetchedJobs = append(fetchedJobs, filtered...)
	}

	if err := database.WriteJobsToDatabase(fetchedJobs); err != nil {
		slog.Error("scraper: write jobs to database", "error", err)
		return
	}

	embedCtx, cancel := context.WithTimeout(context.Background(), embedJobsTimeout)
	defer cancel()

	if err := database.EmbedPendingJobs(embedCtx); err != nil {
		slog.Error("scraper: embed pending jobs", "error", err)
	}
}

func runScraperSafely(sources []jobs.JobSource) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("scraper: panic during scrape cycle", "panic", r)
		}
	}()

	runScraper(sources)
}

func allSources() []jobs.JobSource {
	botAgent := "MyCustomScraperBot/1.0"

	return []jobs.JobSource{
		jobs.NewRemoteOK(botAgent),
		jobs.NewRemotive(botAgent),
		jobs.NewArbeitnow(botAgent),
		jobs.NewJobicy(botAgent),
		jobs.NewHimalayas(botAgent),
		jobs.NewWeWorkRemotely(botAgent),
	}
}

// RunScrapeCycle runs one fetch-all-sources-and-write pass. For a scheduler
// with no long-lived process of its own (a Vercel Cron Job hitting an
// endpoint per invocation), instead of StartScrapingJob's ticker loop.
func RunScrapeCycle() {
	runScraperSafely(allSources())
}

// StartScrapingJob fetches immediately, then every 10 minutes, until ctx is
// cancelled. Run it in its own goroutine. On cancel it finishes any in-flight
// cycle first instead of abandoning a write mid-flight.
func StartScrapingJob(ctx context.Context) {
	sources := allSources()

	runScraperSafely(sources)
	ticker := time.NewTicker(time.Minute * 10)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runScraperSafely(sources)
		}
	}
}
