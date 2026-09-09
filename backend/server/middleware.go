package server

import (
	"context"
	"fmt"
	"log/slog"
	"main/utils"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const profileIDContextKey contextKey = "profileID"

// parseProfileIDFromToken validates a bearer token and extracts its profileID
// claim. ok is false for any invalid/expired/malformed token.
func parseProfileIDFromToken(tokenString string) (profileID int64, ok bool) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}

		return []byte(utils.GetEnv()["SECRET_KEY"]), nil
	})

	if err != nil || !token.Valid {
		return 0, false
	}

	claims, ok := token.Claims.(jwt.MapClaims)

	if !ok {
		return 0, false
	}

	id, ok := claims["profileID"].(float64)

	if !ok {
		return 0, false
	}

	return int64(id), true
}

func withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenString, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")

		if !ok || tokenString == "" {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}

		profileID, ok := parseProfileIDFromToken(tokenString)

		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		ctx := context.WithValue(r.Context(), profileIDContextKey, profileID)
		next(w, r.WithContext(ctx))
	}
}

// withOptionalAuth attaches profileID to the request context when a valid bearer
// token is present, but never rejects the request. For endpoints (like job
// search) that behave sensibly both for anonymous and authenticated callers.
func withOptionalAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if tokenString, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok && tokenString != "" {
			if profileID, ok := parseProfileIDFromToken(tokenString); ok {
				r = r.WithContext(context.WithValue(r.Context(), profileIDContextKey, profileID))
			}
		}

		next(w, r)
	}
}

func profileIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(profileIDContextKey).(int64)

	return id, ok
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(status int) {
	rec.status = status
	rec.ResponseWriter.WriteHeader(status)
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		slog.Info("http request", "method", r.Method, "path", r.URL.Path, "status", rec.status, "duration", time.Since(start))
	})
}

func withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic handling request", "method", r.Method, "path", r.URL.Path, "panic", err)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func allowedOrigin() string {
	if origin := os.Getenv("ALLOWED_ORIGIN"); origin != "" {
		return origin
	}

	return "http://localhost:5173"
}

func withCORS(next http.Handler) http.Handler {
	origin := allowedOrigin()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		if origin != "*" {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
