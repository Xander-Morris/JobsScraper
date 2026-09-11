package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"main/database"
	"main/llm"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const maxResumeSize = 10 << 20 // 10 MiB

const resumeUploadReadTimeout = 2 * time.Minute

func handleUploadResume(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	fileName, contentType, content, err := readResumeUpload(w, r, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	id, err := database.AddResume(r.Context(), profileID, fileName, contentType, content)
	if err != nil {
		slog.Error("upload resume", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to upload resume")
		return
	}

	runResumeExtraction(id, fileName, contentType, content)

	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func handleUpdateResume(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	resumeID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid resume id")
		return
	}

	fileName, contentType, content, err := readResumeUpload(w, r, false)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if content == nil {
		err = database.RenameResume(r.Context(), profileID, resumeID, fileName)
	} else {
		err = database.ReplaceResume(r.Context(), profileID, resumeID, fileName, contentType, content)
		if err == nil {
			runResumeExtraction(resumeID, fileName, contentType, content)
		}
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "resume not found")
			return
		}

		slog.Error("update resume", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update resume")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func handleDownloadResume(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	resumeID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid resume id")
		return
	}

	resume, content, err := database.GetResume(r.Context(), profileID, resumeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "resume not found")
			return
		}

		slog.Error("download resume", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to download resume")
		return
	}

	w.Header().Set("Content-Type", resume.ContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": resume.FileName}))
	w.Header().Set("Content-Length", strconv.FormatInt(resume.FileSize, 10))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func handleDeleteResume(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	resumeID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid resume id")
		return
	}

	if err := database.DeleteResume(r.Context(), profileID, resumeID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "resume not found")
			return
		}

		slog.Error("delete resume", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete resume")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// resumeExtractionTimeout bounds one extraction call, not the HTTP request it
// runs inside. Kept under a serverless function's max duration (60s on Vercel
// Hobby) since extraction now runs synchronously in-request, not in a
// detached goroutine.
const resumeExtractionTimeout = 45 * time.Second

func runResumeExtraction(resumeID int64, fileName, contentType string, content []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), resumeExtractionTimeout)
	defer cancel()

	if err := database.UpsertResumeExtractionPending(ctx, resumeID); err != nil {
		slog.Error("resume extraction: mark pending", "resume_id", resumeID, "error", err)
		return
	}

	extracted, err := llm.ExtractResumeFields(ctx, fileName, contentType, content)
	if err != nil {
		status := "failed"
		if errors.Is(err, llm.ErrUnsupportedFormat) {
			status = "unsupported"
		}

		saveCtx, saveCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer saveCancel()

		if dbErr := database.SaveResumeExtractionFailure(saveCtx, resumeID, status, err.Error()); dbErr != nil {
			slog.Error("resume extraction: save failure", "resume_id", resumeID, "error", dbErr)
		}
		return
	}

	if err := database.SaveResumeExtractionResult(ctx, resumeID, extracted); err != nil {
		slog.Error("resume extraction: save result", "resume_id", resumeID, "error", err)
		return
	}

	embedResumeExtraction(ctx, resumeID, extracted)
}

// embedResumeExtraction generates and stores a semantic embedding for a
// just-completed extraction. Best-effort: log and swallow failures here rather
// than flip the extraction back to failed, since the structured data already
// saved fine, and search/digest just fall back to keyword matching.
func embedResumeExtraction(ctx context.Context, resumeID int64, extracted *llm.ExtractedResume) {
	text := llm.EmbeddingText(llm.ResumeProfile{
		FullName:       extracted.FullName,
		Summary:        extracted.Summary,
		Skills:         extracted.Skills,
		WorkExperience: extracted.WorkExperience,
		Projects:       extracted.Projects,
	})

	embeddings, err := llm.EmbedTexts(ctx, []string{text})
	if err != nil {
		slog.Error("resume extraction: embed", "resume_id", resumeID, "error", err)
		return
	}

	if err := database.SaveResumeEmbedding(ctx, resumeID, embeddings[0]); err != nil {
		slog.Error("resume extraction: save embedding", "resume_id", resumeID, "error", err)
	}
}

func handleGetResumeExtraction(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	resumeID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid resume id")
		return
	}

	extraction, err := database.GetResumeExtraction(r.Context(), profileID, resumeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "resume extraction not found")
			return
		}

		slog.Error("get resume extraction", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get resume extraction")
		return
	}

	writeJSON(w, http.StatusOK, extraction)
}

func handleTriggerResumeExtraction(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	resumeID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid resume id")
		return
	}

	resume, content, err := database.GetResume(r.Context(), profileID, resumeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "resume not found")
			return
		}

		slog.Error("trigger resume extraction", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to trigger resume extraction")
		return
	}

	runResumeExtraction(resumeID, resume.FileName, resume.ContentType, content)

	writeJSON(w, http.StatusOK, map[string]string{"status": "pending"})
}

func handleActivateResume(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	resumeID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid resume id")
		return
	}

	if err := database.SetActiveResume(r.Context(), profileID, resumeID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "resume not found")
			return
		}

		slog.Error("activate resume", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to activate resume")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "activated"})
}

func readResumeUpload(w http.ResponseWriter, r *http.Request, requireFile bool) (string, string, []byte, error) {
	// A 10 MiB body does not fit in the server's 15s ReadTimeout on a slow
	// uplink, so give the upload its own read window.
	if err := http.NewResponseController(w).SetReadDeadline(time.Now().Add(resumeUploadReadTimeout)); err != nil {
		slog.Error("resume upload: extend read deadline", "error", err)
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxResumeSize+(1<<20))
	if err := r.ParseMultipartForm(maxResumeSize); err != nil {
		return "", "", nil, fmt.Errorf("resume upload must be 10 MB or smaller")
	}

	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	file, header, err := r.FormFile("resume")
	if err == http.ErrMissingFile && !requireFile {
		fileName, err := normalizeResumeName(r.FormValue("file_name"))
		if err != nil {
			return "", "", nil, err
		}
		return fileName, "", nil, nil
	}
	if err != nil {
		return "", "", nil, fmt.Errorf("a resume file is required")
	}
	defer file.Close()

	fileName, err := normalizeResumeName(header.Filename)
	if err != nil {
		return "", "", nil, err
	}

	content, err := io.ReadAll(io.LimitReader(file, maxResumeSize+1))
	if err != nil {
		return "", "", nil, fmt.Errorf("could not read resume")
	}
	if len(content) > maxResumeSize {
		return "", "", nil, fmt.Errorf("resume upload must be 10 MB or smaller")
	}

	return fileName, contentTypeForResume(fileName), content, nil
}

func normalizeResumeName(raw string) (string, error) {
	fileName := filepath.Base(strings.TrimSpace(raw))
	if fileName == "." || fileName == "" || len(fileName) > 255 {
		return "", fmt.Errorf("a valid resume file name is required")
	}

	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".pdf", ".doc", ".docx":
		return fileName, nil
	default:
		return "", fmt.Errorf("resume must be a PDF, DOC, or DOCX file")
	}
}

func contentTypeForResume(fileName string) string {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".pdf":
		return "application/pdf"
	case ".doc":
		return "application/msword"
	default:
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}
}
