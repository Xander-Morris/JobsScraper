// Package handler is a Vercel Cron Job entrypoint: one scrape cycle per
// invocation. Replaces scraper.StartScrapingJob's ticker loop, which has
// nowhere to live in serverless. See vercel.json's "crons" entry for the schedule.
package handler

import (
	"net/http"

	"main/coldstart"
	"main/scraper"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	if !coldstart.AuthorizedCron(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	coldstart.Ensure()
	scraper.RunScrapeCycle()

	w.WriteHeader(http.StatusOK)
}
