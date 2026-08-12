package server

import (
	"net/http"

	"main/database"
)

func handleHealth(w http.ResponseWriter, r *http.Request) {
	db, err := database.GetDb()

	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}

	if err := db.Ping(); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
