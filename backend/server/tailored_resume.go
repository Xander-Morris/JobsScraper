package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"main/database"
	"main/llm"
)

const maxTailoredResumeSize = 64 << 10 // 64 KiB

func writeTailoredResume(w http.ResponseWriter, r *http.Request, profileID, jobID int64) {
	tailored, err := database.GetTailoredResume(r.Context(), profileID, jobID)
	if err != nil {
		writeDBError(w, "get tailored resume", err, "no tailored resume for this job", "failed to load tailored resume")
		return
	}

	writeJSON(w, http.StatusOK, tailored)
}

func handleGetTailoredResume(w http.ResponseWriter, r *http.Request, profileID int64) {
	jobID, ok := pathID(w, r, "id", "job")
	if !ok {
		return
	}

	writeTailoredResume(w, r, profileID, jobID)
}

func handleGenerateTailoredResume(w http.ResponseWriter, r *http.Request, profileID int64) {
	jobID, ok := pathID(w, r, "id", "job")
	if !ok {
		return
	}

	extraction, found, err := database.GetActiveResumeExtraction(r.Context(), profileID)
	if err != nil {
		slog.Error("tailor resume: get active resume extraction", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load active resume")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "no completed resume extraction found for your active resume")
		return
	}

	job, err := database.GetJobByID(r.Context(), jobID, database.JobDetailParams{})
	if err != nil {
		writeDBError(w, "tailor resume: get job", err, "job not found", "failed to load job")
		return
	}

	if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(generationTimeout + 15*time.Second)); err != nil {
		slog.Error("tailor resume: extend write deadline", "error", err)
	}

	ctx, cancel := context.WithTimeout(r.Context(), generationTimeout)
	defer cancel()

	tailored, err := llm.TailorResume(ctx, llm.ExtractedResume{
		FullName:       extraction.FullName,
		Email:          extraction.Email,
		Phone:          extraction.Phone,
		LinkedIn:       extraction.LinkedIn,
		GitHub:         extraction.GitHub,
		Portfolio:      extraction.Portfolio,
		Summary:        extraction.Summary,
		Skills:         extraction.Skills,
		Education:      extraction.Education,
		WorkExperience: extraction.WorkExperience,
		Projects:       extraction.Projects,
	}, llm.JobPosting{
		Title:       job.Title,
		Company:     job.Company,
		Tags:        job.Tags,
		Description: job.Description,
	})

	if err != nil {
		slog.Error("tailor resume", "profile_id", profileID, "job_id", jobID, "error", err)
		writeError(w, http.StatusBadGateway, "failed to tailor resume")
		return
	}

	if err := database.UpsertTailoredResume(r.Context(), profileID, jobID, extraction.ResumeID, extraction.UpdatedAt, *tailored); err != nil {
		slog.Error("tailor resume: save", "profile_id", profileID, "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to save tailored resume")
		return
	}

	writeTailoredResume(w, r, profileID, jobID)
}

func handleUpdateTailoredResume(w http.ResponseWriter, r *http.Request, profileID int64) {
	jobID, ok := pathID(w, r, "id", "job")
	if !ok {
		return
	}

	var content llm.TailoredResume
	r.Body = http.MaxBytesReader(w, r.Body, maxTailoredResumeSize)

	if err := decodeJSON(w, r, &content); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "tailored resume is too large")
			return
		}

		writeError(w, http.StatusBadRequest, "invalid tailored resume")
		return
	}

	if err := database.UpdateTailoredResumeContent(r.Context(), profileID, jobID, content); err != nil {
		writeDBError(w, "update tailored resume", err, "no tailored resume for this job", "failed to save tailored resume")
		return
	}

	writeTailoredResume(w, r, profileID, jobID)
}
