package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// maxJSONBodySize caps request bodies decoded as JSON — none of these payloads
// (profile fields, education/work-experience entries) legitimately approach this,
// so it's purely a guard against an oversized body tying up memory before the
// per-IP rate limiter would otherwise catch a repeat offender.
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
