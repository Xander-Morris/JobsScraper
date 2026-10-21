package database

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	pgx5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migrationsDir is a var so tests can point it somewhere empty.
var migrationsDir = "migrations"

// runMigrations applies every pending migration in migrations/ to db, tracked
// via golang-migrate's schema_migrations table. Safe to call every boot.
func runMigrations(db *sql.DB) error {
	source, err := iofs.New(migrationsFS, migrationsDir)

	if err != nil {
		return fmt.Errorf("load embedded migrations: %w", err)
	}

	dbDriver, err := pgx5.WithInstance(db, &pgx5.Config{})

	if err != nil {
		return fmt.Errorf("init migration driver: %w", err)
	}

	// NewWithInstance never returns an error.
	m, _ := migrate.NewWithInstance("iofs", source, "pgx5", dbDriver)

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}
