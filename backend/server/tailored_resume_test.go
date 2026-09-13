package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"main/database"
	"main/llm"
)

func TestHandleGetTailoredResumeUnauthorized(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/jobs/1/tailored-resume", nil)
	r.SetPathValue("id", "1")
	rec := httptest.NewRecorder()

	handleGetTailoredResume(rec, r)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleGetTailoredResumeInvalidJobID(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/jobs/abc/tailored-resume", nil)
	r.SetPathValue("id", "abc")
	r = withProfileID(r, 1)
	rec := httptest.NewRecorder()

	handleGetTailoredResume(rec, r)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateTailoredResumeTooLarge(t *testing.T) {
	body := `{"summary":"` + strings.Repeat("a", maxTailoredResumeSize) + `"}`
	r := httptest.NewRequest("PUT", "/api/jobs/1/tailored-resume", strings.NewReader(body))
	r.SetPathValue("id", "1")
	r = withProfileID(r, 1)
	rec := httptest.NewRecorder()

	handleUpdateTailoredResume(rec, r)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestHandleGetTailoredResumeNotFound(t *testing.T) {
	profileID := newTestProfileWithActiveResume(t, "tailored-get-none@example.com")
	jobID := seedJob(t, "https://example.com/tailored-get-none")

	r := httptest.NewRequest("GET", "/api/jobs/x/tailored-resume", nil)
	r.SetPathValue("id", strconv.FormatInt(jobID, 10))
	r = withProfileID(r, profileID)
	rec := httptest.NewRecorder()

	handleGetTailoredResume(rec, r)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d, body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestHandleGenerateTailoredResumeNoActiveResume(t *testing.T) {
	requireTestDB(t)

	profileID, err := database.CreateProfile(&database.ProfileRequest{Email: "tailored-no-resume@example.com", Password: "securepassword123"})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	r := httptest.NewRequest("POST", "/api/jobs/1/tailored-resume", nil)
	r.SetPathValue("id", "1")
	r = withProfileID(r, profileID)
	rec := httptest.NewRecorder()

	handleGenerateTailoredResume(rec, r)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d, body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestHandleGenerateTailoredResumeJobNotFound(t *testing.T) {
	profileID := newTestProfileWithActiveResume(t, "tailored-job-not-found@example.com")

	r := httptest.NewRequest("POST", "/api/jobs/999999999/tailored-resume", nil)
	r.SetPathValue("id", "999999999")
	r = withProfileID(r, profileID)
	rec := httptest.NewRecorder()

	handleGenerateTailoredResume(rec, r)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d, body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestHandleUpdateTailoredResumeNotFound(t *testing.T) {
	profileID := newTestProfileWithActiveResume(t, "tailored-put-none@example.com")
	jobID := seedJob(t, "https://example.com/tailored-put-none")

	r := httptest.NewRequest("PUT", "/api/jobs/x/tailored-resume", strings.NewReader(`{"summary":"edited"}`))
	r.SetPathValue("id", strconv.FormatInt(jobID, 10))
	r = withProfileID(r, profileID)
	rec := httptest.NewRecorder()

	handleUpdateTailoredResume(rec, r)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d, body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestHandleUpdateTailoredResumeRoundTrip(t *testing.T) {
	profileID := newTestProfileWithActiveResume(t, "tailored-round-trip@example.com")
	jobID := seedJob(t, "https://example.com/tailored-round-trip")

	extraction, found, err := database.GetActiveResumeExtraction(context.Background(), profileID)
	if err != nil || !found {
		t.Fatalf("get active extraction: found=%v err=%v", found, err)
	}

	original := llm.TailoredResume{FullName: "Ada Lovelace", Summary: "original"}
	if err := database.UpsertTailoredResume(context.Background(), profileID, jobID, extraction.ResumeID, extraction.UpdatedAt, original); err != nil {
		t.Fatalf("upsert tailored resume: %v", err)
	}

	r := httptest.NewRequest("PUT", "/api/jobs/x/tailored-resume", strings.NewReader(`{"full_name":"Ada Lovelace","summary":"edited"}`))
	r.SetPathValue("id", strconv.FormatInt(jobID, 10))
	r = withProfileID(r, profileID)
	rec := httptest.NewRecorder()

	handleUpdateTailoredResume(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got database.TailoredResume
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.Content.Summary != "edited" {
		t.Errorf("Summary = %q, want %q", got.Content.Summary, "edited")
	}
	if got.Stale {
		t.Error("Stale = true, want false right after generation")
	}
}
