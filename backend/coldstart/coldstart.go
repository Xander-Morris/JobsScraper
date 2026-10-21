// Package coldstart holds the startup work every entrypoint shares: JSON
// logging, env validation, and opening the database.
package coldstart

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"main/database"
	"main/utils"
)

var (
	once      sync.Once
	ensureErr error
)

// Ensure runs startup once per process, or per warm container on serverless,
// and returns the same result on every later call. requiredEnv names the
// variables that entrypoint cannot run without. Callers treat an error as
// fatal, so misconfiguration surfaces at startup instead of as a mystery error
// on the first request.
func Ensure(requiredEnv ...string) error {
	once.Do(func() {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

		if err := utils.RequireEnv(requiredEnv...); err != nil {
			ensureErr = fmt.Errorf("env validation failed: %w", err)
			return
		}

		if err := database.Connect(context.Background()); err != nil {
			ensureErr = fmt.Errorf("database connection failed: %w", err)
		}
	})

	if ensureErr != nil {
		slog.Error("startup failed", "error", ensureErr)
	}

	return ensureErr
}
