// Command cron runs one scrape or digest pass and exits, for .github/workflows/cron.yml.
package main

import (
	"flag"
	"fmt"
	"os"

	"main/coldstart"
	"main/database"
	"main/digest"
	"main/scraper"
)

func main() {
	job := flag.String("job", "", "scrape or digest")
	flag.Parse()

	jobs := map[string]func(){
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

	run()
}
