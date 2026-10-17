package digest

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"main/database"
	"main/jobs"
	"main/llm"
	"main/utils"

	emailverifier "github.com/AfterShip/email-verifier"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func score(v float64) *float64 { return &v }

func TestGoodFitJobs(t *testing.T) {
	candidates := []jobs.Job{
		{Title: "Best", MatchScore: score(1.0)},
		{Title: "Close", MatchScore: score(0.5)},
		{Title: "Edge", MatchScore: score(0.4)},
		{Title: "Weak", MatchScore: score(0.39)},
		{Title: "Unscored"},
	}

	var got []string
	for _, j := range goodFitJobs(candidates) {
		got = append(got, j.Title)
	}

	if strings.Join(got, ",") != "Best,Close,Edge" {
		t.Errorf("goodFitJobs = %v", got)
	}
}

func TestGoodFitJobsNoScores(t *testing.T) {
	if got := goodFitJobs([]jobs.Job{{Title: "A"}, {Title: "B", MatchScore: score(0)}}); got != nil {
		t.Errorf("goodFitJobs = %v, want nil", got)
	}

	if got := goodFitJobs(nil); got != nil {
		t.Errorf("goodFitJobs(nil) = %v", got)
	}
}

func TestSubject(t *testing.T) {
	if got := subject(1); got != "1 new job matches your resume" {
		t.Errorf("subject(1) = %q", got)
	}

	if got := subject(3); got != "3 new jobs match your resume" {
		t.Errorf("subject(3) = %q", got)
	}
}

func TestRenderEmail(t *testing.T) {
	t.Setenv("SECRET_KEY", "test-secret")
	t.Setenv("PUBLIC_BACKEND_URL", "https://api.example.com/")

	body := renderEmail([]jobs.Job{
		{Title: "Go Dev", Company: "Acme", URL: "https://acme.example/1"},
		{Title: "SRE", Company: "Globex", URL: "https://globex.example/2"},
	}, 7)

	for _, want := range []string{
		"Go Dev at Acme\nhttps://acme.example/1\n",
		"SRE at Globex\nhttps://globex.example/2\n",
		"https://api.example.com/api/digest/unsubscribe?profile_id=7&token=" + unsubscribeToken(7),
	} {
		if !strings.Contains(body, want) {
			t.Errorf("email missing %q:\n%s", want, body)
		}
	}
}

func TestUnsubscribeLinkDefaultsToLocalhost(t *testing.T) {
	t.Setenv("SECRET_KEY", "test-secret")
	t.Setenv("PUBLIC_BACKEND_URL", "")

	if got := unsubscribeLink(1); !strings.HasPrefix(got, "http://localhost:8090/api/digest/unsubscribe?profile_id=1&token=") {
		t.Errorf("unsubscribeLink = %q", got)
	}
}

func TestUnsubscribeToken(t *testing.T) {
	t.Setenv("SECRET_KEY", "test-secret")

	token := unsubscribeToken(42)

	if !ValidUnsubscribeToken(42, token) {
		t.Error("own token rejected")
	}

	if ValidUnsubscribeToken(43, token) {
		t.Error("token accepted for another profile")
	}

	if ValidUnsubscribeToken(42, token[:len(token)-1]+"x") || ValidUnsubscribeToken(42, "") {
		t.Error("tampered token accepted")
	}

	if strings.ContainsAny(token, "+/=") {
		t.Errorf("token %q should be URL-safe", token)
	}

	t.Setenv("SECRET_KEY", "rotated")
	if ValidUnsubscribeToken(42, token) {
		t.Error("token survived a SECRET_KEY change")
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
	prevCheck := database.SetEmailVerifier(func(email string) (*emailverifier.Result, error) {
		return &emailverifier.Result{Syntax: emailverifier.Syntax{Valid: true}, HasMxRecords: true}, nil
	})
	t.Cleanup(func() { database.SetEmailVerifier(prevCheck) })

	return testDB
}

type sentEmail struct {
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
}

func fakeResend(t *testing.T) func() []sentEmail {
	t.Helper()

	var (
		mu   sync.Mutex
		sent []sentEmail
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var e sentEmail
		_ = json.NewDecoder(r.Body).Decode(&e)

		mu.Lock()
		sent = append(sent, e)
		mu.Unlock()
	}))
	t.Cleanup(srv.Close)

	t.Setenv("RESEND_API_URL", srv.URL)
	t.Setenv("RESEND_API_KEY", "test-key")
	t.Setenv("SECRET_KEY", "test-secret")

	return func() []sentEmail {
		mu.Lock()
		defer mu.Unlock()
		return append([]sentEmail(nil), sent...)
	}
}

