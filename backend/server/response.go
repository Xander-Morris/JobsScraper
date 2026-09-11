package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
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
