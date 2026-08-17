package scraper

import (
	"fmt"
	"main/database"
	"main/jobs"
	"sync"
	"time"
)

func runScraper(sources []jobs.JobSource) {
	fmt.Printf("\n--- Starting fetch cycle at %v ---\n", time.Now().Format(time.RFC3339))

	var wg sync.WaitGroup
	ch := make(chan []jobs.Job, len(sources))

	for _, source := range sources {
		wg.Go(func() {
			sourceJobs, err := source.FetchJobs()

			if err != nil {
				fmt.Printf("Failed to fetch jobs from %T: %v\n", source, err)
				return
			}

			fmt.Printf("Fetched %d jobs from %T\n", len(sourceJobs), source)
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

		fmt.Printf("Length after filter: %d\n", len(filtered))
		fetchedJobs = append(fetchedJobs, filtered...)
	}

	if err := database.WriteJobsToDatabase(fetchedJobs); err != nil {
		fmt.Println(err)
	}
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

	runScraper(sources)
	ticker := time.NewTicker(12 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		runScraper(sources)
	}
}
