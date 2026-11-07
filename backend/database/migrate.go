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

// Regenerates internal/gen from the migrations; needs a Postgres, see scripts/gen-db.sh.
//go:generate bash ../scripts/gen-db.sh

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migrationsDir is a var so tests can point it somewhere empty.
var migrationsDir = "migrations"

// runMigrations applies every pending migration in migrations/ to db, tracked
// via golang-migrate's schema_migrations table. Safe to call every boot.
// closeAfter closes db and the conn migrate holds; without it that conn is never released.
func runMigrations(db *sql.DB, closeAfter bool) error {
	if closeAfter {
		defer db.Close()
	}

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

	if closeAfter {
		defer m.Close()
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}
