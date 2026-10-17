// Package crawler finds job pages on career sites through robots.txt, sitemaps, and links,
// loads each page, and reads the schema.org JobPosting data embedded in its HTML.
package crawler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"main/jobs"

	"golang.org/x/net/html/charset"
	"golang.org/x/time/rate"
)

// Seed is one career site to crawl.
type Seed struct {
	// URL is where crawling starts, and bounds it: only pages under this path are fetched.
	URL string
	// Company replaces hiringOrganization, which is often a legal entity like "2100 NVIDIA USA".
	Company string
	// Include, when set, limits which pages past the start page are fetched.
	Include *regexp.Regexp
}

// KnownFunc reports which URLs are already stored, so the page budget goes to new postings.
type KnownFunc func(ctx context.Context, urls []string) (map[string]bool, error)

const (
	defaultMinDelay        = time.Second
	maxCrawlDelay          = 10 * time.Second
	defaultMaxPagesPerSeed = 40
	defaultTimeout         = 6 * time.Minute
	requestTimeout         = 20 * time.Second
	seedConcurrency        = 4
	// maxDepth is how many links away from the start page a crawl goes.
	maxDepth        = 2
	maxSitemaps     = 10
	maxPageBytes    = 3 << 20
	maxSitemapBytes = 20 << 20
)

var _ jobs.JobSource = (*Crawler)(nil)

// Crawler fetches at most MaxPagesPerSeed pages per seed per run, waiting MinDelay
// (or robots.txt's Crawl-delay) between requests to the same host.
type Crawler struct {
	HTTPClient      *http.Client
	UserAgent       string
	Seeds           []Seed
	Known           KnownFunc
	MinDelay        time.Duration
	MaxPagesPerSeed int
	Timeout         time.Duration
}

func New(userAgent string, known KnownFunc) *Crawler {
	return &Crawler{
		HTTPClient:      &http.Client{Timeout: requestTimeout},
		UserAgent:       userAgent,
		Seeds:           seeds,
		Known:           known,
		MinDelay:        defaultMinDelay,
		MaxPagesPerSeed: defaultMaxPagesPerSeed,
		Timeout:         defaultTimeout,
	}
}

// FetchJobs crawls every seed, returning what it found by Timeout. It only errors if every seed fails.
func (c *Crawler) FetchJobs(ctx context.Context) ([]jobs.Job, error) {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	limiters := &hostLimiters{minDelay: c.MinDelay, byHost: make(map[string]*rate.Limiter)}

	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		result []jobs.Job
		errs   []error
	)

	sem := make(chan struct{}, seedConcurrency)

	for _, seed := range c.Seeds {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()

			seedJobs, err := c.crawlSeedSafely(ctx, seed, limiters)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				slog.Warn("crawler: seed failed", "seed", seed.URL, "error", err)
				errs = append(errs, fmt.Errorf("%s: %w", seed.URL, err))
				return
			}

			result = append(result, seedJobs...)
		})
	}

	wg.Wait()

	if len(c.Seeds) > 0 && len(errs) == len(c.Seeds) {
		return nil, errors.Join(errs...)
	}

	return result, nil
}

func (c *Crawler) crawlSeedSafely(ctx context.Context, seed Seed, limiters *hostLimiters) (seedJobs []jobs.Job, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	return c.crawlSeed(ctx, seed, limiters)
}

// target is a page waiting to be fetched, depth links from the start page.
type target struct {
	url   string
	depth int
}

// seedCrawl is the state of one seed's crawl.
type seedCrawl struct {
	*Crawler
	seed    Seed
	scope   *url.URL
	robots  robots
	limiter *rate.Limiter
	queued  map[string]bool
}

