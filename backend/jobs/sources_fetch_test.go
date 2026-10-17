package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// feedServer serves body for every request and records request paths and User-Agents.
func feedServer(t *testing.T, status int, body func(r *http.Request) string) (*httptest.Server, func() []string) {
	t.Helper()

	var (
		mu    sync.Mutex
		paths []string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != "test-agent" {
			t.Errorf("User-Agent = %q, want test-agent", got)
		}

		mu.Lock()
		paths = append(paths, r.URL.RequestURI())
		mu.Unlock()

		w.WriteHeader(status)
		fmt.Fprint(w, body(r))
	}))
	t.Cleanup(srv.Close)

	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		out := slices.Clone(paths)
		slices.Sort(out)
		return out
	}
}

func titles(jobs []Job) []string {
	out := make([]string, len(jobs))
	for i, j := range jobs {
		out[i] = j.Title
	}
	slices.Sort(out)
	return out
}

func TestAshbyFetchJobs(t *testing.T) {
	srv, paths := feedServer(t, http.StatusOK, func(r *http.Request) string {
		if strings.Contains(r.URL.Path, "broken") {
			return "{"
		}
		return `{"jobs":[
			{"title":"Listed","isListed":true,"jobUrl":"https://jobs.ashbyhq.com/a/1"},
			{"title":"Unlisted","isListed":false,"jobUrl":"https://jobs.ashbyhq.com/a/2"},
			{"title":"","isListed":true,"jobUrl":"https://jobs.ashbyhq.com/a/3"},
			{"title":"No URL","isListed":true}
		]}`
	})

	a := NewAshby("test-agent")
	a.Endpoint = srv.URL
	a.Boards = []Board{{"acme", "Acme"}, {"broken", "Broken"}}

	got, err := a.FetchJobs(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 1 || got[0].Title != "Listed" || got[0].Company != "Acme" {
		t.Errorf("jobs = %+v", got)
	}

	want := []string{"/acme?includeCompensation=true", "/broken?includeCompensation=true"}
	if p := paths(); !slices.Equal(p, want) {
		t.Errorf("paths = %v, want %v", p, want)
	}
}

func TestGreenhouseFetchJobs(t *testing.T) {
	srv, paths := feedServer(t, http.StatusOK, func(*http.Request) string {
		return `{"jobs":[
			{"title":"Engineer","absolute_url":"https://boards.greenhouse.io/a/1"},
			{"title":"No URL"},
			{"absolute_url":"https://boards.greenhouse.io/a/3"}
		]}`
	})

	g := NewGreenhouse("test-agent")
	g.Endpoint = srv.URL
	g.Boards = []Board{{"acme", "Acme"}}

	got, err := g.FetchJobs(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 1 || got[0].Company != "Acme" {
		t.Errorf("jobs = %+v", got)
	}

	if p := paths(); !slices.Equal(p, []string{"/acme/jobs?content=true"}) {
		t.Errorf("paths = %v", p)
	}
}

func TestLeverFetchJobs(t *testing.T) {
	srv, paths := feedServer(t, http.StatusOK, func(*http.Request) string {
		return `[
			{"text":"Engineer","hostedUrl":"https://jobs.lever.co/a/1"},
			{"text":"No URL"},
			{"hostedUrl":"https://jobs.lever.co/a/3"}
		]`
	})

	l := NewLever("test-agent")
	l.Endpoint = srv.URL
	l.Boards = []Board{{"acme", "Acme"}}

	got, err := l.FetchJobs(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 1 || got[0].Title != "Engineer" || got[0].Company != "Acme" {
		t.Errorf("jobs = %+v", got)
	}

	if p := paths(); !slices.Equal(p, []string{"/acme?mode=json"}) {
		t.Errorf("paths = %v", p)
	}
}

func TestBoardSourcesErrorWhenEveryBoardFails(t *testing.T) {
	srv, _ := feedServer(t, http.StatusInternalServerError, func(*http.Request) string { return "down" })
	boards := []Board{{"a", "A"}, {"b", "B"}}

	a := NewAshby("test-agent")
	a.Endpoint, a.Boards = srv.URL, boards

	g := NewGreenhouse("test-agent")
	g.Endpoint, g.Boards = srv.URL, boards

	l := NewLever("test-agent")
	l.Endpoint, l.Boards = srv.URL, boards

	for name, src := range map[string]JobSource{"ashby": a, "greenhouse": g, "lever": l} {
		if _, err := src.FetchJobs(t.Context()); err == nil || !strings.Contains(err.Error(), "500") {
			t.Errorf("%s: err = %v, want status error", name, err)
		}
	}
}

func TestDefaultBoardListsAreValid(t *testing.T) {
	lists := map[string][]Board{
		"ashby":      NewAshby("ua").Boards,
		"greenhouse": NewGreenhouse("ua").Boards,
		"lever":      NewLever("ua").Boards,
	}

	for name, boards := range lists {
		if len(boards) == 0 {
			t.Errorf("%s has no boards", name)
		}

		seen := map[string]bool{}
		for _, b := range boards {
			if b.Slug == "" || b.Company == "" {
				t.Errorf("%s board %+v missing slug or company", name, b)
			}
			if seen[b.Slug] {
				t.Errorf("%s board %q listed twice", name, b.Slug)
			}
			seen[b.Slug] = true
		}
	}
}

func TestSingleFeedSources(t *testing.T) {
	tests := []struct {
		name string
		src  func(endpoint string) JobSource
		body string
		want []string
	}{
		{
			name: "remoteok skips the legal notice row",
			src: func(e string) JobSource {
				r := NewRemoteOK("test-agent")
				r.Endpoint = e
				return r
			},
			body: `[{"legal":"notice"},{"id":"1","position":"Go Dev","url":"https://remoteok.com/1"},{"id":"2"}]`,
			want: []string{"Go Dev"},
		},
		{
			name: "remotive",
			src: func(e string) JobSource {
				r := NewRemotive("test-agent")
				r.Endpoint = e
				return r
			},
			body: `{"jobs":[{"id":1,"title":"Designer","url":"https://remotive.com/1"},{"id":0,"title":"No ID"},{"id":3}]}`,
			want: []string{"Designer"},
		},
		{
			name: "jobicy",
			src: func(e string) JobSource {
				j := NewJobicy("test-agent")
				j.Endpoint = e
				return j
			},
			body: `{"jobs":[{"id":1,"jobTitle":"PM","url":"https://jobicy.com/1"},{"id":2,"jobTitle":""},{"jobTitle":"No ID"}]}`,
			want: []string{"PM"},
		},
		{
			name: "weworkremotely xml",
			src: func(e string) JobSource {
				w := NewWeWorkRemotely("test-agent")
				w.Endpoint = e
				return w
			},
			body: `<?xml version="1.0"?><rss><channel>
				<item><title>Acme: SRE</title><link>https://weworkremotely.com/1</link></item>
				<item><title>No link</title></item>
				<item><link>https://weworkremotely.com/3</link></item>
			</channel></rss>`,
			want: []string{"SRE"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := feedServer(t, http.StatusOK, func(*http.Request) string { return tt.body })

			got, err := tt.src(srv.URL).FetchJobs(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			if g := titles(got); !slices.Equal(g, tt.want) {
				t.Errorf("titles = %v, want %v", g, tt.want)
			}
		})
	}
}

func TestSingleFeedSourcesErrors(t *testing.T) {
	makers := map[string]func(string) JobSource{
		"remoteok":       func(e string) JobSource { s := NewRemoteOK("test-agent"); s.Endpoint = e; return s },
		"remotive":       func(e string) JobSource { s := NewRemotive("test-agent"); s.Endpoint = e; return s },
		"jobicy":         func(e string) JobSource { s := NewJobicy("test-agent"); s.Endpoint = e; return s },
		"weworkremotely": func(e string) JobSource { s := NewWeWorkRemotely("test-agent"); s.Endpoint = e; return s },
		"arbeitnow":      func(e string) JobSource { s := NewArbeitnow("test-agent"); s.Endpoint = e; return s },
		"himalayas":      func(e string) JobSource { s := NewHimalayas("test-agent"); s.Endpoint = e; return s },
	}

	badStatus, _ := feedServer(t, http.StatusServiceUnavailable, func(*http.Request) string { return "" })
	badBody, _ := feedServer(t, http.StatusOK, func(*http.Request) string { return "<<<{{{" })

	for name, mk := range makers {
		if _, err := mk(badStatus.URL).FetchJobs(t.Context()); err == nil {
			t.Errorf("%s: 503 should error", name)
		}

		if _, err := mk(badBody.URL).FetchJobs(t.Context()); err == nil {
			t.Errorf("%s: malformed body should error", name)
		}

		if _, err := mk("://bad-url").FetchJobs(t.Context()); err == nil {
			t.Errorf("%s: bad url should error", name)
		}
	}
}

func TestFeedRespectsContextCancel(t *testing.T) {
	srv, _ := feedServer(t, http.StatusOK, func(r *http.Request) string {
		<-r.Context().Done()
		return ""
	})

	r := NewRemotive("test-agent")
	r.Endpoint = srv.URL

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()

	if _, err := r.FetchJobs(ctx); err == nil {
		t.Error("want error after context deadline")
	}
}

func TestArbeitnowKeepsEarlierPagesOnFailure(t *testing.T) {
	srv, _ := feedServer(t, http.StatusOK, func(r *http.Request) string {
		if r.URL.Query().Get("page") == "2" {
			return "{"
		}
		return `{"data":[{"slug":"a","title":"First"},{"slug":"","title":"Skipped"}],"links":{"next":"more"}}`
	})

	a := NewArbeitnow("test-agent")
	a.Endpoint = srv.URL

	got, err := a.FetchJobs(t.Context())
	if err != nil {
		t.Fatalf("err = %v, want earlier pages kept", err)
	}

	if g := titles(got); !slices.Equal(g, []string{"First"}) {
		t.Errorf("titles = %v", g)
	}
}

func TestArbeitnowStopsOnEmptyPage(t *testing.T) {
	calls := 0
	srv, _ := feedServer(t, http.StatusOK, func(*http.Request) string {
		calls++
		return `{"data":[],"links":{"next":"more"}}`
	})

	a := NewArbeitnow("test-agent")
	a.Endpoint = srv.URL

	if _, err := a.FetchJobs(t.Context()); err != nil {
		t.Fatal(err)
	}

	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestArbeitnowJobTypesUnmarshal(t *testing.T) {
	var asSlice arbeitnowJobTypes
	if err := json.Unmarshal([]byte(`["full time","remote"]`), &asSlice); err != nil || !slices.Equal(asSlice, arbeitnowJobTypes{"full time", "remote"}) {
		t.Errorf("slice form = %v, %v", asSlice, err)
	}

	var asMap arbeitnowJobTypes
	if err := json.Unmarshal([]byte(`{"0":"contract","1":"part time"}`), &asMap); err != nil {
		t.Fatal(err)
	}
	got := slices.Clone(asMap)
	slices.Sort(got)
	if !slices.Equal(got, []string{"contract", "part time"}) {
		t.Errorf("map form = %v", asMap)
	}

	var bad arbeitnowJobTypes
	if err := json.Unmarshal([]byte(`42`), &bad); err == nil {
		t.Error("number form should error")
	}
}

func TestArbeitnowJobToJob(t *testing.T) {
	raw := arbeitnowJob{
		Slug:        "go-dev",
		CompanyName: "Acme GmbH",
		Title:       "Go Developer",
		Description: "<p>Write <b>Go</b></p>",
		Remote:      true,
		URL:         "https://www.arbeitnow.com/jobs/go-dev",
		Tags:        []string{"Go", " "},
		JobTypes:    arbeitnowJobTypes{"full time"},
		Location:    "Berlin",
		CreatedAt:   1700000000,
	}

	want := Job{
		Title:         "Go Developer",
		Company:       "Acme GmbH",
		Location:      "Berlin",
		WorkplaceType: Remote,
		Tags:          []string{"Go", "full time"},
		URL:           "https://www.arbeitnow.com/jobs/go-dev",
		Description:   "Write Go",
		PostedAt:      time.Unix(1700000000, 0),
	}

	assertJobEqual(t, raw.toJob(), want)

	raw.Remote, raw.CreatedAt = false, 0
	got := raw.toJob()
	if got.WorkplaceType != Unknown || !got.PostedAt.IsZero() {
		t.Errorf("onsite/no date job = %+v", got)
	}
}
