package server

import (
	"net/http"
	"time"
)

func Handler() http.Handler {
	mux := http.NewServeMux()
	registerRoutes(mux)

	return withRecovery(withLogging(withCORS(limit(mux))))
}

func New(addr string) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      Handler(),
		ReadTimeout:  time.Second * 15,
		WriteTimeout: time.Second * 30,
		IdleTimeout:  time.Second * 60,
	}
}
