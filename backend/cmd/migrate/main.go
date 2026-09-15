// Command migrate applies pending migrations and exits. CI runs it on push to main.
package main

import (
	"log/slog"
	"os"

	"main/database"
	"main/utils"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := utils.RequireEnv("DATABASE_CONNECTION"); err != nil {
		slog.Error("migrate: env validation failed", "error", err)
		os.Exit(1)
	}

	err := database.CreateTables()
	database.CloseDb()

	if err != nil {
		slog.Error("migrate: apply migrations failed", "error", err)
		os.Exit(1)
	}

	slog.Info("migrate: up to date")
}
