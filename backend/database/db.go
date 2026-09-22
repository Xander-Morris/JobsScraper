package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"log/slog"
	"main/utils"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var cachedDb *sql.DB = nil
var mu sync.Mutex

// Connect opens and verifies the shared pool. Entrypoints call it at startup so
// a bad connection string fails there, where the process can report it and
// exit, instead of from whichever query happens to run first.
func Connect(ctx context.Context) error {
	mu.Lock()
	defer mu.Unlock()

	if cachedDb != nil {
		return nil
	}

	pool, err := openPool()

	if err != nil {
		return err
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.PingContext(pingCtx); err != nil {
		pool.Close()
		return err
	}

	slog.Info("connected to database")
	cachedDb = pool

	return nil
}

// db returns the shared pool, opening it if Connect hasn't run. Callers don't
// get an error path: sql.Open is lazy, so a database that's down or misconfigured
// surfaces from the query itself, with the message that query needs anyway.
func db() *sql.DB {
	mu.Lock()
	defer mu.Unlock()

	if cachedDb == nil {
		pool, err := openPool()

		if err != nil {
			// Only an unparseable connection string reaches this. Hand back a
			// pool that fails every query with that error rather than a nil
			// one that panics on first use.
			slog.Error("open database pool", "error", err)
			pool = sql.OpenDB(failingConnector{err: err})
		}

		cachedDb = pool
	}

	return cachedDb
}

func openPool() (*sql.DB, error) {
	pool, err := sql.Open("pgx", utils.GetEnv()["DATABASE_CONNECTION"])

	if err != nil {
		return nil, err
	}

	pool.SetMaxOpenConns(5)
	pool.SetMaxIdleConns(5)
	pool.SetConnMaxLifetime(5 * time.Minute)

	return pool, nil
}

// failingConnector backs a pool whose every query reports why the real one
// could not be opened.
type failingConnector struct {
	err error
}

func (c failingConnector) Connect(context.Context) (driver.Conn, error) { return nil, c.err }

func (c failingConnector) Driver() driver.Driver { return nil }

// DB exposes the shared pool for tests in other packages that set up rows
// directly. Production code goes through this package's own functions.
func DB() *sql.DB {
	return db()
}

// Ping reports whether the database is reachable, for the health endpoint.
func Ping(ctx context.Context) error {
	return db().PingContext(ctx)
}

func CloseDb() {
	mu.Lock()
	defer mu.Unlock()

	if cachedDb == nil {
		return
	}

	cachedDb.Close()
}

func SetDB(pool *sql.DB) *sql.DB {
	mu.Lock()
	defer mu.Unlock()

	prev := cachedDb
	cachedDb = pool

	return prev
}
