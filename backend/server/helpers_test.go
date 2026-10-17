package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	tests := []struct {
		name    string
		remote  string
		fwd     string
		want    string
		wantErr bool
	}{
		{"remote addr", "198.51.100.7:4242", "", "198.51.100.7", false},
		{"forwarded wins", "10.0.0.1:1", "203.0.113.9", "203.0.113.9", false},
		{"first forwarded hop", "10.0.0.1:1", " 203.0.113.9 , 10.0.0.2", "203.0.113.9", false},
		{"blank forwarded falls back", "198.51.100.7:1", " , 10.0.0.2", "198.51.100.7", false},
		{"ipv6", "[2001:db8::1]:80", "", "2001:db8::1", false},
		{"bad remote addr", "garbage", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = tt.remote
			if tt.fwd != "" {
				r.Header.Set("X-Forwarded-For", tt.fwd)
			}

			got, err := clientIP(r)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Errorf("clientIP = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

type denyAll struct{}

func (denyAll) allow(string) bool { return false }

func TestLimitWith(t *testing.T) {
	ran := false
	next := func(w http.ResponseWriter, r *http.Request) { ran = true }

	rec := httptest.NewRecorder()
	limitWith(denyAll{}, next)(rec, httptest.NewRequest("POST", "/", nil))
	if rec.Code != http.StatusTooManyRequests || ran {
		t.Errorf("denied: status %d, ran %v", rec.Code, ran)
	}

	rec = httptest.NewRecorder()
	limitWith(allowAll{}, next)(rec, httptest.NewRequest("POST", "/", nil))
	if !ran {
		t.Error("allowed request did not run")
	}

	bad := httptest.NewRequest("POST", "/", nil)
	bad.RemoteAddr = "garbage"
	rec = httptest.NewRecorder()
	limitWith(allowAll{}, next)(rec, bad)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("unresolvable ip status = %d", rec.Code)
	}
}

func TestLimitAuthAndLLMUseTheirLimiters(t *testing.T) {
	prevAuth, prevLLM := authLimiter, llmLimiter
	authLimiter, llmLimiter = denyAll{}, denyAll{}
	t.Cleanup(func() { authLimiter, llmLimiter = prevAuth, prevLLM })

	next := func(w http.ResponseWriter, r *http.Request) {}

	for name, h := range map[string]http.HandlerFunc{"auth": limitAuth(next), "llm": limitLLM(next)} {
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest("POST", "/", nil))
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("%s: status = %d", name, rec.Code)
		}
	}
}

func TestWithOptionalAuth(t *testing.T) {
	t.Setenv("SECRET_KEY", "optional-auth")
	token, _ := createToken(5)

	tests := map[string]struct {
		header string
		wantID int64
		wantOK bool
	}{
		"no header":   {"", 0, false},
		"bad token":   {"Bearer nope", 0, false},
		"empty":       {"Bearer ", 0, false},
		"wrong kind":  {"Basic " + token, 0, false},
		"valid token": {"Bearer " + token, 5, true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var gotID int64
			var gotOK, ran bool

			h := withOptionalAuth(func(w http.ResponseWriter, r *http.Request) {
				ran = true
				gotID, gotOK = profileIDFromContext(r.Context())
			})

			r := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				r.Header.Set("Authorization", tt.header)
			}
			h(httptest.NewRecorder(), r)

			if !ran || gotID != tt.wantID || gotOK != tt.wantOK {
				t.Errorf("ran=%v id=%d ok=%v, want id=%d ok=%v", ran, gotID, gotOK, tt.wantID, tt.wantOK)
			}
		})
	}
}

func TestProfileIDFromContext(t *testing.T) {
	if _, ok := profileIDFromContext(context.Background()); ok {
		t.Error("empty context has a profile id")
	}

	ctx := context.WithValue(context.Background(), profileIDContextKey, "not an int")
	if _, ok := profileIDFromContext(ctx); ok {
		t.Error("wrong-typed value accepted")
	}
}

func TestWriteHelpers(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusTeapot, "short and stout")

	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)

	if rec.Code != http.StatusTeapot || body["error"] != "short and stout" || rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("writeError = %d %v %q", rec.Code, body, rec.Header().Get("Content-Type"))
	}

	if got := statusResponse("ok"); got["status"] != "ok" {
		t.Errorf("statusResponse = %v", got)
	}
}

func TestPathID(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.SetPathValue("id", "42")

	if id, ok := pathID(httptest.NewRecorder(), r, "id", "job"); !ok || id != 42 {
		t.Errorf("pathID = %d, %v", id, ok)
	}

	r.SetPathValue("id", "4x")
	rec := httptest.NewRecorder()
	if _, ok := pathID(rec, r, "id", "job"); ok || rec.Code != http.StatusBadRequest || !json.Valid(rec.Body.Bytes()) {
		t.Errorf("bad pathID = %v, %d", ok, rec.Code)
	}
}

func TestDecodeJSONRejectsHugeBody(t *testing.T) {
	big := make([]byte, maxJSONBodySize+10)
	for i := range big {
		big[i] = ' '
	}
	big[0] = '"'
	big[len(big)-1] = '"'

	r := httptest.NewRequest("POST", "/", bytes.NewReader(big))

	var v string
	if err := decodeJSON(httptest.NewRecorder(), r, &v); err == nil {
		t.Error("oversized body decoded")
	}
}
