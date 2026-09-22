package server

import (
	"log/slog"
	"net/http"
	"strconv"

	"main/database"
	"main/digest"
)

func handleUnsubscribeDigest(w http.ResponseWriter, r *http.Request) {
	profileID, err := strconv.ParseInt(r.URL.Query().Get("profile_id"), 10, 64)

	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid unsubscribe link")
		return
	}

	if !digest.ValidUnsubscribeToken(profileID, r.URL.Query().Get("token")) {
		writeError(w, http.StatusUnauthorized, "invalid unsubscribe link")
		return
	}

	if err := database.DisableEmailDigest(r.Context(), profileID); err != nil {
		slog.Error("unsubscribe digest", "profile_id", profileID, "error", err)
		writeError(w, http.StatusInternalServerError, "could not unsubscribe")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("<!DOCTYPE html><html><body><p>You've been unsubscribed from job digest emails.</p></body></html>"))
}
