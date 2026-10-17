package database

import (
	"database/sql"
	"errors"
	"slices"
	"testing"

	"main/llm"
)

func TestListResumesAndSetActive(t *testing.T) {
	newTestDB(t)
	ctx := t.Context()
	id := newTestProfile(t)
	other := newNamedProfile(t, "other@example.com")

	if resumes, err := ListResumes(ctx, id); err != nil || len(resumes) != 0 {
		t.Fatalf("ListResumes(empty) = %v, %v", resumes, err)
	}

	first, err := AddResume(ctx, id, "a.pdf", "application/pdf", []byte("aaa"))
	if err != nil {
		t.Fatal(err)
	}

	second, err := AddResume(ctx, id, "b.pdf", "application/pdf", []byte("bbbb"))
	if err != nil {
		t.Fatal(err)
	}

	resumes, err := ListResumes(ctx, id)
	if err != nil {
		t.Fatal(err)
	}

	if len(resumes) != 2 {
		t.Fatalf("ListResumes = %d, want 2", len(resumes))
	}

	active := map[int64]bool{}
	for _, r := range resumes {
		active[r.ID] = r.IsActive
		if r.ID == second && r.FileSize != 4 {
			t.Errorf("FileSize = %d, want 4", r.FileSize)
		}
	}

	if !active[first] || active[second] {
		t.Errorf("active = %v, want only the first resume active", active)
	}

	if err := SetActiveResume(ctx, id, second); err != nil {
		t.Fatalf("SetActiveResume: %v", err)
	}

	resumes, _ = ListResumes(ctx, id)
	for _, r := range resumes {
		if r.IsActive != (r.ID == second) {
			t.Errorf("resume %d IsActive = %v after switching", r.ID, r.IsActive)
		}
	}

	if err := SetActiveResume(ctx, other, second); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("SetActiveResume(other profile) = %v, want sql.ErrNoRows", err)
	}

	if err := SetActiveResume(ctx, id, 9999); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("SetActiveResume(missing) = %v, want sql.ErrNoRows", err)
	}
}

