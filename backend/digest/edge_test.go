package digest

import (
	"context"
	"testing"
	"time"

	"main/database"
	"main/internal/faultdb"
	"main/jobs"
	"main/llm"
	"main/utils"
)

func TestStartSchedulerStopsWhenCancelled(t *testing.T) {
	newTestDB(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	// One run with a dead context: listing profiles fails and is logged.
	StartScheduler(ctx)
}

func TestSendForProfileSafelyRecoversPanic(t *testing.T) {
	prev := sendDigest
	sendDigest = func(context.Context, database.DigestProfile) error { panic("boom") }
	t.Cleanup(func() { sendDigest = prev })

	sendForProfileSafely(t.Context(), database.DigestProfile{ProfileID: 1})
}

func TestSendForProfileSkipsResumeWithoutKeywords(t *testing.T) {
	newTestDB(t)
	id := digestProfile(t, "blank@example.com")
	resumeID, err := database.AddResume(t.Context(), id, "r.pdf", "application/pdf", []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	if err := database.SaveResumeExtractionResult(t.Context(), resumeID, &llm.ExtractedResume{}); err != nil {
		t.Fatal(err)
	}

	if err := sendForProfile(t.Context(), database.DigestProfile{ProfileID: id}); err != nil {
		t.Errorf("sendForProfile = %v, want nil", err)
	}
}

func TestDigestSurvivesDatabaseFaults(t *testing.T) {
	newTestDB(t)
	sent := fakeResend(t)
	id := digestProfile(t, "faults@example.com", "go")
	if err := database.WriteJobsToDatabase(t.Context(), []jobs.Job{{Title: "Go Developer", Company: "Acme", URL: "https://example.com/go", Tags: []string{"go"}, PostedAt: time.Now(), Description: "Go"}}); err != nil {
		t.Fatal(err)
	}

	pool, inj, err := faultdb.Open(utils.Getenv("TEST_DATABASE_CONNECTION"))
	if err != nil {
		t.Fatal(err)
	}
	prev := database.SetDB(pool)
	t.Cleanup(func() { database.SetDB(prev); pool.Close() })

	clearSent := func() {
		if _, err := pool.Exec("UPDATE profiles SET last_digest_sent_at = NULL WHERE id = $1", id); err != nil {
			t.Fatal(err)
		}
	}

	// Faults are logged per profile, never escape the run.
	faultdb.SweepEach(t, inj, clearSent, func() { run(t.Context()) }, func(*testing.T) {})

	if len(sent()) == 0 {
		t.Error("no digest was ever sent, so the sweep didn't reach the send")
	}
}