func (c *Crawler) crawlSeed(ctx context.Context, seed Seed, limiters *hostLimiters) ([]jobs.Job, error) {
	scope, err := url.Parse(seed.URL)
	if err != nil || scope.Host == "" {
		return nil, fmt.Errorf("bad seed url %q", seed.URL)
	}

	s := &seedCrawl{
		Crawler: c,
		seed:    seed,
		scope:   scope,
		limiter: limiters.get(scope.Host),
		queued:  map[string]bool{seed.URL: true},
	}

	if s.robots, err = s.fetchRobots(ctx); err != nil {
		return nil, err
	}

	if !s.robots.allowed(requestPath(scope)) {
		return nil, errors.New("robots.txt disallows the seed")
	}

	limiters.slowTo(scope.Host, min(s.robots.crawlDelay, maxCrawlDelay))

	// Sitemap pages are leaves: they're the postings themselves, so their links aren't followed.
	var frontier []target
	for _, link := range s.unknown(ctx, s.sitemapPages(ctx)) {
		frontier = append(frontier, target{link, maxDepth})
	}
	frontier = append(frontier, target{seed.URL, 0})

	var (
		found         []jobs.Job
		pages, failed int
		now           = time.Now()
	)

	for len(frontier) > 0 && pages < c.MaxPagesPerSeed && ctx.Err() == nil {
		next := frontier[0]
		frontier = frontier[1:]
		pages++

		pageURL, parsed, err := s.fetchPage(ctx, next.url)
		if err != nil {
			failed++
			slog.Debug("crawler: page failed", "url", next.url, "error", err)
			continue
		}

		for _, posting := range parsed.postings {
			if job, ok := posting.toJob(pageURL, seed.Company, now); ok {
				found = append(found, job)
			}
		}

		// A page with a posting is a job page; its links lead to other sections, not more jobs.
		if len(parsed.postings) > 0 || next.depth >= maxDepth {
			continue
		}

		var links []string
		for _, link := range parsed.links {
			if !s.queued[link] && s.wanted(link) {
				s.queued[link] = true
				links = append(links, link)
			}
		}

		for _, link := range s.unknown(ctx, links) {
			frontier = append(frontier, target{link, next.depth + 1})
		}
	}

	slog.Info("crawler: crawled seed", "seed", seed.URL, "pages", pages, "failed", failed, "jobs", len(found))

	return found, nil
}

// wanted reports whether link is in scope, matches Include, and is allowed by robots.txt.
func (s *seedCrawl) wanted(link string) bool {
	u, err := url.Parse(link)
	if err != nil || !s.inScope(u) {
		return false
	}

	if s.seed.Include != nil && !s.seed.Include.MatchString(link) {
		return false
	}

	return s.robots.allowed(requestPath(u))
}

func (s *seedCrawl) inScope(u *url.URL) bool {
	if !strings.EqualFold(u.Host, s.scope.Host) {
		return false
	}

	prefix := strings.TrimSuffix(s.scope.Path, "/")

	return u.Path == prefix || strings.HasPrefix(u.Path, prefix+"/")
}

func requestPath(u *url.URL) string {
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}

	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}

	return path
}

// unknown drops links already stored as jobs; if the lookup fails, it keeps them all.
func (s *seedCrawl) unknown(ctx context.Context, links []string) []string {
	if s.Known == nil || len(links) == 0 {
		return links
	}

	known, err := s.Known(ctx, links)
	if err != nil {
		slog.Warn("crawler: known url lookup failed", "seed", s.seed.URL, "error", err)
		return links
	}

	kept := links[:0]
	for _, link := range links {
		if !known[link] {
			kept = append(kept, link)
		}
	}

	return kept
}

