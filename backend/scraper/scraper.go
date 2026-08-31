package scraper

import (
	"fmt"
	"log/slog"
	"main/database"
	"main/jobs"
	"sync"
	"time"
)

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

func StartScrapingJob() {
	botAgent := "MyCustomScraperBot/1.0"

	sources := []jobs.JobSource{
		jobs.NewRemoteOK(botAgent),
		jobs.NewRemotive(botAgent),
		jobs.NewArbeitnow(botAgent),
		jobs.NewJobicy(botAgent),
		jobs.NewHimalayas(botAgent),
		jobs.NewWeWorkRemotely(botAgent),
	}

	runScraperSafely(sources)
	ticker := time.NewTicker(time.Minute * 10)
	defer ticker.Stop()

	for range ticker.C {
		runScraperSafely(sources)
	}
}
