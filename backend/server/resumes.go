package server

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"main/database"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

const maxResumeSize = 10 << 20 // 10 MiB

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
		log.Printf("upload resume: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to upload resume")
		return
	}

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
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "resume not found")
			return
		}

		log.Printf("update resume: %v", err)
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

		log.Printf("download resume: %v", err)
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

		log.Printf("delete resume: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete resume")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// readResumeUpload reads a multipart request. A replacement may omit the file and
// supply file_name to rename the existing resume.
func readResumeUpload(w http.ResponseWriter, r *http.Request, requireFile bool) (string, string, []byte, error) {
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
