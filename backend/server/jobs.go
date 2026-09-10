package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pgvector/pgvector-go"

	"main/database"
	"main/jobs"
)

var datePostedLookback = map[string]time.Duration{
	"24h":   24 * time.Hour,
	"3d":    3 * 24 * time.Hour,
	"week":  7 * 24 * time.Hour,
	"month": 30 * 24 * time.Hour,
}

func handleSearchJobs(w http.ResponseWriter, r *http.Request) {
	params, err := parseJobSearchParams(r)

	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if profileID, ok := profileIDFromContext(r.Context()); ok {
		params.ProfileID = profileID
		params.ResumeQuery, params.ResumeEmbedding = activeResumeSearchContext(r.Context(), profileID, "search jobs")
	}

	result, err := database.SearchForJobs(r.Context(), params)

	if err != nil {
		slog.Error("search jobs", "error", err)
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

	detailParams := database.JobDetailParams{}

	if profileID, ok := profileIDFromContext(r.Context()); ok {
		detailParams.ProfileID = profileID
		detailParams.ResumeQuery, detailParams.ResumeEmbedding = activeResumeSearchContext(r.Context(), profileID, "get job")
	}

	job, err := database.GetJobByID(r.Context(), id, detailParams)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}

		slog.Error("get job", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get job")
		return
	}

	writeJSON(w, http.StatusOK, job)
}

func handleMarkJobApplied(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromContext(r.Context())

	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	jobID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)

	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}

	if err := database.MarkJobApplied(r.Context(), profileID, jobID); err != nil {
		slog.Error("mark job applied", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to mark job applied")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "applied"})
}

func handleUnmarkJobApplied(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromContext(r.Context())

	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	jobID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)

	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}

	if err := database.UnmarkJobApplied(r.Context(), profileID, jobID); err != nil {
		slog.Error("unmark job applied", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to unmark job applied")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "not_applied"})
}

type jobSearchResponse struct {
	Jobs   []jobs.Job `json:"jobs"`
	Total  int        `json:"total"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
}

// activeResumeSearchContext turns the profile's active resume into the
// query/embedding pair job search and job detail rank by. No active or
// unextracted resume just means no resume-based ranking, not an error.
// logLabel identifies the caller in the error log ("search jobs", "get job").
func activeResumeSearchContext(ctx context.Context, profileID int64, logLabel string) (query string, embedding *pgvector.Vector) {
	extraction, found, err := database.GetActiveResumeExtraction(ctx, profileID)

	if err != nil {
		slog.Error(logLabel+": get active resume extraction", "error", err)
		return "", nil
	}

	if !found {
		return "", nil
	}

	return buildResumeSearchQuery(extraction), extraction.Embedding
}

func parseJobSearchParams(r *http.Request) (*database.JobSearchParams, error) {
	query := r.URL.Query()

	params := &database.JobSearchParams{
		SearchQuery: strings.TrimSpace(query.Get("q")),
		Limit:       database.DefaultSearchLimit,
	}

	setters := []func(*database.JobSearchParams, url.Values) error{
		setWorkplaceTypeParam,
		setSalaryRangeParams,
		setDatePostedParam,
		setTagsParam,
		setSortParam,
		setLimitParam,
		setOffsetParam,
	}

	for _, set := range setters {
		if err := set(params, query); err != nil {
			return nil, err
		}
	}

	return params, nil
}

func setWorkplaceTypeParam(params *database.JobSearchParams, query url.Values) error {
	raw := query.Get("workplace_type")

	if raw == "" {
		return nil
	}

	workplaceType, ok := jobs.ParseWorkplaceType(raw)

	if !ok {
		return fmt.Errorf("invalid workplace_type %q", raw)
	}

	params.WorkplaceType = workplaceType

	return nil
}

func setSalaryRangeParams(params *database.JobSearchParams, query url.Values) error {
	if raw := query.Get("min_salary"); raw != "" {
		minSalary, err := strconv.Atoi(raw)

		if err != nil || minSalary < 0 {
			return fmt.Errorf("invalid min_salary %q", raw)
		}

		params.MinSalary = minSalary
	}

	if raw := query.Get("max_salary"); raw != "" {
		maxSalary, err := strconv.Atoi(raw)

		if err != nil || maxSalary < params.MinSalary {
			return fmt.Errorf("invalid max_salary %q", raw)
		}

		params.MaxSalary = maxSalary
	}

	return nil
}

func setDatePostedParam(params *database.JobSearchParams, query url.Values) error {
	raw := query.Get("date_posted")

	if raw == "" {
		return nil
	}

	lookback, ok := datePostedLookback[raw]

	if !ok {
		return fmt.Errorf("invalid date_posted %q", raw)
	}

	cutoff := time.Now().Add(-lookback)
	params.PostedAfter = &cutoff

	return nil
}

func setTagsParam(params *database.JobSearchParams, query url.Values) error {
	raw := query.Get("tags")

	if raw == "" {
		return nil
	}

	for tag := range strings.SplitSeq(raw, ",") {
		if trimmed := strings.TrimSpace(tag); trimmed != "" {
			params.Tags = append(params.Tags, trimmed)
		}
	}

	return nil
}

func setSortParam(params *database.JobSearchParams, query url.Values) error {
	raw := query.Get("sort")

	if raw == "" {
		return nil
	}

	switch database.SortOrder(raw) {
	case database.SortDate:
		params.Sort = database.SortDate
	case database.SortRelevance:
		params.Sort = database.SortRelevance
	default:
		return fmt.Errorf("invalid sort %q", raw)
	}

	return nil
}

func setLimitParam(params *database.JobSearchParams, query url.Values) error {
	raw := query.Get("limit")

	if raw == "" {
		return nil
	}

	limit, err := strconv.Atoi(raw)

	if err != nil || limit <= 0 {
		return fmt.Errorf("invalid limit %q", raw)
	}

	params.Limit = min(limit, database.MaxSearchLimit)

	return nil
}

func setOffsetParam(params *database.JobSearchParams, query url.Values) error {
	raw := query.Get("offset")

	if raw == "" {
		return nil
	}

	offset, err := strconv.Atoi(raw)

	if err != nil || offset < 0 {
		return fmt.Errorf("invalid offset %q", raw)
	}

	params.Offset = offset

	return nil
}
