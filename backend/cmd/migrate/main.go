// Command migrate applies pending migrations and exits. CI runs it on push to main.
package main

import (
	"log/slog"
	"os"

	"main/coldstart"
	"main/database"
)

// Vars so tests can run main without exiting or touching a real database.
var (
	exit         = os.Exit
	ensure       = coldstart.Ensure
	createTables = database.CreateTables
)

func main() {
	exit(run())
}

func run() int {
	if err := ensure("DATABASE_CONNECTION"); err != nil {
		return 1
	}

	err := createTables()
	database.CloseDb()

	if err != nil {
		slog.Error("migrate: apply migrations failed", "error", err)
		return 1
	}

	slog.Info("migrate: up to date")
	return 0
}
