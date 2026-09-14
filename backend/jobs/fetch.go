package jobs

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
)

// getJSON GETs url and decodes the JSON body into dest.
func getJSON(client *http.Client, userAgent, url string, dest any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned status %d", url, resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
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
