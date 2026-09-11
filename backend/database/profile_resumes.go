package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pgvector/pgvector-go"

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
	ResumeID       int64                     `json:"resume_id"`
	Status         string                    `json:"status"`
	FullName       string                    `json:"full_name"`
	Email          string                    `json:"email"`
	Phone          string                    `json:"phone"`
	LinkedIn       string                    `json:"linked_in"`
	GitHub         string                    `json:"github"`
	Portfolio      string                    `json:"portfolio"`
	Summary        string                    `json:"summary"`
	Skills         []string                  `json:"skills"`
	Education      []llm.EducationEntry      `json:"education"`
	WorkExperience []llm.WorkExperienceEntry `json:"work_experience"`
	Projects       []llm.ProjectEntry        `json:"projects"`
	Error          string                    `json:"error"`
	UpdatedAt      time.Time                 `json:"updated_at"`
	// Embedding ranks job matches by meaning instead of keyword overlap. Nil
	// until the background embedding step finishes after extraction, so fall
	// back to keyword matching, don't treat nil as an error.
	Embedding *pgvector.Vector `json:"-"`
}

func ListResumes(ctx context.Context, profileID int64) ([]ProfileResume, error) {
	db, err := GetDb()

	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, `SELECT id, file_name, content_type, file_size, is_active, created_at, updated_at
		FROM profile_resumes WHERE profile_id = $1 ORDER BY updated_at DESC, id DESC`, profileID)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var resumes []ProfileResume

	for rows.Next() {
		var resume ProfileResume

		if err := rows.Scan(&resume.ID, &resume.FileName, &resume.ContentType, &resume.FileSize, &resume.IsActive, &resume.CreatedAt, &resume.UpdatedAt); err != nil {
			return nil, err
		}

		resumes = append(resumes, resume)
	}

	return resumes, rows.Err()
}

// AddResume marks the new resume active if it's the profile's first, so
// relevance sorting has something to work with right away.
func AddResume(ctx context.Context, profileID int64, fileName, contentType string, content []byte) (int64, error) {
	db, err := GetDb()

	if err != nil {
		return 0, err
	}

	var id int64
	err = db.QueryRowContext(ctx, `INSERT INTO profile_resumes (profile_id, file_name, content_type, file_size, content, is_active)
		VALUES ($1, $2, $3, $4, $5, NOT EXISTS (SELECT 1 FROM profile_resumes WHERE profile_id = $1))
		RETURNING id`, profileID, fileName, contentType, len(content), content).Scan(&id)

	return id, err
}

