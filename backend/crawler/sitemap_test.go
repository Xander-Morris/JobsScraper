package crawler

import (
	"bytes"
	"compress/gzip"
	"slices"
	"strings"
	"testing"
)

func TestParseSitemapURLSet(t *testing.T) {
	parsed, err := parseSitemap(strings.NewReader(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc> https://example.com/jobs/1 </loc><lastmod>2026-09-01</lastmod></url>
	<url><loc>https://example.com/jobs/2</loc></url>
</urlset>`))
	if err != nil {
		t.Fatalf("parseSitemap: %v", err)
	}

	if len(parsed.URLs) != 2 || parsed.URLs[0].Loc != "https://example.com/jobs/1" {
		t.Errorf("URLs = %+v, want two trimmed entries", parsed.URLs)
	}
}

func TestParseSitemapIndexGzipped(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write([]byte(`<sitemapindex><sitemap><loc>https://example.com/a.xml</loc></sitemap></sitemapindex>`))
	gz.Close()

	parsed, err := parseSitemap(&buf)
	if err != nil {
		t.Fatalf("parseSitemap: %v", err)
	}

	if len(parsed.Sitemaps) != 1 || parsed.Sitemaps[0].Loc != "https://example.com/a.xml" {
		t.Errorf("Sitemaps = %+v, want the one child sitemap", parsed.Sitemaps)
	}
}

func TestNewestFirst(t *testing.T) {
	entries := []sitemapEntry{
		{Loc: "undated-a"},
		{Loc: "old", LastMod: "2026-01-01"},
		{Loc: "new", LastMod: "2026-09-01T10:00:00Z"},
		{Loc: "undated-b"},
	}

	newestFirst(entries)

	var got []string
	for _, entry := range entries {
		got = append(got, entry.Loc)
	}

	if want := []string{"new", "old", "undated-a", "undated-b"}; !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}
