// Package handler is the Vercel serverless entrypoint that serves the whole
// API through the same handler chain server.New uses for the self-hosted
// binary. See vercel.json's rewrite, which sends every /api/* request here.
package handler

import (
	"net/http"
	"sync"

	"main/coldstart"
	"main/server"
)

var once sync.Once
var httpHandler http.Handler

func Handler(w http.ResponseWriter, r *http.Request) {
	coldstart.Ensure()

	once.Do(func() { httpHandler = server.Handler() })

	httpHandler.ServeHTTP(w, r)
}
