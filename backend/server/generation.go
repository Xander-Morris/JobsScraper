package server

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"main/database"
	"main/llm"
)

// generationTimeout stays under a serverless function's max duration (60s on
// Vercel Hobby) so a slow OpenRouter response ends in a clean error instead of
// the platform hard-killing the function mid-request.
const generationTimeout = 45 * time.Second

func handleGenerateApplicationContent(w http.ResponseWriter, r *http.Request) {
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

	extraction, found, err := database.GetActiveResumeExtraction(r.Context(), profileID)
	if err != nil {
		slog.Error("generate application content: get active resume extraction", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load active resume")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "no completed resume extraction found for your active resume")
		return
	}

	job, err := database.GetJobByID(r.Context(), jobID, database.JobDetailParams{})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}

		slog.Error("generate application content: get job", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load job")
		return
	}

	// A slow LLM response can run past the server's 30s WriteTimeout, which
	// would kill the response mid-generation.
	if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(generationTimeout + 15*time.Second)); err != nil {
		slog.Error("generate application content: extend write deadline", "error", err)
	}

	ctx, cancel := context.WithTimeout(r.Context(), generationTimeout)
	defer cancel()

	generated, err := llm.GenerateApplicationContent(ctx, llm.ResumeProfile{
		FullName:       extraction.FullName,
		Summary:        extraction.Summary,
		Skills:         extraction.Skills,
		WorkExperience: extraction.WorkExperience,
		Projects:       extraction.Projects,
	}, llm.JobPosting{
		Title:       job.Title,
		Company:     job.Company,
		Description: job.Description,
	})

	if err != nil {
		slog.Error("generate application content", "profile_id", profileID, "job_id", jobID, "error", err)
		writeError(w, http.StatusBadGateway, "failed to generate application content")
		return
	}

	writeJSON(w, http.StatusOK, generated)
}
