package server

import (
	"main/database"
	"net/http"
)

func handleGetTags(w http.ResponseWriter, r *http.Request) {
	res, err := database.FetchAllUniqueTags(r.Context())

	if err == nil {
		// Tags only change when the scraper runs.
		w.Header().Set("Cache-Control", "public, max-age=300, stale-while-revalidate=3600")
		writeJSON(w, http.StatusOK, res)
	} else {
		writeJSON(w, http.StatusInternalServerError, res)
	}
}
