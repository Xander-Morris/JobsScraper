package server

import (
	"net/http"
	"time"
)

func New(addr string) *http.Server {
	mux := http.NewServeMux()
	registerRoutes(mux)

	return &http.Server{
		Addr:         addr,
		Handler:      withRecovery(withLogging(withCORS(mux))),
		ReadTimeout:  time.Second * 5,
		WriteTimeout: time.Second * 5,
	}
}
