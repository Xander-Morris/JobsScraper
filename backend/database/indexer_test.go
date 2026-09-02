package database

import (
	"context"
	"database/sql"
	"errors"
	"main/jobs"
	"main/utils"
	"slices"
	"testing"
	"time"

	"github.com/pgvector/pgvector-go"
)

func newTestDB(t *testing.T) {
	t.Helper()

	connString := utils.GetEnv()["TEST_DATABASE_CONNECTION"]

	if connString == "" {
		t.Skip("TEST_DATABASE_CONNECTION not set; skipping tests that require a live Postgres database")
	}

	db, err := sql.Open("pgx", connString)

	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := db.Ping(); err != nil {
		t.Fatalf("ping test db: %v", err)
	}

	// DROP TABLE ... CASCADE only cascades to dependent objects (e.g. FK constraints),
	// not to child tables themselves — every table with a FK into profiles must be
	// listed explicitly or its rows outlive profiles' id sequence reset and collide
	// with fresh test profiles that reuse the same ids.
	// schema_migrations must be dropped too — it's golang-migrate's version
	// table, and it survives this reset since it's not itself one of the
	// tables migration 000001 creates. Left in place, CreateTables' next call
	// would see the target version already applied and skip recreating
	// everything else that was just dropped.
	const dropTables = `job_tags, jobs, tags, profile_refresh_tokens, profiles_education,
		profiles_work_experience_bullets, profiles_work_experience, profiles_skills,
		profile_resume_extractions, profile_resumes, profiles, profile_job_applications,
		schema_migrations`

	if _, err := db.Exec("DROP TABLE IF EXISTS " + dropTables + " CASCADE;"); err != nil {
		t.Fatalf("reset test db: %v", err)
	}
	createdTables = false

	prev := SetDB(db)

	t.Cleanup(func() {
		db.Close()
		SetDB(prev)
	})
}

func TestGetJobByID(t *testing.T) {
	newTestDB(t)

	postedAt := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	salaryMin, salaryMax := 90000, 120000

	seed := jobs.Job{
		Title:         "Backend Engineer",
		Company:       "Acme",
		Location:      "Remote",
		WorkplaceType: jobs.Remote,
		Tags:          []string{"go", "sqlite"},
		SalaryMin:     &salaryMin,
		SalaryMax:     &salaryMax,
		PostedAt:      postedAt,
		URL:           "https://example.com/jobs/1",
		Description:   "Build things",
	}

	if err := WriteJobsToDatabase([]jobs.Job{seed}); err != nil {
		t.Fatalf("seed db: %v", err)
	}

	var id int64
	if err := cachedDb.QueryRow("SELECT id FROM jobs WHERE url = $1", seed.URL).Scan(&id); err != nil {
		t.Fatalf("lookup seeded id: %v", err)
	}

	got, err := GetJobByID(context.Background(), id, JobDetailParams{})

	if err != nil {
		t.Fatalf("GetJobByID: %v", err)
	}

	if got.ID != id {
		t.Errorf("ID = %d, want %d", got.ID, id)
	}

	if got.Title != seed.Title {
		t.Errorf("Title = %q, want %q", got.Title, seed.Title)
	}

	if got.Company != seed.Company {
		t.Errorf("Company = %q, want %q", got.Company, seed.Company)
	}

	if got.SalaryMin == nil || *got.SalaryMin != salaryMin {
		t.Errorf("SalaryMin = %v, want %d", got.SalaryMin, salaryMin)
	}

	if got.SalaryMax == nil || *got.SalaryMax != salaryMax {
		t.Errorf("SalaryMax = %v, want %d", got.SalaryMax, salaryMax)
	}

	if !got.PostedAt.Equal(postedAt) {
		t.Errorf("PostedAt = %v, want %v", got.PostedAt, postedAt)
	}

	gotTags := slices.Clone(got.Tags)
	slices.Sort(gotTags)
	wantTags := []string{"go", "sqlite"}

	if !slices.Equal(gotTags, wantTags) {
		t.Errorf("Tags = %v, want %v", gotTags, wantTags)
	}
}

// unitVector returns a 768-dim vector with a 1 at index and 0 elsewhere, so two
// jobs given the same index are an exact semantic match (cosine similarity 1)
// and two given different indices are orthogonal (similarity 0).
func unitVector(index int) []float32 {
	vec := make([]float32, 768)
	vec[index] = 1

	return vec
}

