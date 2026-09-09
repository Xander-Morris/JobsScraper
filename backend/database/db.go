package database

import (
	"context"
	"database/sql"
	"log/slog"
	"main/utils"
	"os"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var cachedDb *sql.DB = nil
var mu sync.Mutex

func GetDb() (*sql.DB, error) {
	mu.Lock()
	defer mu.Unlock()

	if cachedDb != nil {
		return cachedDb, nil
	}

	connString := utils.GetEnv()["DATABASE_CONNECTION"]
	db, err := sql.Open("pgx", connString)

	if err != nil {
		return nil, err
	}

	// Kept modest since a serverless deployment (backend/api/) can run many of
	// these pools concurrently, one per warm container, all against the same
	// Postgres connection limit, unlike the self-hosted binary, which only
	// ever has one.
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		slog.Error("failed to ping database", "error", err)
		os.Exit(1)
	}

	slog.Info("connected to database")
	cachedDb = db

	return cachedDb, nil
}

func CloseDb() {
	if cachedDb == nil {
		return
	}

	cachedDb.Close()
}

func SetDB(db *sql.DB) *sql.DB {
	mu.Lock()
	defer mu.Unlock()

	prev := cachedDb
	cachedDb = db

	return prev
}
