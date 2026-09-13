package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"main/llm"
)

type TailoredResume struct {
	JobID     int64              `json:"job_id"`
	Content   llm.TailoredResume `json:"content"`
	Stale     bool               `json:"stale"`
	UpdatedAt time.Time          `json:"updated_at"`
}

// GetTailoredResume marks it stale if the source resume was deleted, deactivated, or re-extracted.
func GetTailoredResume(ctx context.Context, profileID, jobID int64) (TailoredResume, error) {
	db, err := GetDb()

	if err != nil {
		return TailoredResume{}, err
	}

	var tailored TailoredResume
	var content string

	err = db.QueryRowContext(ctx, `SELECT t.job_id, t.content, t.updated_at,
			(r.id IS NULL OR NOT r.is_active OR e.updated_at IS NULL OR e.updated_at > t.source_updated_at)
		FROM profile_tailored_resumes t
		LEFT JOIN profile_resumes r ON r.id = t.resume_id
		LEFT JOIN profile_resume_extractions e ON e.resume_id = t.resume_id
		WHERE t.profile_id = $1 AND t.job_id = $2`, profileID, jobID).Scan(
		&tailored.JobID, &content, &tailored.UpdatedAt, &tailored.Stale,
	)

	if err != nil {
		return TailoredResume{}, err
	}

	if err := json.Unmarshal([]byte(content), &tailored.Content); err != nil {
		return TailoredResume{}, fmt.Errorf("decode tailored resume: %w", err)
	}

	return tailored, nil
}

func UpsertTailoredResume(ctx context.Context, profileID, jobID, resumeID int64, sourceUpdatedAt time.Time, content llm.TailoredResume) error {
	db, err := GetDb()

	if err != nil {
		return err
	}

	encoded, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("encode tailored resume: %w", err)
	}

	_, err = db.ExecContext(ctx, `INSERT INTO profile_tailored_resumes (profile_id, job_id, resume_id, source_updated_at, content)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (profile_id, job_id) DO UPDATE SET
			resume_id = excluded.resume_id, source_updated_at = excluded.source_updated_at,
			content = excluded.content, updated_at = NOW()`,
		profileID, jobID, resumeID, sourceUpdatedAt, string(encoded))

	return err
}

func UpdateTailoredResumeContent(ctx context.Context, profileID, jobID int64, content llm.TailoredResume) error {
	db, err := GetDb()

	if err != nil {
		return err
	}

	encoded, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("encode tailored resume: %w", err)
	}

	result, err := db.ExecContext(ctx, `UPDATE profile_tailored_resumes SET content = $1, updated_at = NOW()
		WHERE profile_id = $2 AND job_id = $3`, string(encoded), profileID, jobID)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()

	if err != nil {
		return err
	}

	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}