// digestProfile creates an opted-in profile, optionally with an active resume listing skills.
func digestProfile(t *testing.T, email string, skills ...string) int64 {
	t.Helper()
	ctx := t.Context()

	id, err := database.CreateProfile(ctx, &database.ProfileRequest{Email: email, Password: "securepassword123"})
	if err != nil {
		t.Fatal(err)
	}

	if err := database.UpdateProfile(ctx, id, &database.UpdateProfileRequest{EmailNotifications: true}); err != nil {
		t.Fatal(err)
	}

	if len(skills) > 0 {
		resumeID, err := database.AddResume(ctx, id, "r.pdf", "application/pdf", []byte("x"))
		if err != nil {
			t.Fatal(err)
		}

		if err := database.SaveResumeExtractionResult(ctx, resumeID, &llm.ExtractedResume{Skills: skills}); err != nil {
			t.Fatal(err)
		}
	}

	return id
}

func lastSent(t *testing.T, db *sql.DB, id int64) *time.Time {
	t.Helper()

	var at *time.Time
	if err := db.QueryRow("SELECT last_digest_sent_at FROM profiles WHERE id = $1", id).Scan(&at); err != nil {
		t.Fatal(err)
	}

	return at
}

func TestRunCycleSendsMatchingJobs(t *testing.T) {
	db := newTestDB(t)
	sent := fakeResend(t)

	matched := digestProfile(t, "kube@example.com", "Kubernetes")
	noMatches := digestProfile(t, "cobol@example.com", "Fortran")
	noResume := digestProfile(t, "noresume@example.com")

	optedOut, err := database.CreateProfile(t.Context(), &database.ProfileRequest{Email: "out@example.com", Password: "securepassword123"})
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	if err := database.WriteJobsToDatabase(t.Context(), []jobs.Job{
		{Title: "Kubernetes Engineer", Company: "Acme", URL: "https://example.com/kube", Description: "Run Kubernetes clusters", PostedAt: now},
		{Title: "Pastry Chef", Company: "Bakery", URL: "https://example.com/chef", Description: "Bake bread", PostedAt: now},
	}); err != nil {
		t.Fatal(err)
	}

	RunCycle(t.Context())

	emails := sent()
	if len(emails) != 1 {
		t.Fatalf("sent %d emails, want 1: %+v", len(emails), emails)
	}

	e := emails[0]
	if len(e.To) != 1 || e.To[0] != "kube@example.com" || e.Subject != "1 new job matches your resume" {
		t.Errorf("email = %+v", e)
	}

	if !strings.Contains(e.Text, "Kubernetes Engineer at Acme") || strings.Contains(e.Text, "Pastry") {
		t.Errorf("email body = %q", e.Text)
	}

	if !strings.Contains(e.Text, "profile_id="+strconv.FormatInt(matched, 10)) {
		t.Errorf("email lacks unsubscribe link for profile %d", matched)
	}

	if lastSent(t, db, matched) == nil || lastSent(t, db, noMatches) == nil {
		t.Error("profiles with a resume should be marked sent, even with no matches")
	}

	if lastSent(t, db, noResume) != nil || lastSent(t, db, optedOut) != nil {
		t.Error("profiles without a resume or opted out should not be marked")
	}

	// A second run only looks at jobs first seen since the last digest.
	RunCycle(t.Context())

	if n := len(sent()); n != 1 {
		t.Errorf("second run sent %d total emails, want still 1", n)
	}
}

func TestRunCycleEmailFailureSkipsMark(t *testing.T) {
	db := newTestDB(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RESEND_API_URL", srv.URL)
	t.Setenv("RESEND_API_KEY", "test-key")
	t.Setenv("SECRET_KEY", "test-secret")

	id := digestProfile(t, "kube@example.com", "Kubernetes")

	if err := database.WriteJobsToDatabase(t.Context(), []jobs.Job{
		{Title: "Kubernetes Engineer", Company: "Acme", URL: "https://example.com/kube", Description: "Kubernetes", PostedAt: time.Now()},
	}); err != nil {
		t.Fatal(err)
	}

	RunCycle(t.Context())

	if lastSent(t, db, id) != nil {
		t.Error("failed send should leave last_digest_sent_at unset so the next run retries")
	}
}

func TestUnsubscribeLinkParses(t *testing.T) {
	t.Setenv("SECRET_KEY", "test-secret")
	t.Setenv("PUBLIC_BACKEND_URL", "https://api.example.com")

	u, err := url.Parse(unsubscribeLink(9))
	if err != nil {
		t.Fatal(err)
	}

	q := u.Query()
	if q.Get("profile_id") != "9" || !ValidUnsubscribeToken(9, q.Get("token")) {
		t.Errorf("link query = %v", q)
	}
}
