// Package coldstart holds the one-time process startup work (env validation,
// table creation) shared by every serverless entrypoint under backend/api/.
// main.go does this inline at the top of main() since it's a single long-lived
// process; each Vercel function is its own process that can cold-start
// independently, so the same setup needs to run per-entrypoint instead.
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

// Ensure runs startup validation and migrations exactly once per warm
// container. Safe to call at the top of every request/invocation.
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

// AuthorizedCron reports whether a request carries the CRON_SECRET this
// deployment expects. Vercel Cron Jobs send "Authorization: Bearer
// $CRON_SECRET" automatically when CRON_SECRET is set as a project env var —
// this stops anyone else from hitting a cron endpoint's public URL directly.
func AuthorizedCron(r *http.Request) bool {
	secret := os.Getenv("CRON_SECRET")

	return secret != "" && r.Header.Get("Authorization") == "Bearer "+secret
}
