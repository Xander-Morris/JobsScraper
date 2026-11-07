package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	pg "github.com/go-jet/jet/v2/postgres"
	"github.com/pgvector/pgvector-go"

	"main/internal/gen/model"
	"main/internal/gen/table"
	"main/llm"
)

type ProfileResume struct {
	ID          int64     `json:"id"`
	FileName    string    `json:"file_name"`
	ContentType string    `json:"content_type"`
	FileSize    int64     `json:"file_size"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ResumeExtraction struct {
	ResumeID int64  `json:"resume_id"`
	Status   string `json:"status"`
	llm.ExtractedResume
	Error     string    `json:"error"`
	UpdatedAt time.Time `json:"updated_at"`
	// Embedding ranks job matches by meaning instead of keyword overlap.
	Embedding *pgvector.Vector `json:"-"`
}

func toProfileResume(row model.ProfileResumes) ProfileResume {
	return ProfileResume{
		ID:          int64(row.ID),
		FileName:    row.FileName,
		ContentType: row.ContentType,
		FileSize:    int64(row.FileSize),
		IsActive:    row.IsActive,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

// ownedResume matches the resume only when profileID owns it.
func ownedResume(resumeID, profileID int64) pg.BoolExpression {
	resumes := table.ProfileResumes

	return resumes.ID.EQ(pg.Int(resumeID)).AND(resumes.ProfileID.EQ(pg.Int(profileID)))
}

func ListResumes(ctx context.Context, profileID int64) ([]ProfileResume, error) {
	var rows []model.ProfileResumes

	resumes := table.ProfileResumes
	stmt := pg.SELECT(resumes.AllColumns.Except(resumes.Content)).FROM(resumes).
		WHERE(resumes.ProfileID.EQ(pg.Int(profileID))).
		ORDER_BY(resumes.UpdatedAt.DESC(), resumes.ID.DESC())

	if err := stmt.QueryContext(ctx, db(), &rows); err != nil {
		return nil, err
	}

	var list []ProfileResume

	for _, row := range rows {
		list = append(list, toProfileResume(row))
	}

	return list, nil
}

// AddResume marks the new resume active if it's the profile's first, so
// relevance sorting has something to work with right away.
func AddResume(ctx context.Context, profileID int64, fileName, contentType string, content []byte) (int64, error) {
	var row model.ProfileResumes

	resumes := table.ProfileResumes
	isFirst := pg.NOT(pg.EXISTS(pg.SELECT(resumes.ID).FROM(resumes).WHERE(resumes.ProfileID.EQ(pg.Int(profileID)))))
	stmt := resumes.INSERT(resumes.ProfileID, resumes.FileName, resumes.ContentType, resumes.FileSize, resumes.Content, resumes.IsActive).
		VALUES(pg.Int(profileID), pg.String(fileName), pg.String(contentType), pg.Int(int64(len(content))), pg.Bytea(content), isFirst).
		RETURNING(resumes.ID)

	err := queryOne(ctx, db(), stmt, &row)

	return int64(row.ID), err
}

// SetActiveResume makes resumeID the profile's one active resume (the relevance
// signal for job search) and unmarks whichever one was active before.
func SetActiveResume(ctx context.Context, profileID, resumeID int64) error {
	var owned model.ProfileResumes

	resumes := table.ProfileResumes

	if err := queryOne(ctx, db(), pg.SELECT(resumes.ID).FROM(resumes).WHERE(ownedResume(resumeID, profileID)), &owned); err != nil {
		return err
	}

	_, err := exec(ctx, db(), resumes.UPDATE().
		SET(resumes.IsActive.SET(resumes.ID.EQ(pg.Int(resumeID)))).
		WHERE(resumes.ProfileID.EQ(pg.Int(profileID))))

	return err
}

func ReplaceResume(ctx context.Context, profileID, resumeID int64, fileName, contentType string, content []byte) error {
	resumes := table.ProfileResumes

	return execOne(ctx, resumes.UPDATE().SET(
		resumes.FileName.SET(pg.String(fileName)),
		resumes.ContentType.SET(pg.String(contentType)),
		resumes.FileSize.SET(pg.Int(int64(len(content)))),
		resumes.Content.SET(pg.Bytea(content)),
		resumes.UpdatedAt.SET(pg.NOW()),
	).WHERE(ownedResume(resumeID, profileID)))
}

func RenameResume(ctx context.Context, profileID, resumeID int64, fileName string) error {
	resumes := table.ProfileResumes

	return execOne(ctx, resumes.UPDATE().
		SET(resumes.FileName.SET(pg.String(fileName)), resumes.UpdatedAt.SET(pg.NOW())).
		WHERE(ownedResume(resumeID, profileID)))
}

func GetResume(ctx context.Context, profileID, resumeID int64) (ProfileResume, []byte, error) {
	var row model.ProfileResumes

	resumes := table.ProfileResumes
	stmt := pg.SELECT(resumes.AllColumns.Except(resumes.IsActive)).FROM(resumes).WHERE(ownedResume(resumeID, profileID))

	err := queryOne(ctx, db(), stmt, &row)

	return toProfileResume(row), row.Content, err
}

func DeleteResume(ctx context.Context, profileID, resumeID int64) error {
	return execOne(ctx, table.ProfileResumes.DELETE().WHERE(ownedResume(resumeID, profileID)))
}

func UpsertResumeExtractionPending(ctx context.Context, resumeID int64) error {
	extractions := table.ProfileResumeExtractions
	_, err := exec(ctx, db(), extractions.INSERT(extractions.ResumeID, extractions.Status).
		VALUES(pg.Int(resumeID), pg.String("pending")).
		ON_CONFLICT(extractions.ResumeID).DO_UPDATE(pg.SET(
		extractions.Status.SET(pg.String("pending")),
		extractions.Error.SET(pg.StringExp(pg.NULL)),
		extractions.UpdatedAt.SET(pg.NOW()),
	)))

	return err
}

// saveResumeExtraction upserts the cols of row, bumping updated_at on conflict.
func saveResumeExtraction(ctx context.Context, cols pg.ColumnList, row model.ProfileResumeExtractions) error {
	extractions := table.ProfileResumeExtractions
	_, err := exec(ctx, db(), extractions.INSERT(cols).MODEL(row).
		ON_CONFLICT(extractions.ResumeID).DO_UPDATE(pg.SET(
		setExcluded(cols, extractions.EXCLUDED.AllColumns, extractions.ResumeID),
		extractions.UpdatedAt.SET(pg.NOW()),
	)))

	return err
}

func SaveResumeExtractionResult(ctx context.Context, resumeID int64, extracted *llm.ExtractedResume) error {
	extractions := table.ProfileResumeExtractions

	// Every extracted field, with error cleared; timestamps and the embedding are kept.
	cols := extractions.MutableColumns.Except(extractions.CreatedAt, extractions.UpdatedAt, extractions.Embedding)

	return saveResumeExtraction(ctx, cols, model.ProfileResumeExtractions{
		ResumeID:       int32(resumeID),
		Status:         "completed",
		FullName:       &extracted.FullName,
		Email:          &extracted.Email,
		Phone:          &extracted.Phone,
		LinkedIn:       &extracted.LinkedIn,
		Github:         &extracted.GitHub,
		Portfolio:      &extracted.Portfolio,
		Summary:        &extracted.Summary,
		Skills:         new(encodeJSON(extracted.Skills)),
		Education:      new(encodeJSON(extracted.Education)),
		WorkExperience: new(encodeJSON(extracted.WorkExperience)),
		Projects:       new(encodeJSON(extracted.Projects)),
	})
}

func SaveResumeExtractionFailure(ctx context.Context, resumeID int64, status, errMsg string) error {
	extractions := table.ProfileResumeExtractions

	return saveResumeExtraction(ctx, pg.ColumnList{extractions.ResumeID, extractions.Status, extractions.Error},
		model.ProfileResumeExtractions{ResumeID: int32(resumeID), Status: status, Error: &errMsg})
}

// SaveResumeEmbedding stores the semantic vector for an already-completed
// extraction. Best-effort: caller should log and swallow failures here rather
// than treat them as extraction failures, since search/digest just fall back
// to keyword matching without an embedding.
func SaveResumeEmbedding(ctx context.Context, resumeID int64, embedding []float32) error {
	extractions := table.ProfileResumeExtractions
	_, err := exec(ctx, db(), extractions.UPDATE().
		SET(extractions.Embedding.SET(pg.StringExp(arg(pgvector.NewVector(embedding), "vector")))).
		WHERE(extractions.ResumeID.EQ(pg.Int(resumeID))))

	return err
}

// getResumeExtraction loads the one extraction matching where, joined to its resume.
func getResumeExtraction(ctx context.Context, where pg.BoolExpression) (ResumeExtraction, error) {
	var row struct {
		model.ProfileResumeExtractions
		Vector *pgvector.Vector
	}

	extractions, resumes := table.ProfileResumeExtractions, table.ProfileResumes
	stmt := pg.SELECT(extractions.AllColumns.Except(extractions.Embedding), extractions.Embedding.AS("vector")).
		FROM(extractions.INNER_JOIN(resumes, resumes.ID.EQ(extractions.ResumeID))).
		WHERE(where)

	if err := queryOne(ctx, db(), stmt, &row); err != nil {
		return ResumeExtraction{}, err
	}

	extraction := ResumeExtraction{
		ResumeID:  int64(row.ResumeID),
		Status:    row.Status,
		Error:     deref(row.Error),
		UpdatedAt: row.UpdatedAt,
		Embedding: row.Vector,
	}
	extraction.FullName = deref(row.FullName)
	extraction.Email = deref(row.Email)
	extraction.Phone = deref(row.Phone)
	extraction.LinkedIn = deref(row.LinkedIn)
	extraction.GitHub = deref(row.Github)
	extraction.Portfolio = deref(row.Portfolio)
	extraction.Summary = deref(row.Summary)

	for _, field := range []struct {
		raw  *string
		dest any
		what string
	}{
		{row.Skills, &extraction.Skills, "skills"},
		{row.Education, &extraction.Education, "education"},
		{row.WorkExperience, &extraction.WorkExperience, "work experience"},
		{row.Projects, &extraction.Projects, "projects"},
	} {
		if err := decodeJSON(field.raw, field.dest, field.what); err != nil {
			return ResumeExtraction{}, err
		}
	}

	return extraction, nil
}

func GetResumeExtraction(ctx context.Context, profileID, resumeID int64) (ResumeExtraction, error) {
	extractions, resumes := table.ProfileResumeExtractions, table.ProfileResumes

	return getResumeExtraction(ctx, extractions.ResumeID.EQ(pg.Int(resumeID)).AND(resumes.ProfileID.EQ(pg.Int(profileID))))
}

// GetActiveResumeExtraction returns the completed extraction for the profile's
// active resume, if any. found is false (with no error) when the profile has no
// active resume, or its extraction isn't completed yet. Both are normal,
// expected states, not failures.
func GetActiveResumeExtraction(ctx context.Context, profileID int64) (extraction ResumeExtraction, found bool, err error) {
	extractions, resumes := table.ProfileResumeExtractions, table.ProfileResumes

	// One round trip, since this runs ahead of every signed-in job search.
	extraction, err = getResumeExtraction(ctx, resumes.ProfileID.EQ(pg.Int(profileID)).
		AND(resumes.IsActive).
		AND(extractions.Status.EQ(pg.String("completed"))))

	if errors.Is(err, sql.ErrNoRows) {
		return ResumeExtraction{}, false, nil
	}
	if err != nil {
		return ResumeExtraction{}, false, err
	}

	return extraction, true, nil
}
