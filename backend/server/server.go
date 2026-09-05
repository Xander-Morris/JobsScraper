package server

import (
	"net/http"
	"time"
)

func New(addr string) *http.Server {
	mux := http.NewServeMux()
	registerRoutes(mux)

	// CORS stays outside the limiter so a 429 still carries the headers the
	// browser needs to surface it, and preflights are not counted against it.
	return &http.Server{
		Addr:         addr,
		Handler:      withRecovery(withLogging(withCORS(limit(mux)))),
		// Deliberately tight. The two endpoints that legitimately need longer —
		// resume upload (large body) and LLM generation (slow response) — extend
		// their own deadline via http.ResponseController instead of loosening
		// the limit for all 30-odd handlers.
		ReadTimeout:  time.Second * 15,
		WriteTimeout: time.Second * 30,
		IdleTimeout:  time.Second * 60,
	}
}
