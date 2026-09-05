package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"main/database"
	"main/llm"
)

func newTestProfileWithActiveResume(t *testing.T, email string) int64 {
	t.Helper()
	requireTestDB(t)

	profileID, err := database.CreateProfile(&database.ProfileRequest{Email: email, Password: "securepassword123"})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	resumeID, err := database.AddResume(context.Background(), profileID, "resume.pdf", "application/pdf", []byte("resume content"))
	if err != nil {
		t.Fatalf("add resume: %v", err)
	}

	if err := database.UpsertResumeExtractionPending(context.Background(), resumeID); err != nil {
		t.Fatalf("mark extraction pending: %v", err)
	}

	extracted := &llm.ExtractedResume{FullName: "Ada Lovelace", Summary: "Backend engineer"}
	if err := database.SaveResumeExtractionResult(context.Background(), resumeID, extracted); err != nil {
		t.Fatalf("save extraction result: %v", err)
	}

	return profileID
}

func withProfileID(r *http.Request, profileID int64) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), profileIDContextKey, profileID))
}

func TestHandleGenerateApplicationContentUnauthorized(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/jobs/1/generate", nil)
	r.SetPathValue("id", "1")
	rec := httptest.NewRecorder()

	handleGenerateApplicationContent(rec, r)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleGenerateApplicationContentInvalidJobID(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/jobs/abc/generate", nil)
	r.SetPathValue("id", "abc")
	r = withProfileID(r, 1)
	rec := httptest.NewRecorder()

	handleGenerateApplicationContent(rec, r)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleGenerateApplicationContentNoActiveResume(t *testing.T) {
	requireTestDB(t)

	profileID, err := database.CreateProfile(&database.ProfileRequest{Email: "generation-no-resume@example.com", Password: "securepassword123"})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	r := httptest.NewRequest("POST", "/api/jobs/1/generate", nil)
	r.SetPathValue("id", "1")
	r = withProfileID(r, profileID)
	rec := httptest.NewRecorder()

	handleGenerateApplicationContent(rec, r)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d, body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestHandleGenerateApplicationContentJobNotFound(t *testing.T) {
	profileID := newTestProfileWithActiveResume(t, "generation-job-not-found@example.com")

	r := httptest.NewRequest("POST", "/api/jobs/999999999/generate", nil)
	r.SetPathValue("id", "999999999")
	r = withProfileID(r, profileID)
	rec := httptest.NewRecorder()

	handleGenerateApplicationContent(rec, r)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d, body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}
