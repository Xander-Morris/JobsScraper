// Package jobdetail fetches a job's description live from its posting, since descriptions aren't stored.
package jobdetail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"main/crawler"
	"main/utils"
)

// ErrUnavailable means the posting is gone or doesn't expose a description.
var ErrUnavailable = errors.New("job description unavailable")

const (
	fetchTimeout = 8 * time.Second
	maxBodyBytes = 4 << 20
	userAgent    = "Mozilla/5.0 (compatible; JobsScraper/1.0; +https://github.com/Xander-Morris/JobsScraper)"
)

// httpClient refuses private addresses, since posting URLs come from crawled pages. Tests swap it.
var httpClient = &http.Client{
	Timeout: fetchTimeout,
	Transport: &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second, Control: publicOnly}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: fetchTimeout,
		MaxIdleConnsPerHost:   2,
	},
}

func publicOnly(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}

	ip := net.ParseIP(host)
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return fmt.Errorf("refusing non-public address %s", host)
	}

	return nil
}

// Fetch loads the description from detailURL (a JSON API like Greenhouse's) when set, else from
// the JSON-LD JobPosting on the posting page.
func Fetch(ctx context.Context, postingURL, detailURL string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	if detailURL == "" {
		detailURL = himalayasAPIURL(postingURL)
	}

	if detailURL != "" {
		return fetchAPI(ctx, detailURL)
	}

	return fetchPage(ctx, postingURL)
}

// himalayasAPIURL maps a Himalayas posting to its single-job API, since the page itself answers 403.
func himalayasAPIURL(postingURL string) string {
	parsed, err := url.Parse(postingURL)
	if err != nil || (parsed.Host != "himalayas.app" && parsed.Host != "www.himalayas.app") || !strings.HasPrefix(parsed.Path, "/companies/") {
		return ""
	}

	return "https://himalayas.app/jobs/api" + parsed.Path
}

func get(ctx context.Context, rawURL, accept string) (*http.Response, *url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, nil, ErrUnavailable
	}

	req := (&http.Request{Method: http.MethodGet, URL: parsed, Host: parsed.Host, Header: http.Header{}}).WithContext(ctx)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", accept)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}

	// Gone, or blocking scrapers (Himalayas answers 403): either way there's nothing to show.
	switch resp.StatusCode {
	case http.StatusNotFound, http.StatusGone, http.StatusForbidden, http.StatusUnauthorized:
		resp.Body.Close()
		return nil, nil, ErrUnavailable
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, nil, fmt.Errorf("fetch %s: status %d", rawURL, resp.StatusCode)
	}

	// Redirects can land elsewhere, and JSON-LD links resolve against the final page.
	return resp, resp.Request.URL, nil
}

func fetchAPI(ctx context.Context, detailURL string) (string, error) {
	resp, _, err := get(ctx, detailURL, "application/json")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Greenhouse answers {content}, Himalayas {job: {description}}.
	var body struct {
		Content string `json:"content"`
		Job     struct {
			Description string `json:"description"`
		} `json:"job"`
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes)).Decode(&body); err != nil {
		return "", fmt.Errorf("decode %s: %w", detailURL, err)
	}

	description := strings.TrimSpace(utils.StripHTML(body.Content + body.Job.Description))
	if description == "" {
		return "", ErrUnavailable
	}

	return description, nil
}

func fetchPage(ctx context.Context, postingURL string) (string, error) {
	resp, final, err := get(ctx, postingURL, "text/html")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	description, ok := crawler.PostingDescription(io.LimitReader(resp.Body, maxBodyBytes), final)
	if !ok {
		return "", ErrUnavailable
	}

	return description, nil
}