func TestResumeExtractionLifecycle(t *testing.T) {
	newTestDB(t)
	ctx := t.Context()
	id := newTestProfile(t)
	other := newNamedProfile(t, "other@example.com")

	resumeID, err := AddResume(ctx, id, "a.pdf", "application/pdf", []byte("x"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := GetResumeExtraction(ctx, id, resumeID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetResumeExtraction(none) = %v, want sql.ErrNoRows", err)
	}

	if err := UpsertResumeExtractionPending(ctx, resumeID); err != nil {
		t.Fatal(err)
	}

	got, err := GetResumeExtraction(ctx, id, resumeID)
	if err != nil || got.Status != "pending" {
		t.Fatalf("pending extraction = %+v, %v", got, err)
	}

	if _, found, err := GetActiveResumeExtraction(ctx, id); err != nil || found {
		t.Errorf("GetActiveResumeExtraction(pending) found=%v err=%v, want not found", found, err)
	}

	if err := SaveResumeExtractionFailure(ctx, resumeID, "failed", "boom"); err != nil {
		t.Fatal(err)
	}

	got, _ = GetResumeExtraction(ctx, id, resumeID)
	if got.Status != "failed" || got.Error != "boom" {
		t.Errorf("failed extraction = %+v", got)
	}

	// Retrying clears the old error.
	if err := UpsertResumeExtractionPending(ctx, resumeID); err != nil {
		t.Fatal(err)
	}

	got, _ = GetResumeExtraction(ctx, id, resumeID)
	if got.Status != "pending" || got.Error != "" {
		t.Errorf("re-pending extraction = %+v", got)
	}

	extracted := &llm.ExtractedResume{
		FullName:  "Ada Lovelace",
		Email:     "ada@example.com",
		Phone:     "555",
		LinkedIn:  "https://linkedin.com/in/ada",
		GitHub:    "https://github.com/ada",
		Portfolio: "https://ada.dev",
		Summary:   "Engineer",
		Skills:    []string{"Go", "SQL"},
		Education: []llm.EducationEntry{{SchoolName: "MIT", Degree: "BS", Major: "CS"}},
		WorkExperience: []llm.WorkExperienceEntry{
			{Company: "Acme", JobTitle: "Dev", Bullets: []string{"Built it"}},
		},
		Projects: []llm.ProjectEntry{{Name: "Engine", Technologies: []string{"Go"}}},
	}

	if err := SaveResumeExtractionResult(ctx, resumeID, extracted); err != nil {
		t.Fatal(err)
	}

	got, err = GetResumeExtraction(ctx, id, resumeID)
	if err != nil {
		t.Fatal(err)
	}

	if got.Status != "completed" || got.FullName != "Ada Lovelace" || got.Portfolio != "https://ada.dev" ||
		!slices.Equal(got.Skills, extracted.Skills) || len(got.Education) != 1 || got.Education[0].SchoolName != "MIT" ||
		len(got.WorkExperience) != 1 || got.WorkExperience[0].Bullets[0] != "Built it" ||
		len(got.Projects) != 1 || got.Projects[0].Name != "Engine" || got.Embedding != nil {
		t.Errorf("completed extraction = %+v", got)
	}

	if _, err := GetResumeExtraction(ctx, other, resumeID); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetResumeExtraction(other profile) = %v, want sql.ErrNoRows", err)
	}

	active, found, err := GetActiveResumeExtraction(ctx, id)
	if err != nil || !found || active.ResumeID != resumeID {
		t.Errorf("GetActiveResumeExtraction = %+v, %v, %v", active, found, err)
	}

	if err := SaveResumeEmbedding(ctx, resumeID, unitVector(3)); err != nil {
		t.Fatalf("SaveResumeEmbedding: %v", err)
	}

	got, _ = GetResumeExtraction(ctx, id, resumeID)
	if got.Embedding == nil || got.Embedding.Slice()[3] != 1 {
		t.Errorf("embedding not saved: %v", got.Embedding)
	}
}

func TestGetActiveResumeExtractionIgnoresInactive(t *testing.T) {
	newTestDB(t)
	ctx := t.Context()
	id := newTestProfile(t)

	first, _ := AddResume(ctx, id, "a.pdf", "application/pdf", []byte("x"))
	second, _ := AddResume(ctx, id, "b.pdf", "application/pdf", []byte("y"))

	if err := SaveResumeExtractionResult(ctx, second, &llm.ExtractedResume{FullName: "Second"}); err != nil {
		t.Fatal(err)
	}

	if _, found, err := GetActiveResumeExtraction(ctx, id); err != nil || found {
		t.Errorf("found=%v err=%v; only the inactive resume has an extraction", found, err)
	}

	if err := SaveResumeExtractionResult(ctx, first, &llm.ExtractedResume{FullName: "First"}); err != nil {
		t.Fatal(err)
	}

	got, found, err := GetActiveResumeExtraction(ctx, id)
	if err != nil || !found || got.FullName != "First" {
		t.Errorf("active extraction = %+v, %v, %v", got, found, err)
	}
}

func TestResumeOwnershipChecks(t *testing.T) {
	newTestDB(t)
	ctx := t.Context()
	id := newTestProfile(t)
	other := newNamedProfile(t, "other@example.com")

	resumeID, _ := AddResume(ctx, id, "a.pdf", "application/pdf", []byte("x"))

	if err := RenameResume(ctx, other, resumeID, "stolen.pdf"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("RenameResume(other) = %v", err)
	}

	if err := ReplaceResume(ctx, other, resumeID, "x.pdf", "application/pdf", []byte("z")); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("ReplaceResume(other) = %v", err)
	}

	if _, _, err := GetResume(ctx, other, resumeID); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetResume(other) = %v", err)
	}

	if err := DeleteResume(ctx, other, resumeID); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("DeleteResume(other) = %v", err)
	}

	if _, _, err := GetResume(ctx, id, resumeID); err != nil {
		t.Errorf("owner lost the resume: %v", err)
	}
}
