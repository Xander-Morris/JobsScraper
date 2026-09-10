package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"main/database"
	"main/jobs"
	"main/utils"
)

func TestMain(m *testing.M) {
	connString := testDBConnString()

	if connString == "" {
		fmt.Println("TEST_DATABASE_CONNECTION not set; running only the tests that need no database")

		// Backstop: point the package at an unreachable database, so a
		// DB-backed test missing its requireTestDB guard fails loudly instead
		// of quietly hitting the real database via GetDb/backend/.env.
		unreachable, err := sql.Open("pgx", "postgres://unreachable.invalid:5432/none")
		if err != nil {
			panic(err)
		}

		database.SetDB(unreachable)
		os.Exit(m.Run())
	}

	db, err := sql.Open("pgx", connString)
	if err != nil {
		panic(err)
	}

	if err := db.Ping(); err != nil {
		panic(err)
	}

	// Same list as database.newTestDB, same reason: CASCADE doesn't reach
	// child tables, so anything with an FK into profiles needs naming here —
	// otherwise its rows outlive the id sequence reset and a second local
	// test run collides on an already-registered email.
	const dropTables = `job_tags, jobs, tags, profile_refresh_tokens, profiles_education,
		profiles_work_experience_bullets, profiles_work_experience, profiles_skills,
		profile_resume_extractions, profile_resumes, profiles, profile_job_applications,
		schema_migrations`

	if _, err := db.Exec("DROP TABLE IF EXISTS " + dropTables + " CASCADE;"); err != nil {
		panic(err)
	}

	database.SetDB(db)

	if err := database.CreateTables(); err != nil {
		panic(err)
	}

	code := m.Run()

	db.Close()
	os.Exit(code)
}

// testDBConnString reads through utils.GetEnv, not os.Getenv, so a value in
// backend/.env counts — matches database.newTestDB, lets these tests run
// locally and not just in CI.
func testDBConnString() string {
	return utils.GetEnv()["TEST_DATABASE_CONNECTION"]
}

func requireTestDB(t *testing.T) {
	t.Helper()

	if testDBConnString() == "" {
		t.Skip("TEST_DATABASE_CONNECTION not set; skipping test that requires a live Postgres database")
	}
}

