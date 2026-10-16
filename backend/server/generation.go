package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"main/database"
	"main/jobs"
	"main/llm"
)

// generationTimeout stays under a serverless function's max duration (60s on
// Vercel Hobby) so a slow OpenRouter response ends in a clean error instead of
// the platform hard-killing the function mid-request.
const generationTimeout = 45 * time.Second

// requireActiveResume loads the caller's completed active-resume extraction,
// writing the error response and returning false when there isn't one.
func requireActiveResume(w http.ResponseWriter, r *http.Request, profileID int64, logLabel string) (database.ResumeExtraction, bool) {
	extraction, found, err := database.GetActiveResumeExtraction(r.Context(), profileID)
	if err != nil {
		slog.Error(logLabel+": get active resume extraction", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load active resume")
		return extraction, false
	}
	if !found {
		writeError(w, http.StatusNotFound, "no completed resume extraction found for your active resume")
		return extraction, false
	}

	return extraction, true
}

// llmContext bounds an LLM call by generationTimeout and stretches the write
// deadline to match, since a slow response would otherwise run past the
// server's 30s WriteTimeout and kill the response mid-generation.
func llmContext(w http.ResponseWriter, r *http.Request, logLabel string) (context.Context, context.CancelFunc) {
	if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(generationTimeout + 15*time.Second)); err != nil {
		slog.Error(logLabel+": extend write deadline", "error", err)
	}

	return context.WithTimeout(r.Context(), generationTimeout)
}

func jobPosting(job *jobs.Job) llm.JobPosting {
	return llm.JobPosting{
		Title:       job.Title,
		Company:     job.Company,
		Tags:        job.Tags,
		Description: job.Description,
	}
}

func handleGenerateApplicationContent(w http.ResponseWriter, r *http.Request, profileID int64) {
	jobID, ok := pathID(w, r, "id", "job")
	if !ok {
		return
	}

	extraction, ok := requireActiveResume(w, r, profileID, "generate application content")
	if !ok {
		return
	}

	job, err := database.GetJobByID(r.Context(), jobID, database.JobDetailParams{})
	if err != nil {
		writeDBError(w, "generate application content: get job", err, "job not found", "failed to load job")
		return
	}

	ctx, cancel := llmContext(w, r, "generate application content")
	defer cancel()

	generated, err := llm.GenerateApplicationContent(ctx, extraction.Profile(), jobPosting(job))
	if err != nil {
		slog.Error("generate application content", "profile_id", profileID, "job_id", jobID, "error", err)
		writeError(w, http.StatusBadGateway, "failed to generate application content")
		return
	}

	writeJSON(w, http.StatusOK, generated)
}
