package scraper

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"main/database"
	"main/jobs"
	"main/utils"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type fakeSource struct {
	jobs  []jobs.Job
	err   error
	panic bool
}

func (f fakeSource) FetchJobs(context.Context) ([]jobs.Job, error) {
	if f.panic {
		panic("source exploded")
	}

	return f.jobs, f.err
}

func sortedTitles(js []jobs.Job) []string {
	var out []string
	for _, j := range js {
		out = append(out, j.Title)
	}
	slices.Sort(out)
	return out
}

func TestKeyFor(t *testing.T) {
	march := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	base := jobs.Job{Company: "Acme", Title: "Dev", Location: "NYC", PostedAt: march}

	same := base
	same.PostedAt = march.Add(10 * 24 * time.Hour)
	same.URL = "https://other.example"

	if keyFor(base) != keyFor(same) {
		t.Error("same company/title/location/month should share a key")
	}

	for name, mutate := range map[string]func(*jobs.Job){
		"company":  func(j *jobs.Job) { j.Company = "Globex" },
		"title":    func(j *jobs.Job) { j.Title = "SRE" },
		"location": func(j *jobs.Job) { j.Location = "SF" },
		"month":    func(j *jobs.Job) { j.PostedAt = march.AddDate(0, 1, 0) },
		"year":     func(j *jobs.Job) { j.PostedAt = march.AddDate(1, 0, 0) },
	} {
		other := base
		mutate(&other)

		if keyFor(base) == keyFor(other) {
			t.Errorf("different %s should change the key", name)
		}
	}
}

func TestFetchAll(t *testing.T) {
	now := time.Now()
	salaryMin := 90000

	sources := []jobs.JobSource{
		fakeSource{jobs: []jobs.Job{
			{Title: "Dev", Company: "Acme", PostedAt: now},
			{Title: "Paid", Company: "Acme", PostedAt: now, Description: "Pay: $100,000 - $150,000 per year"},
			{Title: "Explicit", Company: "Acme", PostedAt: now, SalaryMin: &salaryMin, Description: "$1 - $2 per year"},
		}},
		fakeSource{jobs: []jobs.Job{
			{Title: "Dev", Company: "Acme", PostedAt: now, URL: "https://dup.example"},
			{Title: "Other", Company: "Globex", PostedAt: now},
		}},
		fakeSource{err: errors.New("down")},
		fakeSource{panic: true},
	}

	got := fetchAll(t.Context(), sources)

	if titles := sortedTitles(got); !slices.Equal(titles, []string{"Dev", "Explicit", "Other", "Paid"}) {
		t.Fatalf("titles = %v", titles)
	}

	for _, j := range got {
		switch j.Title {
		case "Paid":
			if j.SalaryMin == nil || *j.SalaryMin != 100000 || j.SalaryMax == nil || *j.SalaryMax != 150000 {
				t.Errorf("salary fallback = %v-%v", j.SalaryMin, j.SalaryMax)
			}
		case "Explicit":
			if j.SalaryMin == nil || *j.SalaryMin != 90000 || j.SalaryMax != nil {
				t.Errorf("explicit salary overwritten: %v-%v", j.SalaryMin, j.SalaryMax)
			}
		case "Dev", "Other":
			if j.SalaryMin != nil || j.SalaryMax != nil {
				t.Errorf("%s got salary from nowhere", j.Title)
			}
		}
	}
}

func TestFetchAllNoSources(t *testing.T) {
	if got := fetchAll(t.Context(), nil); len(got) != 0 {
		t.Errorf("fetchAll(nil) = %v", got)
	}
}

func TestAllSources(t *testing.T) {
	sources := allSources()

	if len(sources) < 10 {
		t.Fatalf("allSources() = %d sources, want at least 10", len(sources))
	}

	seen := map[string]bool{}
	for _, s := range sources {
		name := fmt.Sprintf("%T", s)
		if seen[name] {
			t.Errorf("source %s listed twice", name)
		}
		seen[name] = true
	}
}

const resetTestDBSQL = `DO $$
DECLARE t text;
BEGIN
	FOR t IN SELECT tablename FROM pg_tables WHERE schemaname = 'public' LOOP
		EXECUTE 'DROP TABLE IF EXISTS public.' || quote_ident(t) || ' CASCADE';
	END LOOP;
END $$;`

// truncateTestDBSQL empties every table but keeps the schema, between tests.
const truncateTestDBSQL = `DO $$
DECLARE t text;
BEGIN
	FOR t IN SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tablename <> 'schema_migrations' LOOP
		EXECUTE 'TRUNCATE TABLE public.' || quote_ident(t) || ' RESTART IDENTITY CASCADE';
	END LOOP;
END $$;`

var (
	testDBOnce sync.Once
	testDB     *sql.DB
	testDBErr  error
)

// newTestDB rebuilds the schema once per package run, then empties it for each test.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()

	connString := utils.Getenv("TEST_DATABASE_CONNECTION")
	if connString == "" {
		t.Skip("TEST_DATABASE_CONNECTION not set; skipping tests that require a live Postgres database")
	}

	testDBOnce.Do(func() {
		testDB, testDBErr = sql.Open("pgx", connString)
		if testDBErr != nil {
			return
		}

		if _, testDBErr = testDB.Exec(resetTestDBSQL); testDBErr != nil {
			return
		}

		database.SetDB(testDB)
		testDBErr = database.CreateTables()
	})

	if testDBErr != nil {
		t.Fatalf("init test db: %v", testDBErr)
	}

	if _, err := testDB.Exec(truncateTestDBSQL); err != nil {
		t.Fatalf("truncate test db: %v", err)
	}

	database.SetDB(testDB)

	return testDB
}

func TestRunScraperWritesFreshJobs(t *testing.T) {
	db := newTestDB(t)
	// No key: embedding fails fast and is only logged.
	t.Setenv("JINA_API_KEY", "")

	now := time.Now()
	sources := []jobs.JobSource{fakeSource{jobs: []jobs.Job{
		{Title: "Fresh", Company: "Acme", URL: "https://example.com/fresh", PostedAt: now},
		{Title: "Stale", Company: "Acme", URL: "https://example.com/stale", PostedAt: now.Add(-database.JobMaxAge - time.Hour)},
		{Title: "Undated", Company: "Acme", URL: "https://example.com/undated"},
	}}}

	runScraper(t.Context(), sources)

	rows, err := db.Query("SELECT title FROM jobs ORDER BY title")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var titles []string
	for rows.Next() {
		var title string
		_ = rows.Scan(&title)
		titles = append(titles, title)
	}

	if !slices.Equal(titles, []string{"Fresh", "Undated"}) {
		t.Errorf("stored titles = %v, want Fresh and Undated", titles)
	}
}

func TestRunScraperCancelledSkipsWrite(t *testing.T) {
	db := newTestDB(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	runScraper(ctx, []jobs.JobSource{fakeSource{jobs: []jobs.Job{
		{Title: "Fresh", Company: "Acme", URL: "https://example.com/fresh", PostedAt: time.Now()},
	}}})

	var n int
	if err := db.QueryRow("SELECT count(*) FROM jobs").Scan(&n); err != nil {
		t.Fatal(err)
	}

	if n != 0 {
		t.Errorf("stored %d jobs after cancel, want 0", n)
	}
}
