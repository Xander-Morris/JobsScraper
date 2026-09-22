package server

import (
	"net/http"

	"main/database"
)

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := database.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, statusResponse("unavailable"))
		return
	}

	writeJSON(w, http.StatusOK, statusResponse("ok"))
}
