package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"main/database"
	"main/jobs"
	"main/llm"
)

type searchResponse struct {
	Jobs   []jobs.Job `json:"jobs"`
	Total  int        `json:"total"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
}

// seedUniqueJobs writes jobs tagged with a word no other test uses, returning their ids by title.
func seedUniqueJobs(t *testing.T, word string, batch ...jobs.Job) map[string]int64 {
	t.Helper()

	for i := range batch {
		batch[i].Title = word + " " + batch[i].Title
		batch[i].URL = fmt.Sprintf("https://example.com/%s/%d", word, i)
		batch[i].Tags = append(batch[i].Tags, word+"tag")
		if batch[i].PostedAt.IsZero() {
			batch[i].PostedAt = time.Now()
		}
	}

	if err := database.WriteJobsToDatabase(t.Context(), batch); err != nil {
		t.Fatal(err)
	}

	ids := map[string]int64{}
	for _, j := range batch {
		var id int64
		if err := database.DB().QueryRow("SELECT id FROM jobs WHERE url = $1", j.URL).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids[strings.TrimPrefix(j.Title, word+" ")] = id
	}

	return ids
}

func giveActiveResume(t *testing.T, profileID int64, extracted *llm.ExtractedResume) {
	t.Helper()

	resumeID, err := database.AddResume(t.Context(), profileID, "r.pdf", "application/pdf", []byte("x"))
	if err != nil {
		t.Fatal(err)
	}

	if err := database.SaveResumeExtractionResult(t.Context(), resumeID, extracted); err != nil {
		t.Fatal(err)
	}
}

func TestAPISearchJobs(t *testing.T) {
	a := newAPI(t)
	salaryMin, salaryMax := 150000, 180000
	seedUniqueJobs(t, "Qwzx",
		jobs.Job{Title: "Remote Dev", Company: "Acme", WorkplaceType: jobs.Remote, SalaryMin: &salaryMin, SalaryMax: &salaryMax},
		jobs.Job{Title: "Office Dev", Company: "Acme", WorkplaceType: jobs.InPerson},
		jobs.Job{Title: "Intern", Company: "Acme", WorkplaceType: jobs.Remote},
	)

	var resp searchResponse
	a.must(http.StatusOK, "GET", "/api/jobs?q=Qwzx", nil, &resp)
	if resp.Total != 3 || len(resp.Jobs) != 3 || resp.Limit != database.DefaultSearchLimit {
		t.Fatalf("search = %+v", resp)
	}

	for _, j := range resp.Jobs {
		if j.MatchScore != nil || j.Applied || j.Description != "" {
			t.Errorf("anonymous result %+v should have no score, applied flag, or description", j)
		}
	}

	a.must(http.StatusOK, "GET", "/api/jobs?q=Qwzx&workplace_type=remote", nil, &resp)
	if resp.Total != 2 {
		t.Errorf("remote filter total = %d", resp.Total)
	}

	a.must(http.StatusOK, "GET", "/api/jobs?q=Qwzx&job_type=intern", nil, &resp)
	if resp.Total != 1 {
		t.Errorf("intern filter total = %d", resp.Total)
	}

	a.must(http.StatusOK, "GET", "/api/jobs?q=Qwzx&min_salary=100000", nil, &resp)
	if resp.Total != 1 {
		t.Errorf("salary filter total = %d", resp.Total)
	}

	a.must(http.StatusOK, "GET", "/api/jobs?tags=Qwzxtag&limit=2&offset=2", nil, &resp)
	if resp.Total != 3 || len(resp.Jobs) != 1 || resp.Offset != 2 {
		t.Errorf("paged tag search = %+v", resp)
	}

	a.must(http.StatusOK, "GET", "/api/jobs?q=nothingmatchesthisatall", nil, &resp)
	if resp.Jobs == nil || len(resp.Jobs) != 0 {
		t.Errorf("empty search should return [] not null: %+v", resp)
	}

	a.must(http.StatusBadRequest, "GET", "/api/jobs?workplace_type=moon", nil, nil)
	a.must(http.StatusBadRequest, "GET", "/api/jobs?limit=-1", nil, nil)

	req, _ := http.NewRequest("GET", a.srv.URL+"/api/jobs?q=Qwzx", nil)
	if _, _, h := a.send(req); !strings.Contains(h.Get("Server-Timing"), "search;dur=") {
		t.Errorf("Server-Timing = %q", h.Get("Server-Timing"))
	}
}

func TestAPISearchJobsWithResume(t *testing.T) {
	a := newAPI(t).signup("api-search-resume@example.com")
	giveActiveResume(t, a.profile().ID, &llm.ExtractedResume{Skills: []string{"Vbnmq"}})

	ids := seedUniqueJobs(t, "Vbnmq", jobs.Job{Title: "Engineer", Company: "Acme", Description: "Vbnmq Vbnmq Vbnmq"})

	var resp searchResponse
	a.must(http.StatusOK, "GET", "/api/jobs?q=Vbnmq", nil, &resp)
	if len(resp.Jobs) != 1 || resp.Jobs[0].MatchScore == nil {
		t.Fatalf("signed-in search should score against the resume: %+v", resp.Jobs)
	}

	// A bad token on an optional-auth route is just anonymous.
	anon := a.as()
	anon.token = "garbage"
	var anonResp searchResponse
	anon.must(http.StatusOK, "GET", "/api/jobs?q=Vbnmq", nil, &anonResp)
	if len(anonResp.Jobs) != 1 || anonResp.Jobs[0].MatchScore != nil {
		t.Error("invalid token still got a match score")
	}

	var job jobs.Job
	a.must(http.StatusOK, "GET", fmt.Sprintf("/api/jobs/%d", ids["Engineer"]), nil, &job)
	if job.MatchScore == nil || job.Description == "" {
		t.Errorf("detail = %+v", job)
	}
}

func TestAPIApplyToggle(t *testing.T) {
	a := newAPI(t).signup("api-apply@example.com")
	ids := seedUniqueJobs(t, "Plokm", jobs.Job{Title: "Dev", Company: "Acme"})
	path := fmt.Sprintf("/api/jobs/%d", ids["Dev"])

	a.must(http.StatusOK, "POST", path+"/apply", nil, nil)

	var job jobs.Job
	a.must(http.StatusOK, "GET", path, nil, &job)
	if !job.Applied {
		t.Error("job not applied")
	}

	// Another profile's view is unaffected.
	other := a.as().signup("api-apply-other@example.com")
	other.must(http.StatusOK, "GET", path, nil, &job)
	if job.Applied {
		t.Error("applied leaked across profiles")
	}

	a.must(http.StatusOK, "DELETE", path+"/apply", nil, nil)
	a.must(http.StatusOK, "GET", path, nil, &job)
	if job.Applied {
		t.Error("job still applied")
	}

	a.must(http.StatusBadRequest, "POST", "/api/jobs/x/apply", nil, nil)
	a.must(http.StatusBadRequest, "DELETE", "/api/jobs/x/apply", nil, nil)
	a.as().must(http.StatusUnauthorized, "POST", path+"/apply", nil, nil)
}

func TestAPIGenerateApplicationContent(t *testing.T) {
	a := newAPI(t).signup("api-generate@example.com")
	giveActiveResume(t, a.profile().ID, &llm.ExtractedResume{FullName: "Ada", Skills: []string{"Go"}})
	ids := seedUniqueJobs(t, "Mnbvc", jobs.Job{Title: "Go Dev", Company: "Acme", Description: "Write Go"})
	path := fmt.Sprintf("/api/jobs/%d/generate", ids["Go Dev"])

	calls := fakeLLM(t, static(`{"cover_letter":"Dear Acme","tailored_bullets":["Wrote Go"]}`))

	var got llm.GeneratedContent
	a.must(http.StatusOK, "POST", path, nil, &got)
	if got.CoverLetter != "Dear Acme" || len(got.TailoredBullets) != 1 {
		t.Errorf("generated = %+v", got)
	}

	if prompt := calls.prompt(); !strings.Contains(prompt, "Mnbvc Go Dev") || !strings.Contains(prompt, "Ada") {
		t.Errorf("prompt missing job or resume: %q", prompt)
	}

	fakeLLM(t, static(""))
	a.must(http.StatusBadGateway, "POST", path, nil, nil)
}

func TestAPITailoredResume(t *testing.T) {
	a := newAPI(t).signup("api-tailor@example.com")
	giveActiveResume(t, a.profile().ID, &llm.ExtractedResume{
		FullName:       "Ada",
		Skills:         []string{"Go"},
		WorkExperience: []llm.WorkExperienceEntry{{Company: "Acme", JobTitle: "Dev", Bullets: []string{"Built APIs"}}},
	})
	ids := seedUniqueJobs(t, "Lkjhg", jobs.Job{Title: "Go Dev", Company: "Globex"})
	path := fmt.Sprintf("/api/jobs/%d/tailored-resume", ids["Go Dev"])

	a.must(http.StatusNotFound, "GET", path, nil, nil)

	fakeLLM(t, static(`{"summary":"Go engineer","skills":["Go"],
		"work_experience":[{"source":"W0","bullets":[{"text":"Built Go APIs","source":"W0.B0"}]}],"projects":[]}`))

	var tailored database.TailoredResume
	a.must(http.StatusOK, "POST", path, nil, &tailored)
	if tailored.Content.Summary != "Go engineer" || tailored.Stale || len(tailored.Content.WorkExperience) != 1 ||
		tailored.Content.WorkExperience[0].Company != "Acme" {
		t.Fatalf("tailored = %+v", tailored)
	}

	a.must(http.StatusOK, "GET", path, nil, &tailored)
	if tailored.Content.FullName != "Ada" {
		t.Errorf("stored tailored = %+v", tailored)
	}

	fakeLLM(t, static("not json"))
	a.must(http.StatusBadGateway, "POST", path, nil, nil)
}

func TestAPITagsHealthAndCORS(t *testing.T) {
	a := newAPI(t)
	seedUniqueJobs(t, "Tgrfv", jobs.Job{Title: "Dev", Company: "Acme"})

	req, _ := http.NewRequest("GET", a.srv.URL+"/api/tags", nil)
	status, body, header := a.send(req)
	if status != http.StatusOK || !strings.Contains(string(body), `"Tgrfvtag"`) || !strings.Contains(header.Get("Cache-Control"), "max-age=300") {
		t.Errorf("tags = %d %s %q", status, body, header.Get("Cache-Control"))
	}

	var health map[string]string
	a.must(http.StatusOK, "GET", "/api/health", nil, &health)
	if health["status"] != "ok" {
		t.Errorf("health = %v", health)
	}

	req, _ = http.NewRequest("OPTIONS", a.srv.URL+"/api/profile", nil)
	status, _, header = a.send(req)
	if status != http.StatusNoContent || header.Get("Access-Control-Allow-Origin") != "http://localhost:5173" ||
		header.Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("preflight = %d %v", status, header)
	}

	a.must(http.StatusNotFound, "GET", "/api/nope", nil, nil)
	a.must(http.StatusMethodNotAllowed, "PATCH", "/api/profile", nil, nil)
}

func TestHandleHealthDatabaseDown(t *testing.T) {
	unreachable, err := sql.Open("pgx", "postgres://unreachable.invalid:5432/none?connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}

	prev := database.SetDB(unreachable)
	t.Cleanup(func() { database.SetDB(prev) })

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	rec := httptest.NewRecorder()
	handleHealth(rec, httptest.NewRequest("GET", "/api/health", nil).WithContext(ctx))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

func unsubscribeTokenFor(profileID int64) string {
	mac := hmac.New(sha256.New, []byte("api-test-secret"))
	mac.Write([]byte(strconv.FormatInt(profileID, 10)))

	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func TestAPIUnsubscribeDigest(t *testing.T) {
	a := newAPI(t).signup("api-unsub@example.com")
	a.must(http.StatusOK, "PUT", "/api/profile", map[string]any{"email_notifications": true}, nil)
	id := a.profile().ID

	anon := a.as()
	anon.must(http.StatusBadRequest, "GET", "/api/digest/unsubscribe?profile_id=abc&token=x", nil, nil)
	anon.must(http.StatusUnauthorized, "GET", fmt.Sprintf("/api/digest/unsubscribe?profile_id=%d&token=forged", id), nil, nil)
	anon.must(http.StatusUnauthorized, "GET", fmt.Sprintf("/api/digest/unsubscribe?profile_id=%d&token=%s", id+1, unsubscribeTokenFor(id)), nil, nil)

	if !a.profile().EmailNotifications {
		t.Fatal("failed unsubscribes changed the setting")
	}

	body := anon.must(http.StatusOK, "GET", fmt.Sprintf("/api/digest/unsubscribe?profile_id=%d&token=%s", id, unsubscribeTokenFor(id)), nil, nil)
	if !strings.Contains(string(body), "unsubscribed") {
		t.Errorf("body = %s", body)
	}

	if a.profile().EmailNotifications {
		t.Error("still subscribed")
	}
}
