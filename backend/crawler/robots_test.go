package crawler

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestParseRobotsPrefersNamedGroup(t *testing.T) {
	r := parseRobots(strings.NewReader(`
User-agent: *
Disallow: /

User-agent: OtherBot
User-agent: JobsScraper
Disallow: /private
Crawl-delay: 2.5

Sitemap: https://example.com/sitemap.xml
`), "jobsscraper")

	if !r.allowed("/jobs/1") {
		t.Error("/jobs/1 disallowed, want the named group's rules to replace *'s")
	}

	if r.allowed("/private/x") {
		t.Error("/private/x allowed, want disallowed")
	}

	if r.crawlDelay != 2500*time.Millisecond {
		t.Errorf("crawlDelay = %v, want 2.5s", r.crawlDelay)
	}

	if want := []string{"https://example.com/sitemap.xml"}; !slices.Equal(r.sitemaps, want) {
		t.Errorf("sitemaps = %v, want %v", r.sitemaps, want)
	}
}

func TestParseRobotsAgentMatchIsExact(t *testing.T) {
	r := parseRobots(strings.NewReader("User-agent: github\nDisallow: /\n"), "jobsscraper")

	if !r.allowed("/anything") {
		t.Error("disallowed by another agent's group")
	}
}

func TestRobotsAllowed(t *testing.T) {
	r := parseRobots(strings.NewReader(`
User-agent: *
Disallow: /search
Allow: /search/jobs
Disallow: /*.pdf$
Disallow: /tmp*/cache
Disallow:
`), "jobsscraper")

	tests := []struct {
		path string
		want bool
	}{
		{"/", true},
		{"/search?q=go", false},
		{"/search/jobs/1", true},
		{"/files/a.pdf", false},
		{"/files/a.pdf?x=1", true},
		{"/tmp123/cache/x", false},
	}

	for _, tt := range tests {
		if got := r.allowed(tt.path); got != tt.want {
			t.Errorf("allowed(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestProductToken(t *testing.T) {
	if got := productToken("JobsScraper/1.0 (+https://github.com/x/y)"); got != "jobsscraper" {
		t.Errorf("productToken = %q, want jobsscraper", got)
	}
}
