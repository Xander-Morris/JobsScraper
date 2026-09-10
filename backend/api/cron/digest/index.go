// Package handler is a Vercel Cron Job entrypoint: one digest pass per
// invocation. Replaces server.StartDigestScheduler's ticker loop, which has
// nowhere to live in serverless. See vercel.json's "crons" entry for the schedule.
package handler

import (
	"net/http"

	"main/coldstart"
	"main/server"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	if !coldstart.AuthorizedCron(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	coldstart.Ensure()
	server.RunDigestCycle()

	w.WriteHeader(http.StatusOK)
}
