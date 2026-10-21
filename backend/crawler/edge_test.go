package crawler

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

// hangUp drops the connection without a response, so the client's request fails.
func hangUp(w http.ResponseWriter, _ *http.Request) {
	conn, _, err := w.(http.Hijacker).Hijack()
	if err == nil {
		_ = conn.Close()
	}
}

// truncate promises more body than it sends, so reading the body fails.
func truncate(w http.ResponseWriter, _ *http.Request) {
	conn, buf, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return
	}
	fmt.Fprint(buf, "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nContent-Length: 100\r\n\r\n<html>")
	_ = buf.Flush()
	_ = conn.Close()
}

// brokenSite serves every way a page or sitemap can go wrong.
func brokenSite(t *testing.T) *httptest.Server {
	t.Helper()

	var base string
	html := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, strings.ReplaceAll(body, "{{base}}", base))
		}
	}
	xml := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, strings.ReplaceAll(body, "{{base}}", base))
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, strings.ReplaceAll("User-agent: *\nCrawl-delay: 0.001\n"+
			"Sitemap: {{base}}/index.xml\nSitemap: {{base}}/index.xml\nSitemap: {{base}}/missing.xml\nSitemap: {{base}}/drop.xml\n"+
			"Sitemap: http://other.example/sitemap.xml\n", "{{base}}", base))
	})
	mux.HandleFunc("/index.xml", xml(`<sitemapindex>
		<sitemap><loc>{{base}}/jobs.xml</loc></sitemap>
		<sitemap><loc>{{base}}/index.xml</loc></sitemap>
		<sitemap><loc>http://other.example/jobs.xml</loc></sitemap>
	</sitemapindex>`))
	mux.HandleFunc("/jobs.xml", xml(`<urlset><url><loc>{{base}}/careers/jobs/1</loc></url></urlset>`))
	mux.HandleFunc("/drop.xml", hangUp)
	mux.HandleFunc("/careers", html(`<a href="/careers/json">j</a><a href="/careers/redirect">r</a>
		<a href="/careers/trunc">t</a><a href="/careers/empty">e</a><a href="/careers/drop">d</a>
		<a href="/careers/search?q=go">q</a><a href="/careers/excluded">x</a><a href="http://other.example/careers/x">o</a>`))
	mux.HandleFunc("/careers/jobs/1", html(`<script type="application/ld+json">{"@type":"JobPosting","title":"Sitemap Job"}</script>`))
	mux.HandleFunc("/careers/json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	})
	mux.HandleFunc("/careers/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/about", http.StatusFound)
	})
	mux.HandleFunc("/about", html(`about`))
	mux.HandleFunc("/careers/trunc", truncate)
	mux.HandleFunc("/careers/empty", func(w http.ResponseWriter, _ *http.Request) { w.Header().Set("Content-Type", "text/html") })
	mux.HandleFunc("/careers/drop", hangUp)
	mux.HandleFunc("/careers/search", html(`results`))

	srv := httptest.NewServer(mux)
	base = srv.URL
	t.Cleanup(srv.Close)

	return srv
}

