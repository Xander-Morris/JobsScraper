package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"main/database"
)

// maxJSONBodySize caps request bodies decoded as JSON. Nothing we decode
// (profile fields, education/work-experience entries) gets anywhere close;
// this just stops an oversized body from tying up memory before the rate
// limiter catches the repeat offender.
const maxJSONBodySize = 1 << 20 // 1 MiB

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodySize)
	return json.NewDecoder(r.Body).Decode(v)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("write json response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// statusResponse is the {"status": ...} body the mutating endpoints answer with.
func statusResponse(status string) map[string]string {
	return map[string]string{"status": status}
}

// writeDBError turns a database error into the matching response: a validation
// failure the caller can fix becomes a 400 carrying its message, a missing row
// becomes a 404 with notFound, and anything else is a server fault, logged
// under logLabel and answered with failure.
func writeDBError(w http.ResponseWriter, logLabel string, err error, notFound, failure string) {
	switch {
	case errors.Is(err, database.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, sql.ErrNoRows):
		writeError(w, http.StatusNotFound, notFound)
	default:
		slog.Error(logLabel, "error", err)
		writeError(w, http.StatusInternalServerError, failure)
	}
}

// pathID parses a path wildcard as an int64, writing a 400 naming the id
// ("job", "resume", ...) and returning false when it isn't one.
func pathID(w http.ResponseWriter, r *http.Request, name, label string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)

	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid "+label+" id")
		return 0, false
	}

	return id, true
}