// SetActiveResume makes resumeID the profile's one active resume (the relevance
// signal for job search) and unmarks whichever one was active before.
func SetActiveResume(ctx context.Context, profileID, resumeID int64) error {
	db, err := GetDb()

	if err != nil {
		return err
	}

	var exists bool
	err = db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM profile_resumes WHERE id = $1 AND profile_id = $2)`,
		resumeID, profileID).Scan(&exists)

	if err != nil {
		return err
	}

	if !exists {
		return sql.ErrNoRows
	}

	_, err = db.ExecContext(ctx, `UPDATE profile_resumes SET is_active = (id = $2) WHERE profile_id = $1`, profileID, resumeID)

	return err
}

func ReplaceResume(ctx context.Context, profileID, resumeID int64, fileName, contentType string, content []byte) error {
	db, err := GetDb()

	if err != nil {
		return err
	}

	result, err := db.ExecContext(ctx, `UPDATE profile_resumes
		SET file_name = $1, content_type = $2, file_size = $3, content = $4, updated_at = NOW()
		WHERE id = $5 AND profile_id = $6`, fileName, contentType, len(content), content, resumeID, profileID)

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

func RenameResume(ctx context.Context, profileID, resumeID int64, fileName string) error {
	db, err := GetDb()

	if err != nil {
		return err
	}

	result, err := db.ExecContext(ctx, `UPDATE profile_resumes SET file_name = $1, updated_at = NOW()
		WHERE id = $2 AND profile_id = $3`, fileName, resumeID, profileID)

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

func GetResume(ctx context.Context, profileID, resumeID int64) (ProfileResume, []byte, error) {
	db, err := GetDb()

	if err != nil {
		return ProfileResume{}, nil, err
	}

	var resume ProfileResume
	var content []byte
	err = db.QueryRowContext(ctx, `SELECT id, file_name, content_type, file_size, content, created_at, updated_at
		FROM profile_resumes WHERE id = $1 AND profile_id = $2`, resumeID, profileID).Scan(
		&resume.ID, &resume.FileName, &resume.ContentType, &resume.FileSize, &content, &resume.CreatedAt, &resume.UpdatedAt,
	)

	return resume, content, err
}

func DeleteResume(ctx context.Context, profileID, resumeID int64) error {
	db, err := GetDb()

	if err != nil {
		return err
	}

	result, err := db.ExecContext(ctx, `DELETE FROM profile_resumes WHERE id = $1 AND profile_id = $2`, resumeID, profileID)

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

func UpsertResumeExtractionPending(ctx context.Context, resumeID int64) error {
	db, err := GetDb()

	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, `INSERT INTO profile_resume_extractions (resume_id, status)
		VALUES ($1, 'pending')
		ON CONFLICT (resume_id) DO UPDATE SET status = 'pending', error = NULL, updated_at = NOW()`, resumeID)

	return err
}

func SaveResumeExtractionResult(ctx context.Context, resumeID int64, extracted *llm.ExtractedResume) error {
	db, err := GetDb()

	if err != nil {
		return err
	}

	skills, err := json.Marshal(extracted.Skills)
	if err != nil {
		return fmt.Errorf("encode skills: %w", err)
	}

	education, err := json.Marshal(extracted.Education)
	if err != nil {
		return fmt.Errorf("encode education: %w", err)
	}

	workExperience, err := json.Marshal(extracted.WorkExperience)
	if err != nil {
		return fmt.Errorf("encode work experience: %w", err)
	}

	projects, err := json.Marshal(extracted.Projects)
	if err != nil {
		return fmt.Errorf("encode projects: %w", err)
	}

	_, err = db.ExecContext(ctx, `INSERT INTO profile_resume_extractions
			(resume_id, status, full_name, email, phone, linked_in, github, portfolio, summary, skills, education, work_experience, projects, error)
		VALUES ($1, 'completed', $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NULL)
		ON CONFLICT (resume_id) DO UPDATE SET
			status = 'completed', full_name = excluded.full_name, email = excluded.email,
			phone = excluded.phone, linked_in = excluded.linked_in, github = excluded.github,
			portfolio = excluded.portfolio, summary = excluded.summary, skills = excluded.skills,
			education = excluded.education, work_experience = excluded.work_experience,
			projects = excluded.projects, error = NULL, updated_at = NOW()`,
		resumeID, extracted.FullName, extracted.Email, extracted.Phone, extracted.LinkedIn, extracted.GitHub, extracted.Portfolio,
		extracted.Summary, string(skills), string(education), string(workExperience), string(projects))

	return err
}

func SaveResumeExtractionFailure(ctx context.Context, resumeID int64, status, errMsg string) error {
	db, err := GetDb()

	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, `INSERT INTO profile_resume_extractions (resume_id, status, error)
		VALUES ($1, $2, $3)
		ON CONFLICT (resume_id) DO UPDATE SET status = excluded.status, error = excluded.error, updated_at = NOW()`,
		resumeID, status, errMsg)

	return err
}

// SaveResumeEmbedding stores the semantic vector for an already-completed
// extraction. Best-effort: caller should log and swallow failures here rather
// than treat them as extraction failures, since search/digest just fall back
// to keyword matching without an embedding.
func SaveResumeEmbedding(ctx context.Context, resumeID int64, embedding []float32) error {
	db, err := GetDb()

	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, `UPDATE profile_resume_extractions SET embedding = $1 WHERE resume_id = $2`,
		pgvector.NewVector(embedding), resumeID)

	return err
}

func GetResumeExtraction(ctx context.Context, profileID, resumeID int64) (ResumeExtraction, error) {
	db, err := GetDb()

	if err != nil {
		return ResumeExtraction{}, err
	}

	var extraction ResumeExtraction
	var fullName, email, phone, linkedIn, github, portfolio, summary, errMsg sql.NullString
	var skills, education, workExperience, projects sql.NullString
	var embedding sql.Null[pgvector.Vector]

	err = db.QueryRowContext(ctx, `SELECT e.resume_id, e.status, e.full_name, e.email, e.phone,
			e.linked_in, e.github, e.portfolio, e.summary,
			e.skills, e.education, e.work_experience, e.projects, e.error, e.updated_at, e.embedding
		FROM profile_resume_extractions e
		JOIN profile_resumes r ON r.id = e.resume_id
		WHERE e.resume_id = $1 AND r.profile_id = $2`, resumeID, profileID).Scan(
		&extraction.ResumeID, &extraction.Status, &fullName, &email, &phone,
		&linkedIn, &github, &portfolio, &summary,
		&skills, &education, &workExperience, &projects, &errMsg, &extraction.UpdatedAt, &embedding,
	)

	if err != nil {
		return ResumeExtraction{}, err
	}

	extraction.FullName = fullName.String
	extraction.Email = email.String
	extraction.Phone = phone.String
	extraction.LinkedIn = linkedIn.String
	extraction.GitHub = github.String
	extraction.Portfolio = portfolio.String
	extraction.Summary = summary.String
	extraction.Error = errMsg.String

	if skills.Valid {
		if err := json.Unmarshal([]byte(skills.String), &extraction.Skills); err != nil {
			return ResumeExtraction{}, fmt.Errorf("decode skills: %w", err)
		}
	}
	if education.Valid {
		if err := json.Unmarshal([]byte(education.String), &extraction.Education); err != nil {
			return ResumeExtraction{}, fmt.Errorf("decode education: %w", err)
		}
	}
	if workExperience.Valid {
		if err := json.Unmarshal([]byte(workExperience.String), &extraction.WorkExperience); err != nil {
			return ResumeExtraction{}, fmt.Errorf("decode work experience: %w", err)
		}
	}
	if projects.Valid {
		if err := json.Unmarshal([]byte(projects.String), &extraction.Projects); err != nil {
			return ResumeExtraction{}, fmt.Errorf("decode projects: %w", err)
		}
	}
	if embedding.Valid {
		extraction.Embedding = &embedding.V
	}

	return extraction, nil
}

// GetActiveResumeExtraction returns the completed extraction for the profile's
// active resume, if any. found is false (with no error) when the profile has no
// active resume, or its extraction isn't completed yet. Both are normal,
// expected states, not failures.
func GetActiveResumeExtraction(ctx context.Context, profileID int64) (extraction ResumeExtraction, found bool, err error) {
	db, err := GetDb()

	if err != nil {
		return ResumeExtraction{}, false, err
	}

	var resumeID int64
	err = db.QueryRowContext(ctx, `SELECT id FROM profile_resumes WHERE profile_id = $1 AND is_active`, profileID).Scan(&resumeID)

	if err == sql.ErrNoRows {
		return ResumeExtraction{}, false, nil
	}
	if err != nil {
		return ResumeExtraction{}, false, err
	}

	extraction, err = GetResumeExtraction(ctx, profileID, resumeID)

	if err == sql.ErrNoRows {
		return ResumeExtraction{}, false, nil
	}
	if err != nil {
		return ResumeExtraction{}, false, err
	}

	if extraction.Status != "completed" {
		return ResumeExtraction{}, false, nil
	}

	return extraction, true, nil
}
