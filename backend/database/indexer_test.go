package database

import (
	"context"
	"database/sql"
	"errors"
	"main/jobs"
	"os"
	"slices"
	"testing"
	"time"
)

func newTestDB(t *testing.T) {
	t.Helper()

	connString := os.Getenv("TEST_DATABASE_CONNECTION")

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

	if _, err := db.Exec("DROP TABLE IF EXISTS job_tags, jobs, tags CASCADE;"); err != nil {
		t.Fatalf("reset test db: %v", err)
	}

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

	if err := WriteToDatabase([]jobs.Job{seed}); err != nil {
		t.Fatalf("seed db: %v", err)
	}

	var id int64
	if err := cachedDb.QueryRow("SELECT id FROM jobs WHERE url = ?", seed.URL).Scan(&id); err != nil {
		t.Fatalf("lookup seeded id: %v", err)
	}

	got, err := GetJobByID(context.Background(), id)

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

func TestGetJobByIDNotFound(t *testing.T) {
	newTestDB(t)

	if err := WriteToDatabase(nil); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	_, err := GetJobByID(context.Background(), 999)

	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("err = %v, want sql.ErrNoRows", err)
	}
}
