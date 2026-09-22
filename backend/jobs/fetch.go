package jobs

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// feedTimeout bounds one request to a single-endpoint feed.
const feedTimeout = 10 * time.Second

// boardFeedTimeout is longer because one fetch walks every board on the ATS.
const boardFeedTimeout = 60 * time.Second

// feed is what every source needs to reach its API: a client, the User-Agent
// we identify as, and the endpoint, which tests point at a local server.
type feed struct {
	HTTPClient *http.Client
	UserAgent  string
	Endpoint   string
}

func newFeed(userAgent, endpoint string) feed {
	return feed{
		HTTPClient: &http.Client{Timeout: feedTimeout},
		UserAgent:  userAgent,
		Endpoint:   endpoint,
	}
}

func newBoardFeed(userAgent, endpoint string) feed {
	return feed{
		HTTPClient: &http.Client{Timeout: boardFeedTimeout},
		UserAgent:  userAgent,
		Endpoint:   endpoint,
	}
}

// getJSON GETs url and decodes the JSON body into dest.
func (f feed) getJSON(url string, dest any) error {
	return f.get(url, func(r io.Reader) error { return json.NewDecoder(r).Decode(dest) })
}

// getXML GETs url and decodes the XML body into dest.
func (f feed) getXML(url string, dest any) error {
	return f.get(url, func(r io.Reader) error { return xml.NewDecoder(r).Decode(dest) })
}

func (f feed) get(url string, decode func(io.Reader) error) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("User-Agent", f.UserAgent)

	resp, err := f.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned status %d", url, resp.StatusCode)
	}

	if err := decode(resp.Body); err != nil {
		return fmt.Errorf("decode %s: %w", url, err)
	}

	return nil
}

// Board is one company's public job board on an applicant tracking system.
type Board struct {
	Slug    string
	Company string
}

// boardFetchConcurrency keeps requests to one ATS host polite.
const boardFetchConcurrency = 4

// fetchBoards fetches every board, logging and skipping ones that fail. It only errors if all of them fail.
func fetchBoards(source string, boards []Board, fetch func(Board) ([]Job, error)) ([]Job, error) {
	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		result []Job
		errs   []error
	)

	sem := make(chan struct{}, boardFetchConcurrency)

	for _, board := range boards {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()

			boardJobs, err := fetchBoardSafely(board, fetch)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				slog.Warn("jobs: board fetch failed", "source", source, "board", board.Slug, "error", err)
				errs = append(errs, fmt.Errorf("%s: %w", board.Slug, err))
				return
			}

			result = append(result, boardJobs...)
		})
	}

	wg.Wait()

	if len(boards) > 0 && len(errs) == len(boards) {
		return nil, errors.Join(errs...)
	}

	return result, nil
}

func fetchBoardSafely(board Board, fetch func(Board) ([]Job, error)) (boardJobs []Job, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	return fetch(board)
}
