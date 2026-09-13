package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"main/jobs"
	"main/llm"
)

func TestTailoredResumeUpsertAndStale(t *testing.T) {
	newTestDB(t)
	ctx := context.Background()
	profileID := newTestProfile(t)

	resumeID, err := AddResume(ctx, profileID, "resume.pdf", "application/pdf", []byte("resume"))
	if err != nil {
		t.Fatalf("add resume: %v", err)
	}
	if err := SaveResumeExtractionResult(ctx, resumeID, &llm.ExtractedResume{FullName: "Ada"}); err != nil {
		t.Fatalf("save extraction: %v", err)
	}

	seed := jobs.Job{Title: "Engineer", Company: "Acme", URL: "https://example.com/tailored", PostedAt: time.Now()}
	if err := WriteJobsToDatabase([]jobs.Job{seed}); err != nil {
		t.Fatalf("seed job: %v", err)
	}
	jobID := jobIDByURL(t, seed.URL)

	if _, err := GetTailoredResume(ctx, profileID, jobID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetTailoredResume before upsert err = %v, want sql.ErrNoRows", err)
	}

	extraction, found, err := GetActiveResumeExtraction(ctx, profileID)
	if err != nil || !found {
		t.Fatalf("get active extraction: found=%v err=%v", found, err)
	}

	for _, summary := range []string{"first", "second"} {
		content := llm.TailoredResume{Summary: summary}
		if err := UpsertTailoredResume(ctx, profileID, jobID, resumeID, extraction.UpdatedAt, content); err != nil {
			t.Fatalf("upsert %q: %v", summary, err)
		}
	}

	var count int
	if err := cachedDb.QueryRow("SELECT COUNT(*) FROM profile_tailored_resumes WHERE profile_id = $1", profileID).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Errorf("rows = %d, want 1 per (profile, job)", count)
	}

	got, err := GetTailoredResume(ctx, profileID, jobID)
	if err != nil {
		t.Fatalf("GetTailoredResume: %v", err)
	}
	if got.Content.Summary != "second" || got.Stale {
		t.Errorf("got summary=%q stale=%v, want second/false", got.Content.Summary, got.Stale)
	}

	if err := SaveResumeExtractionResult(ctx, resumeID, &llm.ExtractedResume{FullName: "Ada L"}); err != nil {
		t.Fatalf("re-extract: %v", err)
	}

	got, err = GetTailoredResume(ctx, profileID, jobID)
	if err != nil {
		t.Fatalf("GetTailoredResume after re-extraction: %v", err)
	}
	if !got.Stale {
		t.Error("Stale = false after re-extraction, want true")
	}
}
