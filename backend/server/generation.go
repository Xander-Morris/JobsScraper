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

const generationTimeout = 2 * time.Minute

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
