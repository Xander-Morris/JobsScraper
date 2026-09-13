// Package coldstart holds the one-time startup work (env validation, table
// creation) for entrypoints other than main.go: the Vercel function under
// backend/api/ and the scheduled jobs in backend/cmd/cron.
package coldstart

import (
	"log/slog"
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
