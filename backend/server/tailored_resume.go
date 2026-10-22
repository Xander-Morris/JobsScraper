package server

import (
	"errors"
	"log/slog"
	"net/http"

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

	extraction, ok := requireActiveResume(w, r, profileID, "tailor resume")
	if !ok {
		return
	}

	job, err := database.GetJobByID(r.Context(), jobID, database.JobDetailParams{})
	if err != nil {
		writeDBError(w, "tailor resume: get job", err, "job not found", "failed to load job")
		return
	}

	ctx, cancel := llmContext(w, r, "tailor resume")
	defer cancel()

	tailored, err := llm.TailorResume(ctx, extraction.ExtractedResume, jobPosting(job))

	if err != nil {
		slog.Error("tailor resume", "profile_id", profileID, "job_id", jobID, "error", err)
		writeLLMError(w, err, "failed to tailor resume")
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
