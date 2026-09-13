// Command cron runs one scrape or digest pass and exits, for .github/workflows/cron.yml.
package main

import (
	"flag"
	"fmt"
	"os"

	"main/coldstart"
	"main/database"
	"main/scraper"
	"main/server"
)

func main() {
	job := flag.String("job", "", "scrape or digest")
	flag.Parse()

	jobs := map[string]func(){
		"scrape": scraper.RunScrapeCycle,
		"digest": server.RunDigestCycle,
	}

	run, ok := jobs[*job]
	if !ok {
		fmt.Fprintln(os.Stderr, "usage: cron -job=scrape|digest")
		os.Exit(2)
	}

	coldstart.Ensure()
	defer database.CloseDb()

	run()
}
