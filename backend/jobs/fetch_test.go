package jobs

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"testing"
)

func TestFetchBoardsSkipsFailedBoards(t *testing.T) {
	boards := []Board{{"ok", "OK Co"}, {"broken", "Broken Co"}}

	got, err := fetchBoards("test", boards, func(board Board) ([]Job, error) {
		if board.Slug == "broken" {
			return nil, errors.New("boom")
		}

		return []Job{{Title: "Engineer", Company: board.Company}}, nil
	})

	if err != nil {
		t.Fatalf("err = %v, want nil when only some boards fail", err)
	}

	if len(got) != 1 || got[0].Company != "OK Co" {
		t.Errorf("jobs = %v, want the one job from the working board", got)
	}
}

func TestFetchBoardsErrorsWhenAllFail(t *testing.T) {
	boards := []Board{{"a", "A"}, {"b", "B"}}

	_, err := fetchBoards("test", boards, func(board Board) ([]Job, error) {
		panic("bad payload")
	})

	if err == nil {
		t.Fatal("err = nil, want an error when every board fails")
	}
}

func TestHimalayasFetchJobsFollowsCursor(t *testing.T) {
	var cursors []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		cursors = append(cursors, cursor)

		if cursor == "" {
			fmt.Fprint(w, `{"jobs":[{"title":"First","guid":"https://himalayas.app/jobs/1"}],"nextCursor":"page2"}`)
			return
		}

		fmt.Fprint(w, `{"jobs":[{"title":"Second","guid":"https://himalayas.app/jobs/2"}],"nextCursor":""}`)
	}))
	defer srv.Close()

	h := NewHimalayas("test-agent")
	h.Endpoint = srv.URL

	got, err := h.FetchJobs()
	if err != nil {
		t.Fatalf("FetchJobs: %v", err)
	}

	if len(got) != 2 {
		t.Errorf("got %d jobs, want 2", len(got))
	}

	if want := []string{"", "page2"}; !slices.Equal(cursors, want) {
		t.Errorf("cursors requested = %v, want %v", cursors, want)
	}
}

func TestArbeitnowFetchJobsStopsAtLastPage(t *testing.T) {
	var pages []int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pages = append(pages, page)

		next := `"https://www.arbeitnow.com/api/job-board-api?page=2"`
		if page == 2 {
			next = "null"
		}

		fmt.Fprintf(w, `{"data":[{"slug":"job-%d","title":"Job %d","url":"https://www.arbeitnow.com/jobs/%d"}],"links":{"next":%s}}`, page, page, page, next)
	}))
	defer srv.Close()

	a := NewArbeitnow("test-agent")
	a.Endpoint = srv.URL

	got, err := a.FetchJobs()
	if err != nil {
		t.Fatalf("FetchJobs: %v", err)
	}

	if len(got) != 2 {
		t.Errorf("got %d jobs, want 2", len(got))
	}

	if want := []int{1, 2}; !slices.Equal(pages, want) {
		t.Errorf("pages requested = %v, want %v", pages, want)
	}
}
