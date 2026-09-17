// Package coldstart holds the one-time startup work (env validation) for
// entrypoints other than main.go.
package coldstart

import (
	"log/slog"
	"os"
	"sync"

	"main/utils"
)

var once sync.Once

// Ensure runs startup validation once per warm container.
func Ensure() {
	once.Do(func() {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

		if err := utils.RequireEnv("DATABASE_CONNECTION", "SECRET_KEY"); err != nil {
			slog.Error("startup: env validation failed", "error", err)
			os.Exit(1)
		}
	})
}