func TestCrawlSurvivesBrokenPagesAndSitemaps(t *testing.T) {
	srv := brokenSite(t)

	c := &Crawler{
		HTTPClient: srv.Client(),
		UserAgent:  "JobsScraper/1.0",
		Seeds: []Seed{{
			URL:     srv.URL + "/careers",
			Company: "Acme",
			Include: regexp.MustCompile(`/careers(/(jobs|json|redirect|trunc|empty|drop|search).*)?$`),
		}},
		Known:           func(context.Context, []string) (map[string]bool, error) { return nil, errors.New("db down") },
		MaxPagesPerSeed: 20,
		Timeout:         time.Minute,
	}

	got, err := c.FetchJobs(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "Sitemap Job" {
		t.Errorf("jobs = %+v, want only the sitemap job", got)
	}
}

func TestCrawlSeedAtHostRoot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html>home</html>`)
	}))
	t.Cleanup(srv.Close)

	c := &Crawler{HTTPClient: srv.Client(), UserAgent: "JobsScraper/1.0", Seeds: []Seed{{URL: srv.URL}}, MaxPagesPerSeed: 2, Timeout: time.Minute}
	if _, err := c.FetchJobs(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestCrawlSeedErrors(t *testing.T) {
	for name, c := range map[string]*Crawler{
		"bad url":           {Seeds: []Seed{{URL: "http://[::1"}}},
		"no host":           {Seeds: []Seed{{URL: "/careers"}}},
		"unreachable":       {HTTPClient: http.DefaultClient, Seeds: []Seed{{URL: "http://127.0.0.1:1/careers"}}},
		"panics (nil http)": {Seeds: []Seed{{URL: "http://example.invalid/careers"}}},
	} {
		c.UserAgent, c.Timeout, c.MaxPagesPerSeed = "JobsScraper/1.0", time.Minute, 1
		if _, err := c.FetchJobs(t.Context()); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestCrawlStopsWaitingPastDeadline(t *testing.T) {
	site := newCareerSite(t)
	c := site.crawler(nil)
	c.MinDelay = time.Hour
	c.Timeout = 2 * time.Second

	if _, err := c.FetchJobs(t.Context()); err != nil {
		t.Fatal(err)
	}
	// Only robots.txt fits before the deadline; every later wait gives up at once.
	if got := site.paths(); !slices.Equal(got, []string{"/robots.txt"}) {
		t.Errorf("requests = %v", got)
	}
}

func TestParseSitemapErrors(t *testing.T) {
	if _, err := parseSitemap(bytes.NewReader([]byte{0x1f, 0x8b, 'x'})); err == nil {
		t.Error("bad gzip: want error")
	}
	if _, err := parseSitemap(strings.NewReader("<urlset><url>")); err == nil {
		t.Error("bad xml: want error")
	}

	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	_, _ = zw.Write([]byte(`<urlset><url><loc>https://a.dev/1</loc></url></urlset>`))
	_ = zw.Close()
	if got, err := parseSitemap(&gz); err != nil || len(got.URLs) != 1 {
		t.Errorf("gzip urlset = %+v, %v", got, err)
	}

	latin := `<?xml version="1.0" encoding="ISO-8859-1"?><urlset><url><loc> https://a.dev/2 </loc></url></urlset>`
	if got, err := parseSitemap(strings.NewReader(latin)); err != nil || got.URLs[0].Loc != "https://a.dev/2" {
		t.Errorf("latin-1 urlset = %+v, %v", got, err)
	}
}

func TestParsePageEdges(t *testing.T) {
	base, _ := url.Parse("https://a.dev/careers")

	if _, err := parsePage(iotest.ErrReader(errors.New("boom")), base); err == nil {
		t.Error("read error: want error")
	}

	got, err := parsePage(strings.NewReader(`<script>var x</script><a>no href</a><a href="http://[::1">bad</a><a href="/ok">ok</a>`), base)
	if err != nil || !slices.Equal(got.links, []string{"https://a.dev/ok"}) || got.postings != nil {
		t.Errorf("page = %+v, %v", got, err)
	}
}

func TestTextsShapes(t *testing.T) {
	for raw, want := range map[string][]string{
		`"  "`:           nil,
		`42`:             {"42"},
		`true`:           nil,
		`{}`:             nil,
		`{"@value":"x"}`: {"x"},
	} {
		if got := texts(json.RawMessage(raw)); !slices.Equal(got, want) {
			t.Errorf("texts(%s) = %v, want %v", raw, got, want)
		}
	}
}

func TestPlacesAndAddresses(t *testing.T) {
	if got := placesText(json.RawMessage(`[{"name":"HQ"},{"address":" Austin "}]`)); got != "HQ; Austin" {
		t.Errorf("placesText = %q", got)
	}
	if got := addressText(json.RawMessage(`5`)); got != "" {
		t.Errorf("addressText(5) = %q", got)
	}
}

func TestNewUsesDefaults(t *testing.T) {
	c := New("JobsScraper/1.0", nil)
	if c.MaxPagesPerSeed != defaultMaxPagesPerSeed || c.MinDelay != defaultMinDelay || len(c.Seeds) == 0 {
		t.Errorf("New() = %+v", c)
	}
}
