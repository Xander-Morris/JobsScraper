package database

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"main/jobs"
)

func seedJobs(t *testing.T, batch ...jobs.Job) {
	t.Helper()

	for i := range batch {
		if batch[i].PostedAt.IsZero() {
			batch[i].PostedAt = time.Now()
		}
	}

	if err := WriteJobsToDatabase(t.Context(), batch); err != nil {
		t.Fatalf("seed jobs: %v", err)
	}
}

func TestUpdateEducation(t *testing.T) {
	newTestDB(t)
	ctx := t.Context()
	id := newTestProfile(t)
	other := newNamedProfile(t, "other@example.com")

	eduID, err := AddEducation(ctx, id, &AddEducationRequest{SchoolName: "MIT", Major: "CS", Degree: "BS"})
	if err != nil {
		t.Fatal(err)
	}

	gpa := 3.5
	if err := UpdateEducation(ctx, id, eduID, &AddEducationRequest{
		SchoolName: "Stanford", Major: "Math", Degree: "MS", GPA: &gpa, StartDate: "2020-09-01", EndDate: "2022-06-01",
	}); err != nil {
		t.Fatalf("UpdateEducation: %v", err)
	}

	profile, _ := GetProfile(ctx, id)
	got := profile.Education[0]

	if got.SchoolName != "Stanford" || got.Major != "Math" || got.Degree != "MS" || got.GPA == nil || *got.GPA != gpa ||
		got.StartDate == nil || *got.StartDate != "2020-09-01" || got.EndDate == nil || *got.EndDate != "2022-06-01" {
		t.Errorf("education = %+v", got)
	}

	for _, req := range []AddEducationRequest{
		{Major: "x", Degree: "y"},
		{SchoolName: "x", Major: "y", Degree: "z", StartDate: "bad"},
		{SchoolName: "x", Major: "y", Degree: "z", EndDate: "bad"},
	} {
		if err := UpdateEducation(ctx, id, eduID, &req); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("UpdateEducation(%+v) = %v, want ErrInvalidInput", req, err)
		}
	}

	valid := &AddEducationRequest{SchoolName: "x", Major: "y", Degree: "z"}
	if err := UpdateEducation(ctx, other, eduID, valid); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("UpdateEducation(other profile) = %v, want sql.ErrNoRows", err)
	}
}

func TestMarkAndUnmarkJobApplied(t *testing.T) {
	newTestDB(t)
	ctx := t.Context()
	id := newTestProfile(t)

	seedJobs(t, jobs.Job{Title: "Dev", Company: "Acme", URL: "https://example.com/a"})
	jobID := jobIDByURL(t, "https://example.com/a")

	applied := func() bool {
		job, err := GetJobByID(ctx, jobID, JobDetailParams{ProfileID: id})
		if err != nil {
			t.Fatal(err)
		}
		return job.Applied
	}

	if applied() {
		t.Fatal("job applied before marking")
	}

	if err := MarkJobApplied(ctx, id, jobID); err != nil {
		t.Fatal(err)
	}

	// Marking twice is idempotent.
	if err := MarkJobApplied(ctx, id, jobID); err != nil {
		t.Fatalf("second MarkJobApplied: %v", err)
	}

	if !applied() {
		t.Error("job not applied after marking")
	}

	if err := UnmarkJobApplied(ctx, id, jobID); err != nil {
		t.Fatal(err)
	}

	if applied() {
		t.Error("job still applied after unmarking")
	}

	if err := UnmarkJobApplied(ctx, id, jobID); err != nil {
		t.Errorf("unmarking twice = %v, want nil", err)
	}
}

