// Command cron runs one scrape or digest pass and exits, for .github/workflows/cron.yml.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"main/coldstart"
	"main/database"
	"main/digest"
	"main/scraper"
)

func main() {
	job := flag.String("job", "", "scrape or digest")
	flag.Parse()

	jobs := map[string]func(context.Context){
		"scrape": scraper.RunScrapeCycle,
		"digest": digest.RunCycle,
	}

	run, ok := jobs[*job]
	if !ok {
		fmt.Fprintln(os.Stderr, "usage: cron -job=scrape|digest")
		os.Exit(2)
	}

	coldstart.Ensure("DATABASE_CONNECTION", "SECRET_KEY")
	defer database.CloseDb()

	// A cancelled workflow sends SIGTERM; stop the run instead of being killed mid-write.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	run(ctx)
}
