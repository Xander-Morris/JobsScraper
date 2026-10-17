package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
)

// careerSite serves a fake career site under /careers and records each path requested.
type careerSite struct {
	*httptest.Server
	mu        sync.Mutex
	requested []string
	robots    string
	sitemap   string
}

func newCareerSite(t *testing.T) *careerSite {
	t.Helper()

	site := &careerSite{}

	site.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		site.mu.Lock()
		site.requested = append(site.requested, r.URL.RequestURI())
		site.mu.Unlock()

		switch {
		case r.URL.Path == "/robots.txt":
			if site.robots == "" {
				http.NotFound(w, r)
				return
			}
			fmt.Fprint(w, strings.ReplaceAll(site.robots, "{{base}}", site.URL))
		case r.URL.Path == "/careers/sitemap.xml" && site.sitemap != "":
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, strings.ReplaceAll(site.sitemap, "{{base}}", site.URL))
		case r.URL.Path == "/careers" || r.URL.Path == "/careers/page2":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<html><body>
				<a href="/careers/jobs/a">A</a>
				<a href="/careers/jobs/b">B</a>
				<a href="/careers/page2">More</a>
				<a href="/careers/private/x">Private</a>
				<a href="/about">About</a>
			</body></html>`)
		case strings.HasPrefix(r.URL.Path, "/careers/jobs/"):
			id := strings.TrimPrefix(r.URL.Path, "/careers/jobs/")
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<html><head><script type="application/ld+json">
				{"@context":"https://schema.org","@type":"JobPosting","title":"Job %s","hiringOrganization":{"name":"Acme Corp"}}
			</script></head><body><a href="/careers/jobs/never-followed">x</a></body></html>`, id)
		default:
			http.NotFound(w, r)
		}
	}))

	t.Cleanup(site.Close)

	return site
}

func (s *careerSite) paths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.Clone(s.requested)
}

func (s *careerSite) crawler(known KnownFunc) *Crawler {
	return &Crawler{
		HTTPClient:      s.Client(),
		UserAgent:       "JobsScraper/1.0",
		Seeds:           []Seed{{URL: s.URL + "/careers", Company: "Acme"}},
		Known:           known,
		MaxPagesPerSeed: 20,
		Timeout:         defaultTimeout,
	}
}

func jobTitles(t *testing.T, c *Crawler) []string {
	t.Helper()

	found, err := c.FetchJobs(t.Context())
	if err != nil {
		t.Fatalf("FetchJobs: %v", err)
	}

	var titles []string
	for _, job := range found {
		if job.Company != "Acme" {
			t.Errorf("Company = %q, want the seed's override", job.Company)
		}
		titles = append(titles, job.Title)
	}

	slices.Sort(titles)

	return titles
}

func TestCrawlFollowsLinksWithinScope(t *testing.T) {
	site := newCareerSite(t)
	site.robots = "User-agent: *\nDisallow: /careers/private\n"

	if got, want := jobTitles(t, site.crawler(nil)), []string{"Job a", "Job b"}; !slices.Equal(got, want) {
		t.Errorf("titles = %v, want %v", got, want)
	}

	for _, path := range site.paths() {
		if path == "/about" || strings.HasPrefix(path, "/careers/private") || path == "/careers/jobs/never-followed" {
			t.Errorf("fetched %s, want it skipped (out of scope, disallowed, or linked from a job page)", path)
		}
	}
}

func TestCrawlReadsSitemapNewestFirst(t *testing.T) {
	site := newCareerSite(t)
	site.robots = "Sitemap: {{base}}/careers/sitemap.xml\nSitemap: https://elsewhere.example/sitemap.xml\n"
	site.sitemap = `<urlset>
		<url><loc>{{base}}/careers/jobs/old</loc><lastmod>2026-01-01</lastmod></url>
		<url><loc>{{base}}/careers/jobs/new</loc><lastmod>2026-09-01</lastmod></url>
		<url><loc>{{base}}/blog/post</loc></url>
	</urlset>`

	c := site.crawler(nil)
	c.MaxPagesPerSeed = 2

	if got, want := jobTitles(t, c), []string{"Job new", "Job old"}; !slices.Equal(got, want) {
		t.Errorf("titles = %v, want %v", got, want)
	}

	want := []string{"/robots.txt", "/careers/sitemap.xml", "/careers/jobs/new", "/careers/jobs/old"}
	if got := site.paths(); !slices.Equal(got, want) {
		t.Errorf("requests = %v, want %v (sitemap pages first, start page past the budget)", got, want)
	}
}

func TestCrawlSkipsKnownURLs(t *testing.T) {
	site := newCareerSite(t)

	known := func(_ context.Context, urls []string) (map[string]bool, error) {
		result := make(map[string]bool)
		for _, u := range urls {
			result[u] = strings.HasSuffix(u, "/jobs/a")
		}
		return result, nil
	}

	if got, want := jobTitles(t, site.crawler(known)), []string{"Job b"}; !slices.Equal(got, want) {
		t.Errorf("titles = %v, want %v", got, want)
	}

	if slices.Contains(site.paths(), "/careers/jobs/a") {
		t.Error("fetched a known job page")
	}
}

func TestCrawlStopsAtPageBudget(t *testing.T) {
	site := newCareerSite(t)

	c := site.crawler(nil)
	c.MaxPagesPerSeed = 2

	if got, want := jobTitles(t, c), []string{"Job a"}; !slices.Equal(got, want) {
		t.Errorf("titles = %v, want %v (start page plus one job page)", got, want)
	}
}

func TestCrawlSkipsSeedWhenRobotsUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := &Crawler{
		HTTPClient:      srv.Client(),
		UserAgent:       "JobsScraper/1.0",
		Seeds:           []Seed{{URL: srv.URL + "/careers"}},
		MaxPagesPerSeed: 5,
		Timeout:         defaultTimeout,
	}

	if _, err := c.FetchJobs(t.Context()); err == nil {
		t.Error("err = nil, want an error when robots.txt is unreachable for every seed")
	}
}

func TestCrawlRespectsDisallowedSeed(t *testing.T) {
	site := newCareerSite(t)
	site.robots = "User-agent: jobsscraper\nDisallow: /careers\n"

	if _, err := site.crawler(nil).FetchJobs(t.Context()); err == nil {
		t.Error("err = nil, want an error when robots.txt disallows the only seed")
	}

	if got := site.paths(); !slices.Equal(got, []string{"/robots.txt"}) {
		t.Errorf("requests = %v, want only robots.txt", got)
	}
}
