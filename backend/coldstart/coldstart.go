// Package coldstart holds the startup work every entrypoint shares: JSON
// logging, env validation, and opening the database.
package coldstart

import (
	"context"
	"log/slog"
	"os"
	"sync"

	"main/database"
	"main/utils"
)

var once sync.Once

// Ensure runs startup once per process, or per warm container on serverless.
// requiredEnv names the variables that entrypoint cannot run without. Failures
// are fatal here rather than deeper down, so misconfiguration surfaces at
// startup instead of as a mystery error on the first request.
func Ensure(requiredEnv ...string) {
	once.Do(func() {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

		if err := utils.RequireEnv(requiredEnv...); err != nil {
			slog.Error("startup: env validation failed", "error", err)
			os.Exit(1)
		}

		if err := database.Connect(context.Background()); err != nil {
			slog.Error("startup: database connection failed", "error", err)
			os.Exit(1)
		}
	})
}