// fetchRobots treats a missing robots.txt as allowing everything and an unreachable one as an error.
func (s *seedCrawl) fetchRobots(ctx context.Context) (robots, error) {
	robotsURL := s.scope.Scheme + "://" + s.scope.Host + "/robots.txt"

	resp, err := s.get(ctx, robotsURL)
	if err != nil {
		return robots{}, fmt.Errorf("fetch robots.txt: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		return parseRobots(io.LimitReader(resp.Body, maxPageBytes), productToken(s.UserAgent)), nil
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return robots{}, nil
	default:
		return robots{}, fmt.Errorf("robots.txt returned status %d", resp.StatusCode)
	}
}

// productToken is the name robots.txt groups match, like "jobsscraper" from "JobsScraper/1.0 (...)".
func productToken(userAgent string) string {
	token, _, _ := strings.Cut(userAgent, "/")
	token, _, _ = strings.Cut(token, " ")

	return strings.ToLower(token)
}

// sitemapPages walks the host's sitemaps from robots.txt and returns wanted pages, newest first.
func (s *seedCrawl) sitemapPages(ctx context.Context) []string {
	var queue []string
	for _, sitemap := range s.robots.sitemaps {
		if u, err := url.Parse(sitemap); err == nil && strings.EqualFold(u.Host, s.scope.Host) {
			queue = append(queue, sitemap)
		}
	}

	var entries []sitemapEntry
	fetched := make(map[string]bool)

	for len(queue) > 0 && len(fetched) < maxSitemaps && ctx.Err() == nil {
		sitemapURL := queue[0]
		queue = queue[1:]

		if fetched[sitemapURL] {
			continue
		}
		fetched[sitemapURL] = true

		parsed, err := s.fetchSitemap(ctx, sitemapURL)
		if err != nil {
			slog.Warn("crawler: sitemap failed", "url", sitemapURL, "error", err)
			continue
		}

		for _, child := range parsed.Sitemaps {
			if u, err := url.Parse(child.Loc); err == nil && strings.EqualFold(u.Host, s.scope.Host) {
				queue = append(queue, child.Loc)
			}
		}

		for _, entry := range parsed.URLs {
			if !s.queued[entry.Loc] && s.wanted(entry.Loc) {
				s.queued[entry.Loc] = true
				entries = append(entries, entry)
			}
		}
	}

	newestFirst(entries)

	links := make([]string, len(entries))
	for i, entry := range entries {
		links[i] = entry.Loc
	}

	return links
}

func (s *seedCrawl) fetchSitemap(ctx context.Context, sitemapURL string) (sitemapFile, error) {
	resp, err := s.get(ctx, sitemapURL)
	if err != nil {
		return sitemapFile{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return sitemapFile{}, fmt.Errorf("status %d", resp.StatusCode)
	}

	return parseSitemap(io.LimitReader(resp.Body, maxSitemapBytes))
}

// fetchPage loads an HTML page, returning the URL it ended up at after redirects.
func (s *seedCrawl) fetchPage(ctx context.Context, pageURL string) (string, page, error) {
	resp, err := s.get(ctx, pageURL)
	if err != nil {
		return "", page{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", page{}, fmt.Errorf("status %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "html") {
		return "", page{}, fmt.Errorf("not html: %q", contentType)
	}

	final := resp.Request.URL
	if !s.inScope(final) {
		return "", page{}, fmt.Errorf("redirected out of scope to %s", final)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPageBytes))
	if err != nil {
		return "", page{}, fmt.Errorf("read body: %w", err)
	}

	decoded, err := charset.NewReader(bytes.NewReader(body), contentType)
	if err != nil {
		return "", page{}, fmt.Errorf("decode charset: %w", err)
	}

	parsed, err := parsePage(decoded, final)
	if err != nil {
		return "", page{}, fmt.Errorf("parse html: %w", err)
	}

	final.Fragment = ""

	return final.String(), parsed, nil
}

// get waits its turn for the host, then GETs rawURL.
func (s *seedCrawl) get(ctx context.Context, rawURL string) (*http.Response, error) {
	if err := s.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("User-Agent", s.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	return s.HTTPClient.Do(req)
}

// hostLimiters spaces out requests per host, shared by every seed on that host.
type hostLimiters struct {
	mu       sync.Mutex
	minDelay time.Duration
	byHost   map[string]*rate.Limiter
}

func (h *hostLimiters) get(host string) *rate.Limiter {
	h.mu.Lock()
	defer h.mu.Unlock()

	host = strings.ToLower(host)

	limiter, ok := h.byHost[host]
	if !ok {
		limiter = rate.NewLimiter(every(h.minDelay), 1)
		h.byHost[host] = limiter
	}

	return limiter
}

// slowTo raises the host's delay to delay if that's slower than it is now.
func (h *hostLimiters) slowTo(host string, delay time.Duration) {
	limiter := h.get(host)

	if limit := every(delay); limit < limiter.Limit() {
		limiter.SetLimit(limit)
	}
}

func every(delay time.Duration) rate.Limit {
	if delay <= 0 {
		return rate.Inf
	}

	return rate.Every(delay)
}
