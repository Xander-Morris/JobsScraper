package server

import (
	"log/slog"
	"net/http"

	"main/database"
)

func handleGetTags(w http.ResponseWriter, r *http.Request) {
	res, err := database.FetchAllUniqueTags(r.Context())

	if err != nil {
		slog.Error("get tags", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load tags")
		return
	}

	// Tags only change when the scraper runs.
	w.Header().Set("Cache-Control", "public, max-age=300, stale-while-revalidate=3600")
	writeJSON(w, http.StatusOK, res)
}
