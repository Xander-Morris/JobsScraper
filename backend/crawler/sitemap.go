package crawler

import (
	"bufio"
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"
)

// sitemapFile decodes both a <urlset> and a <sitemapindex>.
type sitemapFile struct {
	URLs     []sitemapEntry `xml:"url"`
	Sitemaps []sitemapEntry `xml:"sitemap"`
}

type sitemapEntry struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

// parseSitemap reads a plain or gzipped sitemap.
func parseSitemap(r io.Reader) (sitemapFile, error) {
	buffered := bufio.NewReader(r)

	if magic, err := buffered.Peek(2); err == nil && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(buffered)
		if err != nil {
			return sitemapFile{}, fmt.Errorf("open gzip sitemap: %w", err)
		}
		defer gz.Close()

		r = io.LimitReader(gz, maxSitemapBytes)
	} else {
		r = buffered
	}

	var parsed sitemapFile

	decoder := xml.NewDecoder(r)
	// Sitemaps occasionally declare a non-UTF-8 charset; read them as-is rather than failing.
	decoder.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }

	if err := decoder.Decode(&parsed); err != nil {
		return sitemapFile{}, fmt.Errorf("decode sitemap: %w", err)
	}

	for _, list := range [][]sitemapEntry{parsed.URLs, parsed.Sitemaps} {
		for i := range list {
			list[i].Loc = strings.TrimSpace(list[i].Loc)
		}
	}

	return parsed, nil
}

// newestFirst sorts entries by lastmod, undated ones last in their original order.
func newestFirst(entries []sitemapEntry) {
	slices.SortStableFunc(entries, func(a, b sitemapEntry) int {
		return parseDate(b.LastMod).Compare(parseDate(a.LastMod))
	})
}

var dateLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05Z0700",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02",
}

// parseDate accepts the W3C datetime forms sitemaps and schema.org use; zero if unparseable.
func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)

	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}

	return time.Time{}
}
