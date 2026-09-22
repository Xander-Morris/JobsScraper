package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestWithAuthRejectsBadTokens(t *testing.T) {
	tests := map[string]string{
		"missing":      "",
		"not bearer":   "Basic abc",
		"empty bearer": "Bearer ",
		"garbage":      "Bearer not-a-jwt",
	}

	for name, header := range tests {
		t.Run(name, func(t *testing.T) {
			called := false
			handler := withAuth(func(http.ResponseWriter, *http.Request, int64) { called = true })

			r := httptest.NewRequest("GET", "/api/profile", nil)
			if header != "" {
				r.Header.Set("Authorization", header)
			}

			rec := httptest.NewRecorder()
			handler(rec, r)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
			if called {
				t.Error("handler ran for an unauthenticated request")
			}
		})
	}
}

func TestWithAuthPassesProfileID(t *testing.T) {
	t.Setenv("SECRET_KEY", "test-secret-key")

	token, err := createToken(42)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	var got int64
	handler := withAuth(func(w http.ResponseWriter, r *http.Request, profileID int64) {
		got = profileID
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequest("GET", "/api/profile", nil)
	r.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	handler(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got != 42 {
		t.Errorf("profileID = %d, want 42", got)
	}
}

func TestWithCORSDefaultOrigin(t *testing.T) {
	os.Unsetenv("ALLOWED_ORIGIN")

	handler := withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/api/health", nil))

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "http://localhost:5173")
	}
}

func TestWithCORSConfiguredOrigin(t *testing.T) {
	t.Setenv("ALLOWED_ORIGIN", "https://example.com")

	handler := withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/api/health", nil))

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "https://example.com")
	}
}

func TestWithCORSPreflight(t *testing.T) {
	called := false
	handler := withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/api/jobs", nil))

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if called {
		t.Errorf("next handler should not be called for OPTIONS preflight")
	}
}

func TestWithRecoveryCatchesPanic(t *testing.T) {
	handler := withRecovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/api/jobs", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestWithLoggingCapturesStatus(t *testing.T) {
	handler := withLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/api/jobs", nil))

	if rec.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTeapot)
	}
}