func setJobEmbedding(t *testing.T, jobID int64, vec []float32) {
	t.Helper()

	if _, err := cachedDb.Exec("UPDATE jobs SET embedding = $1 WHERE id = $2", pgvector.NewVector(vec), jobID); err != nil {
		t.Fatalf("set job embedding: %v", err)
	}
}

func jobIDByURL(t *testing.T, url string) int64 {
	t.Helper()

	var id int64
	if err := cachedDb.QueryRow("SELECT id FROM jobs WHERE url = $1", url).Scan(&id); err != nil {
		t.Fatalf("lookup job id: %v", err)
	}

	return id
}

func TestSearchForJobsResumeEmbedding(t *testing.T) {
	newTestDB(t)

	seed := []jobs.Job{
		{Title: "Match", Company: "Acme", PostedAt: time.Now(), URL: "https://example.com/jobs/match", Description: "x"},
		{Title: "Orthogonal", Company: "Acme", PostedAt: time.Now(), URL: "https://example.com/jobs/orthogonal", Description: "x"},
		{Title: "Unembedded", Company: "Acme", PostedAt: time.Now(), URL: "https://example.com/jobs/unembedded", Description: "x"},
	}

	if err := WriteJobsToDatabase(seed); err != nil {
		t.Fatalf("seed db: %v", err)
	}

	matchID := jobIDByURL(t, seed[0].URL)
	orthogonalID := jobIDByURL(t, seed[1].URL)

	resumeVec := unitVector(0)
	setJobEmbedding(t, matchID, unitVector(0))
	setJobEmbedding(t, orthogonalID, unitVector(1))
	// third job's embedding stays NULL — not yet processed by the background embed step.

	resumeEmbedding := pgvector.NewVector(resumeVec)

	result, err := SearchForJobs(context.Background(), &JobSearchParams{
		ResumeEmbedding: &resumeEmbedding,
		Limit:           10,
	})

	if err != nil {
		t.Fatalf("SearchForJobs: %v", err)
	}

	if len(result.Jobs) != 3 {
		t.Fatalf("got %d jobs, want 3", len(result.Jobs))
	}

	if result.Jobs[0].ID != matchID {
		t.Errorf("first result ID = %d, want %d (the semantic match)", result.Jobs[0].ID, matchID)
	}

	if result.Jobs[0].MatchScore == nil || *result.Jobs[0].MatchScore < 0.99 {
		t.Errorf("match job MatchScore = %v, want ~1", result.Jobs[0].MatchScore)
	}

	for _, job := range result.Jobs {
		if job.ID == orthogonalID {
			if job.MatchScore == nil || *job.MatchScore > 0.01 {
				t.Errorf("orthogonal job MatchScore = %v, want ~0", job.MatchScore)
			}
		}
	}
}

func TestGetJobByIDResumeEmbedding(t *testing.T) {
	newTestDB(t)

	seed := jobs.Job{Title: "Match", Company: "Acme", PostedAt: time.Now(), URL: "https://example.com/jobs/match", Description: "x"}
	unembedded := jobs.Job{Title: "Unembedded", Company: "Acme", PostedAt: time.Now(), URL: "https://example.com/jobs/unembedded", Description: "x"}

	if err := WriteJobsToDatabase([]jobs.Job{seed, unembedded}); err != nil {
		t.Fatalf("seed db: %v", err)
	}

	matchID := jobIDByURL(t, seed.URL)
	unembeddedID := jobIDByURL(t, unembedded.URL)
	setJobEmbedding(t, matchID, unitVector(0))

	resumeEmbedding := pgvector.NewVector(unitVector(0))

	got, err := GetJobByID(context.Background(), matchID, JobDetailParams{ResumeEmbedding: &resumeEmbedding})
	if err != nil {
		t.Fatalf("GetJobByID: %v", err)
	}
	if got.MatchScore == nil || *got.MatchScore < 0.99 {
		t.Errorf("MatchScore = %v, want ~1", got.MatchScore)
	}

	gotUnembedded, err := GetJobByID(context.Background(), unembeddedID, JobDetailParams{ResumeEmbedding: &resumeEmbedding})
	if err != nil {
		t.Fatalf("GetJobByID: %v", err)
	}
	if gotUnembedded.MatchScore != nil {
		t.Errorf("MatchScore = %v, want nil for a job with no embedding", *gotUnembedded.MatchScore)
	}
}

func TestGetJobByIDNotFound(t *testing.T) {
	newTestDB(t)

	if err := WriteJobsToDatabase(nil); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	_, err := GetJobByID(context.Background(), 999, JobDetailParams{})

	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("err = %v, want sql.ErrNoRows", err)
	}
}
