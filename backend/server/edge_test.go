package server

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/time/rate"
)

func TestParseJobSearchParamsMaxSalaryAndRelevance(t *testing.T) {
	params, err := parseJobSearchParams(httptest.NewRequest("GET", "/api/jobs?max_salary=150000&sort=relevance", nil))
	if err != nil {
		t.Fatal(err)
	}
	if params.MaxSalary != 150000 || params.Sort != "relevance" {
		t.Errorf("params = %+v", params)
	}
}

func TestParseProfileIDRejectsOddTokens(t *testing.T) {
	t.Setenv("SECRET_KEY", "secret")

	// An RS256 header is refused before any key is used.
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	body := base64.RawURLEncoding.EncodeToString([]byte(`{"profileID":1}`))
	if _, ok := parseProfileIDFromToken(header + "." + body + ".sig"); ok {
		t.Error("RS256 token accepted")
	}

	noID, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "x"}).SignedString([]byte("secret"))
	if _, ok := parseProfileIDFromToken(noID); ok {
		t.Error("token without profileID accepted")
	}
}

func TestLimitRejectsUnresolvableClient(t *testing.T) {
	handler := limit(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("handler ran") }))
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "no-port"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestIPLimiterForgetsIdleVisitors(t *testing.T) {
	prev := limiterCleanupInterval
	limiterCleanupInterval = time.Millisecond
	t.Cleanup(func() { limiterCleanupInterval = prev })

	l := newIPLimiter(rate.Inf, 1)
	l.allow("1.1.1.1")
	l.allow("2.2.2.2")
	l.mu.Lock()
	l.visitors["1.1.1.1"].lastSeen = time.Now().Add(-time.Hour)
	l.mu.Unlock()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		l.mu.Lock()
		_, stale := l.visitors["1.1.1.1"]
		_, fresh := l.visitors["2.2.2.2"]
		l.mu.Unlock()
		if !stale && fresh {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Error("idle visitor was never removed")
}

func TestSharedRedisFromEnv(t *testing.T) {
	prevClient := redisClient
	t.Cleanup(func() { redisOnce, redisClient = sync.Once{}, prevClient })

	for url, wantClient := range map[string]bool{"": false, "not a url": false, "redis://localhost:6379/0": true} {
		redisOnce, redisClient = sync.Once{}, nil
		t.Setenv("REDIS_URL", url)

		if got := sharedRedis(); (got != nil) != wantClient {
			t.Errorf("REDIS_URL=%q: client = %v, want client %v", url, got, wantClient)
		}
	}
}

func TestWriteJSONUnencodable(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, make(chan int))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestAPIBadRequests(t *testing.T) {
	a := newAPI(t).signup("api-bad-requests@example.com")

	a.must(http.StatusBadRequest, "POST", "/api/jobs/abc/tailored-resume", nil, nil)
	a.must(http.StatusBadRequest, "PUT", "/api/jobs/abc/tailored-resume", nil, nil)
	a.must(http.StatusBadRequest, "PUT", "/api/jobs/1/tailored-resume", "{", nil)
	a.must(http.StatusBadRequest, "POST", "/api/profile/password-reset/confirm", "{", nil)
	a.must(http.StatusBadRequest, "POST", "/api/profile/password-reset/confirm", map[string]string{"password": "securepassword123"}, nil)
}

func TestAPIPasswordResetEmailFailureStillAnswers(t *testing.T) {
	a := newAPI(t).signup("api-reset-down@example.com")
	t.Setenv("RESEND_API_KEY", "re_test")
	t.Setenv("RESEND_API_URL", "http://127.0.0.1:1")

	a.must(http.StatusAccepted, "POST", "/api/profile/password-reset", map[string]string{"email": "api-reset-down@example.com"}, nil)
}

func TestAPIExtractionWithoutEmbeddings(t *testing.T) {
	a := newAPI(t).signup("api-no-embed@example.com")
	fakeLLM(t, static(extractionJSON))
	t.Setenv("JINA_API_KEY", "")

	status, raw := a.upload("POST", "/api/profile/resumes", "ada.docx", testDocx(t, "Ada Lovelace"), nil)
	if status != http.StatusCreated {
		t.Fatalf("upload = %d: %s", status, raw)
	}
	id := a.profile().Resumes[0].ID

	var trigger map[string]string
	a.must(http.StatusOK, "POST", fmt.Sprintf("/api/profile/resumes/%d/extraction", id), nil, &trigger)
	if trigger["status"] != "completed" {
		t.Errorf("trigger = %v, want completed despite the embedding failure", trigger)
	}
}
