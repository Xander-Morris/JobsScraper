package server

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"main/database"
	"main/jobs"
)

func handleSearchJobs(w http.ResponseWriter, r *http.Request) {
	params, err := parseJobSearchParams(r)

	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := database.SearchForJobs(r.Context(), params)

	if err != nil {
		log.Printf("search jobs: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to search jobs")
		return
	}

	if result.Jobs == nil {
		result.Jobs = []jobs.Job{}
	}

	writeJSON(w, http.StatusOK, jobSearchResponse{
		Jobs:   result.Jobs,
		Total:  result.Total,
		Limit:  params.Limit,
		Offset: params.Offset,
	})
}

func handleGetJob(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)

	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}

	job, err := database.GetJobByID(r.Context(), id)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}

		log.Printf("get job: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get job")
		return
	}

	writeJSON(w, http.StatusOK, job)
}

type jobSearchResponse struct {
	Jobs   []jobs.Job `json:"jobs"`
	Total  int        `json:"total"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
}

func parseJobSearchParams(r *http.Request) (*database.JobSearchParams, error) {
	query := r.URL.Query()

	params := &database.JobSearchParams{
		SearchQuery: strings.TrimSpace(query.Get("q")),
		Limit:       database.DefaultSearchLimit,
	}

	if raw := query.Get("workplace_type"); raw != "" {
		workplaceType, ok := jobs.ParseWorkplaceType(raw)

		if !ok {
			return nil, fmt.Errorf("invalid workplace_type %q", raw)
		}

		params.WorkplaceType = workplaceType
	}

	if raw := query.Get("min_salary"); raw != "" {
		minSalary, err := strconv.Atoi(raw)

		if err != nil || minSalary < 0 {
			return nil, fmt.Errorf("invalid min_salary %q", raw)
		}

		params.MinSalary = minSalary
	}

	if raw := query.Get("max_salary"); raw != "" {
		maxSalary, err := strconv.Atoi(raw)

		if err != nil || maxSalary < params.MinSalary {
			return nil, fmt.Errorf("invalid max_salary %q", raw)
		}

		params.MaxSalary = maxSalary
	}

	if raw := query.Get("tags"); raw != "" {
		for tag := range strings.SplitSeq(raw, ",") {
			if trimmed := strings.TrimSpace(tag); trimmed != "" {
				params.Tags = append(params.Tags, trimmed)
			}
		}
	}

	if raw := query.Get("sort"); raw != "" {
		switch database.SortOrder(raw) {
		case database.SortDate:
			params.Sort = database.SortDate
		case database.SortRelevance:
			params.Sort = database.SortRelevance
		default:
			return nil, fmt.Errorf("invalid sort %q", raw)
		}
	}

	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)

		if err != nil || limit <= 0 {
			return nil, fmt.Errorf("invalid limit %q", raw)
		}

		params.Limit = min(limit, database.MaxSearchLimit)
	}

	if raw := query.Get("offset"); raw != "" {
		offset, err := strconv.Atoi(raw)

		if err != nil || offset < 0 {
			return nil, fmt.Errorf("invalid offset %q", raw)
		}

		params.Offset = offset
	}

	return params, nil
}
