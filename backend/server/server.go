package server

import (
	"net/http"
	"time"
)

// Handler builds the full request-handling chain shared by both the
// self-hosted binary (New, below) and the Vercel serverless entrypoint
// (backend/api/index.go).
func Handler() http.Handler {
	mux := http.NewServeMux()
	registerRoutes(mux)

	// CORS stays outside the limiter so a 429 still carries the headers the
	// browser needs to surface it, and preflights are not counted against it.
	return withRecovery(withLogging(withCORS(limit(mux))))
}

func New(addr string) *http.Server {
	return &http.Server{
		Addr:    addr,
		Handler: Handler(),
		// Deliberately tight. Resume upload and LLM generation are the two
		// endpoints that legitimately need longer; they extend their own
		// deadline via http.ResponseController instead of loosening this for
		// everyone else.
		ReadTimeout:  time.Second * 15,
		WriteTimeout: time.Second * 30,
		IdleTimeout:  time.Second * 60,
	}
}
