package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"log/slog"
	"main/utils"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

var cachedDb *sql.DB = nil

// cachedPool backs cachedDb when openPool built it; nil for a DB set by SetDB.
var cachedPool *pgxpool.Pool
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

	db, pool := openPool()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return err
	}

	slog.Info("connected to database")
	cachedDb, cachedPool = db, pool

	return nil
}

// db returns the shared pool, opening it if Connect hasn't run. Callers don't
// get an error path: the pool is lazy, so a database that's down or misconfigured
// surfaces from the query itself, with the message that query needs anyway.
func db() *sql.DB {
	mu.Lock()
	defer mu.Unlock()

	if cachedDb == nil {
		cachedDb, cachedPool = openPool()
	}

	return cachedDb
}

// openPool backs a *sql.DB with a pgxpool, so jet, migrate and faultdb keep working.
// The pool is nil when the connection string doesn't parse.
func openPool() (*sql.DB, *pgxpool.Pool) {
	dsn := utils.Getenv("DATABASE_CONNECTION")
	cfg, err := pgxpool.ParseConfig(dsn)

	if err != nil {
		// Lazy, so the parse error surfaces from the first query.
		fallback, _ := sql.Open("pgx", dsn)
		return fallback, nil
	}

	cfg.MaxConns = 5
	cfg.MaxConnLifetime = 5 * time.Minute

	// Doesn't connect yet; only fails for a config ParseConfig didn't build.
	pool, _ := pgxpool.NewWithConfig(context.Background(), cfg)

	// pgxpool manages idle conns, as stdlib.OpenDBFromPool does.
	db := sql.OpenDB(poolConnector{stdlib.GetPoolConnector(pool), pool})
	db.SetMaxIdleConns(0)

	return db, pool
}

// poolConnector closes the pgxpool when the *sql.DB closes.
type poolConnector struct {
	driver.Connector
	pool *pgxpool.Pool
}

func (c poolConnector) Close() error {
	c.pool.Close()
	return nil
}

// migrationDB opens the shared pool and returns a second handle on it that
// golang-migrate can close, releasing the conn it holds without closing the
// pool; nil when there's no pool, as for a DB set by SetDB.
func migrationDB() *sql.DB {
	db()

	mu.Lock()
	defer mu.Unlock()

	if cachedPool == nil {
		return nil
	}

	return stdlib.OpenDBFromPool(cachedPool)
}

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
	cachedDb, cachedPool = nil, nil
}

func SetDB(pool *sql.DB) *sql.DB {
	mu.Lock()
	defer mu.Unlock()

	prev := cachedDb
	cachedDb, cachedPool = pool, nil

	return prev
}
