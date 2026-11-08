package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"main/database"
	"main/jobdetail"
	"main/jobs"
)

const (
	descriptionTTL        = 12 * time.Hour
	missingDescriptionTTL = time.Hour
	maxLocalDescriptions  = 500
)

// fetchDescription is a var so tests can stub the live fetch.
var fetchDescription = jobdetail.Fetch

// Without Redis, descriptions cache in process.
var localDescriptions = struct {
	sync.Mutex
	entries map[int64]cachedDescription
}{entries: map[int64]cachedDescription{}}

type cachedDescription struct {
	text    string
	expires time.Time
}

func descriptionKey(jobID int64) string {
	return "jobdesc:" + strconv.FormatInt(jobID, 10)
}

// cachedJobDescription is the cached description, "" meaning known unavailable; ok is false on a miss.
func cachedJobDescription(ctx context.Context, jobID int64) (string, bool) {
	if rdb := sharedRedis(); rdb != nil {
		text, err := rdb.Get(ctx, descriptionKey(jobID)).Result()
		if err == nil {
			return text, true
		}

		return "", false
	}

	localDescriptions.Lock()
	defer localDescriptions.Unlock()

	entry, ok := localDescriptions.entries[jobID]
	if !ok || time.Now().After(entry.expires) {
		return "", false
	}

	return entry.text, true
}

func cacheJobDescription(ctx context.Context, jobID int64, text string) {
	ttl := descriptionTTL
	if text == "" {
		ttl = missingDescriptionTTL
	}

	if rdb := sharedRedis(); rdb != nil {
		if err := rdb.Set(ctx, descriptionKey(jobID), text, ttl).Err(); err != nil {
			slog.Warn("cache job description", "job_id", jobID, "error", err)
		}

		return
	}

	localDescriptions.Lock()
	defer localDescriptions.Unlock()

	if len(localDescriptions.entries) >= maxLocalDescriptions {
		clear(localDescriptions.entries)
	}

	localDescriptions.entries[jobID] = cachedDescription{text: text, expires: time.Now().Add(ttl)}
}

// jobDescription is job's description from cache or its posting; "" when the posting has none.
// Transient fetch errors aren't cached, so the next open retries.
func jobDescription(ctx context.Context, job *jobs.Job) (string, error) {
	if text, ok := cachedJobDescription(ctx, job.ID); ok {
		return text, nil
	}

	text, err := fetchDescription(ctx, job.URL, job.DetailURL)
	if err != nil && !errors.Is(err, jobdetail.ErrUnavailable) {
		return "", err
	}

	cacheJobDescription(ctx, job.ID, text)

	return text, nil
}

type jobDescriptionResponse struct {
	// Description is nil when the posting doesn't expose one.
	Description *string `json:"description"`
}

func handleGetJobDescription(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id", "job")
	if !ok {
		return
	}

	job, err := database.GetJobByID(r.Context(), id, database.JobDetailParams{})
	if err != nil {
		writeDBError(w, "get job description", err, "job not found", "failed to get job")
		return
	}

	text, err := jobDescription(r.Context(), job)
	if err != nil {
		slog.Warn("fetch job description", "job_id", id, "url", job.URL, "error", err)
		writeError(w, http.StatusBadGateway, "couldn't load the description from the posting")
		return
	}

	response := jobDescriptionResponse{}
	if text != "" {
		response.Description = &text
	}

	// Browsers can reuse it for a while; the posting rarely changes.
	w.Header().Set("Cache-Control", "private, max-age=3600")
	writeJSON(w, http.StatusOK, response)
}

// withDescription fills job.Description for LLM prompts; a missing one leaves title, company and tags to go on.
func withDescription(ctx context.Context, job *jobs.Job) {
	text, err := jobDescription(ctx, job)
	if err != nil {
		slog.Warn("fetch job description for prompt", "job_id", job.ID, "error", err)
	}

	job.Description = text
}
