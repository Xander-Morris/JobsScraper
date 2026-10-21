// Command cron runs one scrape or digest pass and exits, for .github/workflows/cron.yml.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"main/coldstart"
	"main/database"
	"main/digest"
	"main/scraper"
)

// Vars so tests can run main without exiting or running a real job.
var (
	exit   = os.Exit
	ensure = coldstart.Ensure
	jobs   = map[string]func(context.Context){
		"scrape": scraper.RunScrapeCycle,
		"digest": digest.RunCycle,
	}
)

func main() {
	// A cancelled workflow sends SIGTERM; stop the run instead of being killed mid-write.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stderr)
	stop()
	exit(code)
}

func run(ctx context.Context, args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("cron", flag.ContinueOnError)
	flags.SetOutput(stderr)
	job := flags.String("job", "", "scrape or digest")

	if err := flags.Parse(args); err != nil {
		return 2
	}

	runJob, ok := jobs[*job]
	if !ok {
		fmt.Fprintln(stderr, "usage: cron -job=scrape|digest")
		return 2
	}

	if err := ensure("DATABASE_CONNECTION", "SECRET_KEY"); err != nil {
		return 1
	}
	defer database.CloseDb()

	runJob(ctx)
	return 0
}
