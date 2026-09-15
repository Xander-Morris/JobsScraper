// Package coldstart holds the one-time startup work (env validation) for
// entrypoints other than main.go: the Vercel function under backend/api/ and
// the scheduled jobs in backend/cmd/cron. Migrations run from CI via
// backend/cmd/migrate instead, so cold starts don't wait on them.
package coldstart

import (
	"log/slog"
	"os"
	"sync"

	"main/utils"
)

var once sync.Once

// Ensure runs startup validation once per warm container.
// Safe to call at the top of every request.
func Ensure() {
	once.Do(func() {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

		if err := utils.RequireEnv("DATABASE_CONNECTION", "SECRET_KEY"); err != nil {
			slog.Error("startup: env validation failed", "error", err)
			os.Exit(1)
		}
	})
}
