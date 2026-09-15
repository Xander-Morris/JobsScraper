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

	// CASCADE only cascades to dependent objects like FK constraints, not to
	// child tables themselves; every table with an FK into profiles has to be
	// listed here, or its rows outlive the id sequence reset and collide with
	// fresh test profiles reusing the same ids.
	// schema_migrations needs dropping too, since it survives the reset
	// otherwise. Left in place, CreateTables' next call sees the target
	// version already applied and skips recreating everything just dropped.
	const dropTables = `job_tags, jobs, tags, profile_refresh_tokens, profiles_education,
		profiles_work_experience_bullets, profiles_work_experience, profiles_skills,
		profile_resume_extractions, profile_resumes, profiles, profile_job_applications,
		profile_tailored_resumes, profile_password_reset_tokens, schema_migrations`

	if _, err := db.Exec("DROP TABLE IF EXISTS " + dropTables + " CASCADE;"); err != nil {
		t.Fatalf("reset test db: %v", err)
	}
	createdTables = false

	prev := SetDB(db)

	t.Cleanup(func() {
		db.Close()
		SetDB(prev)
	})

	if err := CreateTables(); err != nil {
		t.Fatalf("init schema: %v", err)
	}
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

	if got.Description != seed.Description {
		t.Errorf("Description = %q, want %q", got.Description, seed.Description)
	}
}

func TestSearchForJobsPage(t *testing.T) {
	newTestDB(t)
	ctx := context.Background()

	seed := []jobs.Job{
		{Title: "Older", Company: "Acme", Tags: []string{"go", "sql"}, PostedAt: time.Now().Add(-48 * time.Hour),
			URL: "https://example.com/jobs/older", Description: "long text"},
		{Title: "Newer", Company: "Acme", PostedAt: time.Now().Add(-time.Hour),
			URL: "https://example.com/jobs/newer", Description: "long text"},
	}

	if err := WriteJobsToDatabase(seed); err != nil {
		t.Fatalf("seed db: %v", err)
	}

	olderID := jobIDByURL(t, seed[0].URL)

	var profileID int64
	if err := cachedDb.QueryRow(`INSERT INTO profiles (email, password) VALUES ('page@example.com', 'x') RETURNING id`).Scan(&profileID); err != nil {
		t.Fatalf("seed profile: %v", err)
	}

	if err := MarkJobApplied(ctx, profileID, olderID); err != nil {
		t.Fatalf("mark applied: %v", err)
	}

	result, err := SearchForJobs(ctx, &JobSearchParams{Sort: SortDate, ProfileID: profileID, Limit: 10})
	if err != nil {
		t.Fatalf("SearchForJobs: %v", err)
	}

	if len(result.Jobs) != 2 || result.Jobs[0].Title != "Newer" || result.Jobs[1].Title != "Older" {
		t.Fatalf("jobs = %+v, want Newer then Older", result.Jobs)
	}

	if result.Total != 2 {
		t.Errorf("Total = %d, want 2", result.Total)
	}

	newer, older := result.Jobs[0], result.Jobs[1]

	if !slices.Equal(older.Tags, []string{"go", "sql"}) {
		t.Errorf("older Tags = %v, want [go sql]", older.Tags)
	}

	if newer.Tags == nil || len(newer.Tags) != 0 {
		t.Errorf("newer Tags = %#v, want empty non-nil slice", newer.Tags)
	}

	if !older.Applied || newer.Applied {
		t.Errorf("Applied = older %v, newer %v, want true, false", older.Applied, newer.Applied)
	}

	for _, job := range result.Jobs {
		if job.Description != "" {
			t.Errorf("%s Description = %q, want empty in search results", job.Title, job.Description)
		}
	}

	past, err := SearchForJobs(ctx, &JobSearchParams{Limit: 10, Offset: 50})
	if err != nil {
		t.Fatalf("SearchForJobs past end: %v", err)
	}

	if len(past.Jobs) != 0 || past.Total != 2 {
		t.Errorf("past end = %d jobs, Total %d, want 0 jobs, Total 2", len(past.Jobs), past.Total)
	}
}

func TestJobTypeFlagsFor(t *testing.T) {
	tests := []struct {
		job  jobs.Job
		want jobTypeFlags
	}{
		{jobs.Job{Title: "Software Engineer Intern"}, jobTypeFlags{intern: true}},
		{jobs.Job{Title: "Data Analyst", Tags: []string{"Internship", "Part-time"}}, jobTypeFlags{intern: true, partTime: true}},
		{jobs.Job{Title: "Support Agent", Tags: []string{"Part Time"}}, jobTypeFlags{partTime: true}},
		{jobs.Job{Title: "Internal Tools Engineer", Tags: []string{"full_time"}}, jobTypeFlags{fullTime: true}},
		{jobs.Job{Title: "International Sales", Tags: []string{"Fulltimer", "internals"}}, jobTypeFlags{}},
	}

	for _, tt := range tests {
		if got := jobTypeFlagsFor(tt.job); got != tt.want {
			t.Errorf("jobTypeFlagsFor(%q, %v) = %+v, want %+v", tt.job.Title, tt.job.Tags, got, tt.want)
		}
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
	// third job's embedding stays NULL, not yet processed by the background embed step.

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

func TestSearchForJobsJobType(t *testing.T) {
	newTestDB(t)

	seed := []jobs.Job{
		{Title: "Software Engineer Intern", Company: "Acme", PostedAt: time.Now(), URL: "https://example.com/jobs/intern-title"},
		{Title: "Data Analyst", Company: "Acme", Tags: []string{"Internship", "Part-time"}, PostedAt: time.Now(), URL: "https://example.com/jobs/intern-and-part"},
		{Title: "Support Agent", Company: "Acme", Tags: []string{"Part Time"}, PostedAt: time.Now(), URL: "https://example.com/jobs/part"},
		{Title: "Backend Engineer", Company: "Acme", Tags: []string{"Full-time permanent"}, PostedAt: time.Now(), URL: "https://example.com/jobs/full"},
		{Title: "Internal Tools Engineer", Company: "Acme", Tags: []string{"full_time"}, PostedAt: time.Now(), URL: "https://example.com/jobs/internal-full"},
		{Title: "Designer", Company: "Acme", PostedAt: time.Now(), URL: "https://example.com/jobs/untyped"},
	}

	if err := WriteJobsToDatabase(seed); err != nil {
		t.Fatalf("seed db: %v", err)
	}

	tests := []struct {
		jobType JobTypeFilter
		want    []string
	}{
		{JobTypeFilterIntern, []string{"Data Analyst", "Software Engineer Intern"}},
		{JobTypeFilterPartTime, []string{"Support Agent"}},
		{JobTypeFilterFullTime, []string{"Backend Engineer", "Internal Tools Engineer"}},
	}

	for _, tt := range tests {
		t.Run(string(tt.jobType), func(t *testing.T) {
			result, err := SearchForJobs(context.Background(), &JobSearchParams{JobType: tt.jobType, Limit: 10})
			if err != nil {
				t.Fatalf("SearchForJobs: %v", err)
			}

			var got []string
			for _, job := range result.Jobs {
				got = append(got, job.Title)
			}
			slices.Sort(got)

			if !slices.Equal(got, tt.want) {
				t.Errorf("titles = %v, want %v", got, tt.want)
			}

			if result.Total != len(tt.want) {
				t.Errorf("Total = %d, want %d", result.Total, len(tt.want))
			}
		})
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
