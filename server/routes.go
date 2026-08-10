package server

import "net/http"

func registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("GET /api/jobs", handleSearchJobs)
	mux.HandleFunc("GET /api/jobs/{id}", handleGetJob)
	mux.HandleFunc("GET /api/tags", handleGetTags)
}