func TestParseJobSearchParams(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		check   func(t *testing.T, params *database.JobSearchParams)
	}{
		{
			name: "defaults",
			url:  "/api/jobs",
			check: func(t *testing.T, params *database.JobSearchParams) {
				if params.Limit != database.DefaultSearchLimit {
					t.Errorf("Limit = %d, want %d", params.Limit, database.DefaultSearchLimit)
				}
				if params.Offset != 0 {
					t.Errorf("Offset = %d, want 0", params.Offset)
				}
			},
		},
		{
			name: "trims search query",
			url:  "/api/jobs?q=%20go%20developer%20",
			check: func(t *testing.T, params *database.JobSearchParams) {
				if params.SearchQuery != "go developer" {
					t.Errorf("SearchQuery = %q, want %q", params.SearchQuery, "go developer")
				}
			},
		},
		{
			name: "valid workplace_type",
			url:  "/api/jobs?workplace_type=remote",
			check: func(t *testing.T, params *database.JobSearchParams) {
				if params.WorkplaceType != jobs.Remote {
					t.Errorf("WorkplaceType = %v, want %v", params.WorkplaceType, jobs.Remote)
				}
			},
		},
		{name: "invalid workplace_type", url: "/api/jobs?workplace_type=nowhere", wantErr: true},
		{name: "invalid min_salary", url: "/api/jobs?min_salary=abc", wantErr: true},
		{name: "invalid max_salary", url: "/api/jobs?max_salary=abc", wantErr: true},
		{
			name: "tags split and trimmed",
			url:  "/api/jobs?tags=go,%20python%20,,rust",
			check: func(t *testing.T, params *database.JobSearchParams) {
				want := []string{"go", "python", "rust"}
				if len(params.Tags) != len(want) {
					t.Fatalf("Tags = %v, want %v", params.Tags, want)
				}
				for i := range want {
					if params.Tags[i] != want[i] {
						t.Errorf("Tags[%d] = %q, want %q", i, params.Tags[i], want[i])
					}
				}
			},
		},
		{
			name: "valid sort date",
			url:  "/api/jobs?sort=date",
			check: func(t *testing.T, params *database.JobSearchParams) {
				if params.Sort != database.SortDate {
					t.Errorf("Sort = %v, want %v", params.Sort, database.SortDate)
				}
			},
		},
		{name: "invalid sort", url: "/api/jobs?sort=random", wantErr: true},
		{
			name: "valid date_posted",
			url:  "/api/jobs?date_posted=week",
			check: func(t *testing.T, params *database.JobSearchParams) {
				if params.PostedAfter == nil {
					t.Fatal("PostedAfter = nil, want non-nil")
				}
				wantAfter := time.Now().Add(-8 * 24 * time.Hour)
				if params.PostedAfter.Before(wantAfter) {
					t.Errorf("PostedAfter = %v, want after %v", params.PostedAfter, wantAfter)
				}
			},
		},
		{name: "invalid date_posted", url: "/api/jobs?date_posted=yesterday", wantErr: true},
		{
			name: "limit capped at max",
			url:  "/api/jobs?limit=9999",
			check: func(t *testing.T, params *database.JobSearchParams) {
				if params.Limit != database.MaxSearchLimit {
					t.Errorf("Limit = %d, want %d", params.Limit, database.MaxSearchLimit)
				}
			},
		},
		{name: "zero limit rejected", url: "/api/jobs?limit=0", wantErr: true},
		{name: "negative limit rejected", url: "/api/jobs?limit=-5", wantErr: true},
		{
			name: "valid offset",
			url:  "/api/jobs?offset=40",
			check: func(t *testing.T, params *database.JobSearchParams) {
				if params.Offset != 40 {
					t.Errorf("Offset = %d, want 40", params.Offset)
				}
			},
		},
		{name: "negative offset rejected", url: "/api/jobs?offset=-1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", tt.url, nil)
			params, err := parseJobSearchParams(r)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.check != nil {
				tt.check(t, params)
			}
		})
	}
}

func seedJob(t *testing.T, url string) int64 {
	t.Helper()
	requireTestDB(t)

	job := jobs.Job{
		Title:    "Backend Engineer",
		Company:  "Acme",
		URL:      url,
		PostedAt: time.Now(),
	}

	if err := database.WriteJobsToDatabase([]jobs.Job{job}); err != nil {
		t.Fatalf("seed job: %v", err)
	}

	result, err := database.SearchForJobs(context.Background(), &database.JobSearchParams{Limit: database.MaxSearchLimit})

	if err != nil {
		t.Fatalf("find seeded job: %v", err)
	}

	for _, j := range result.Jobs {
		if j.URL == url {
			return j.ID
		}
	}

	t.Fatalf("seeded job %q not found in search results", url)
	return 0
}

func TestHandleGetJob(t *testing.T) {
	id := seedJob(t, "https://example.com/jobs/handle-get-job")

	r := httptest.NewRequest("GET", "/api/jobs/"+strconv.FormatInt(id, 10), nil)
	r.SetPathValue("id", strconv.FormatInt(id, 10))
	rec := httptest.NewRecorder()

	handleGetJob(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got jobs.Job
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if got.ID != id {
		t.Errorf("ID = %d, want %d", got.ID, id)
	}

	if got.URL != "https://example.com/jobs/handle-get-job" {
		t.Errorf("URL = %q, want seeded url", got.URL)
	}
}

func TestHandleGetJobNotFound(t *testing.T) {
	requireTestDB(t)

	r := httptest.NewRequest("GET", "/api/jobs/999999999", nil)
	r.SetPathValue("id", "999999999")
	rec := httptest.NewRecorder()

	handleGetJob(rec, r)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleGetJobInvalidID(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/jobs/abc", nil)
	r.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	handleGetJob(rec, r)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
