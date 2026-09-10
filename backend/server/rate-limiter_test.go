package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// The limiter was built but never wired into the handler chain for a while,
// so every limit silently did nothing. Testing through New() instead of
// calling limit() directly is what would've caught that.
func TestNewEnforcesGlobalRateLimit(t *testing.T) {
	original := globalLimiter
	globalLimiter = newIPLimiter(rate.Every(time.Hour), 2)
	t.Cleanup(func() { globalLimiter = original })

	handler := New(":0").Handler

	send := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/unrouted", nil)
		req.RemoteAddr = "203.0.113.1:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		return rec
	}

	if got := send().Code; got == http.StatusTooManyRequests {
		t.Fatalf("first request within burst was limited")
	}

	send()

	rec := send()
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status past burst = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}

	// Without CORS headers the browser reports a network error instead of the 429.
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got == "" {
		t.Errorf("rate-limited response is missing Access-Control-Allow-Origin")
	}
}

func TestRateLimitIsPerIP(t *testing.T) {
	original := globalLimiter
	globalLimiter = newIPLimiter(rate.Every(time.Hour), 1)
	t.Cleanup(func() { globalLimiter = original })

	handler := New(":0").Handler

	send := func(remoteAddr string) int {
		req := httptest.NewRequest(http.MethodGet, "/api/unrouted", nil)
		req.RemoteAddr = remoteAddr
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		return rec.Code
	}

	send("203.0.113.2:1234")

	if got := send("203.0.113.3:1234"); got == http.StatusTooManyRequests {
		t.Errorf("a second IP was limited by the first IP's budget")
	}
}
