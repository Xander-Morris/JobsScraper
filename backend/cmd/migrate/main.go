// Command migrate applies pending migrations and exits. CI runs it on push to main.
package main

import (
	"log/slog"
	"os"

	"main/coldstart"
	"main/database"
)

func main() {
	coldstart.Ensure("DATABASE_CONNECTION")

	err := database.CreateTables()
	database.CloseDb()

	if err != nil {
		slog.Error("migrate: apply migrations failed", "error", err)
		os.Exit(1)
	}

	slog.Info("migrate: up to date")
}
