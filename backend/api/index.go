// Package handler is the Vercel serverless entrypoint that serves the whole
// API through the same handler chain server.New uses for the self-hosted
// binary.
package handler

import (
	"net/http"
	"sync"

	"main/coldstart"
	"main/server"
)

var once sync.Once
var httpHandler http.Handler

// ensure is a var so tests can fail startup.
var ensure = coldstart.Ensure

func Handler(w http.ResponseWriter, r *http.Request) {
	if err := ensure("DATABASE_CONNECTION", "SECRET_KEY"); err != nil {
		http.Error(w, `{"error":"server is misconfigured"}`, http.StatusInternalServerError)
		return
	}

	once.Do(func() { httpHandler = server.Handler() })

	httpHandler.ServeHTTP(w, r)
}