func TestFetchAllUniqueTags(t *testing.T) {
	newTestDB(t)

	seedJobs(t,
		jobs.Job{Title: "A", Company: "X", URL: "https://example.com/1", Tags: []string{"go", "sql"}},
		jobs.Job{Title: "B", Company: "Y", URL: "https://example.com/2", Tags: []string{"sql", "aws"}},
	)

	tags, err := FetchAllUniqueTags(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{"aws", "go", "sql"}; !slices.Equal(tags, want) {
		t.Errorf("tags = %v, want %v", tags, want)
	}
}

func TestKnownJobURLs(t *testing.T) {
	newTestDB(t)

	seedJobs(t, jobs.Job{Title: "A", Company: "X", URL: "https://example.com/known"})

	known, err := KnownJobURLs(t.Context(), []string{"https://example.com/known", "https://example.com/new"})
	if err != nil {
		t.Fatal(err)
	}

	if !known["https://example.com/known"] || known["https://example.com/new"] || len(known) != 1 {
		t.Errorf("known = %v", known)
	}

	empty, err := KnownJobURLs(t.Context(), nil)
	if err != nil || len(empty) != 0 {
		t.Errorf("KnownJobURLs(nil) = %v, %v", empty, err)
	}
}

func TestDigestQueries(t *testing.T) {
	newTestDB(t)
	ctx := t.Context()

	optedIn := newNamedProfile(t, "in@example.com")
	newNamedProfile(t, "out@example.com")

	if err := UpdateProfile(ctx, optedIn, &UpdateProfileRequest{EmailNotifications: true}); err != nil {
		t.Fatal(err)
	}

	profiles, err := ListProfilesForDigest(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(profiles) != 1 || profiles[0].ProfileID != optedIn || profiles[0].Email != "in@example.com" || profiles[0].LastDigestSentAt != nil {
		t.Fatalf("profiles = %+v", profiles)
	}

	sentAt := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	if err := MarkDigestSent(ctx, optedIn, sentAt); err != nil {
		t.Fatal(err)
	}

	profiles, _ = ListProfilesForDigest(ctx)
	if profiles[0].LastDigestSentAt == nil || !profiles[0].LastDigestSentAt.Equal(sentAt) {
		t.Errorf("LastDigestSentAt = %v, want %v", profiles[0].LastDigestSentAt, sentAt)
	}

	if err := DisableEmailDigest(ctx, optedIn); err != nil {
		t.Fatal(err)
	}

	profiles, _ = ListProfilesForDigest(ctx)
	if len(profiles) != 0 {
		t.Errorf("profiles after unsubscribe = %+v", profiles)
	}
}

// fakeJina serves deterministic embeddings and counts calls.
func fakeJina(t *testing.T) *atomic.Int32 {
	t.Helper()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		var req struct {
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		type datum struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		}

		resp := struct {
			Data []datum `json:"data"`
		}{}
		for i := range req.Input {
			resp.Data = append(resp.Data, datum{Index: i, Embedding: unitVector(i % 768)})
		}

		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)

	t.Setenv("JINA_API_URL", srv.URL)
	t.Setenv("JINA_API_KEY", "test-key")

	return &calls
}

func TestEmbedPendingJobs(t *testing.T) {
	newTestDB(t)
	calls := fakeJina(t)

	var batch []jobs.Job
	for i := range embedJobsBatchSize + 5 {
		batch = append(batch, jobs.Job{
			Title: fmt.Sprintf("Job %d", i), Company: "X", URL: fmt.Sprintf("https://example.com/%d", i), Description: "Write Go",
		})
	}
	batch = append(batch, jobs.Job{Title: "No description", Company: "X", URL: "https://example.com/nodesc"})
	seedJobs(t, batch...)

	if err := EmbedPendingJobs(t.Context()); err != nil {
		t.Fatalf("EmbedPendingJobs: %v", err)
	}

	if n := calls.Load(); n != 2 {
		t.Errorf("jina calls = %d, want 2 batches", n)
	}

	var pending, empty int
	if err := cachedDb.QueryRow(`SELECT
		count(*) FILTER (WHERE embedding IS NULL AND coalesce(description, '') <> ''),
		count(*) FILTER (WHERE embedding IS NULL AND coalesce(description, '') = '')
		FROM jobs`).Scan(&pending, &empty); err != nil {
		t.Fatal(err)
	}

	if pending != 0 || empty != 1 {
		t.Errorf("pending = %d, empty-description unembedded = %d; want 0, 1", pending, empty)
	}

	// Nothing left, so no further calls.
	if err := EmbedPendingJobs(t.Context()); err != nil {
		t.Fatal(err)
	}

	if n := calls.Load(); n != 2 {
		t.Errorf("jina calls after second run = %d, want 2", n)
	}
}

func TestEmbedPendingJobsJinaError(t *testing.T) {
	newTestDB(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("JINA_API_URL", srv.URL)
	t.Setenv("JINA_API_KEY", "test-key")

	seedJobs(t, jobs.Job{Title: "A", Company: "X", URL: "https://example.com/a", Description: "d"})

	if err := EmbedPendingJobs(t.Context()); err == nil {
		t.Fatal("EmbedPendingJobs = nil, want error")
	}
}
