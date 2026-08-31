package server

import (
	"net/http"
	"time"
)

func New(addr string) *http.Server {
	mux := http.NewServeMux()
	registerRoutes(mux)
	limit(mux)

	return &http.Server{
		Addr:         addr,
		Handler:      withRecovery(withLogging(withCORS(mux))),
		ReadTimeout:  time.Second * 15,
		WriteTimeout: time.Second * 30,
		IdleTimeout:  time.Second * 60,
	}
}
