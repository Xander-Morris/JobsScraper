// Package coldstart holds the one-time startup work (env validation, table
// creation) shared by every serverless entrypoint under backend/api/. main.go
// does this inline since it's one long-lived process. Each Vercel function is
// its own process and cold-starts independently, so it needs this per-entrypoint.
package coldstart

import (
	"log/slog"
	"net/http"
	"os"
	"sync"

	"main/database"
	"main/utils"
)

var once sync.Once

// Ensure runs startup validation and migrations once per warm container.
// Safe to call at the top of every request.
func Ensure() {
	once.Do(func() {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

		if err := utils.RequireEnv("DATABASE_CONNECTION", "SECRET_KEY"); err != nil {
			slog.Error("startup: env validation failed", "error", err)
			os.Exit(1)
		}

		if err := database.CreateTables(); err != nil {
			slog.Error("startup: create tables failed", "error", err)
			os.Exit(1)
		}
	})
}

// AuthorizedCron checks the request carries our CRON_SECRET. Vercel sends
// "Authorization: Bearer $CRON_SECRET" automatically once that env var is
// set, which keeps randoms from hitting the cron endpoint directly.
func AuthorizedCron(r *http.Request) bool {
	secret := os.Getenv("CRON_SECRET")

	return secret != "" && r.Header.Get("Authorization") == "Bearer "+secret
}
