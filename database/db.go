package database

import (
	"database/sql"
	"sync"
)

var cachedDb *sql.DB = nil
var mu sync.Mutex

func GetDb() (*sql.DB, error) {
	mu.Lock()
	defer mu.Unlock()

	if cachedDb != nil {
		return cachedDb, nil
	}

	db, err := sql.Open("sqlite", databaseFileName)

	if err != nil {
		return nil, err
	}

	// Verify the connection works
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	cachedDb = db

	return cachedDb, nil
}

func CloseDb() {
	if cachedDb == nil {
		return
	}

	cachedDb.Close()
}

// SetDB overrides the cached connection (for tests, e.g. an in-memory
// sqlite db) and returns the previous one so callers can restore it.
func SetDB(db *sql.DB) *sql.DB {
	mu.Lock()
	defer mu.Unlock()

	prev := cachedDb
	cachedDb = db

	return prev
}
