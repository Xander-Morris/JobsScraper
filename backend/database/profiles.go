package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	emailverifier "github.com/AfterShip/email-verifier"
	"github.com/pgvector/pgvector-go"
	"golang.org/x/crypto/bcrypt"

	"main/llm"
)

const dateLayout = "2006-01-02"

type Profile struct {
	ID                 int64                   `json:"id"`
	Email              string                  `json:"email"`
	Name               string                  `json:"name"`
	Address            string                  `json:"address"`
	LinkedIn           string                  `json:"linked_in"`
	GitHub             string                  `json:"github"`
	Portfolio          string                  `json:"portfolio"`
	EmailNotifications bool                    `json:"email_notifications"`
	Education          []ProfileEducation      `json:"education"`
	Skills             []ProfileSkill          `json:"skills"`
	WorkExperience     []ProfileWorkExperience `json:"work_experience"`
	Resumes            []ProfileResume         `json:"resumes"`
}

type ProfileResume struct {
	ID          int64     `json:"id"`
	FileName    string    `json:"file_name"`
	ContentType string    `json:"content_type"`
	FileSize    int64     `json:"file_size"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ProfileEducation struct {
	ID         int64    `json:"id"`
	SchoolName string   `json:"school_name"`
	Major      string   `json:"major"`
	Degree     string   `json:"degree"`
	GPA        *float64 `json:"gpa"`
	StartDate  *string  `json:"start_date"`
	EndDate    *string  `json:"end_date"`
}

type ProfileSkill struct {
	ID    int64  `json:"id"`
	Skill string `json:"skill"`
}

type ProfileWorkExperienceBullet struct {
	ID       int64  `json:"id"`
	Bullet   string `json:"bullet"`
	Position int    `json:"position"`
}

type ProfileWorkExperience struct {
	ID        int64                         `json:"id"`
	Company   string                        `json:"company"`
	JobTitle  string                        `json:"job_title"`
	JobType   JobType                       `json:"job_type"`
	Location  *string                       `json:"location"`
	StartDate *string                       `json:"start_date"`
	EndDate   *string                       `json:"end_date"`
	Bullets   []ProfileWorkExperienceBullet `json:"bullets"`
}

type UpdateProfileRequest struct {
	Name               string `json:"name"`
	Address            string `json:"address"`
	LinkedIn           string `json:"linked_in"`
	GitHub             string `json:"github"`
	Portfolio          string `json:"portfolio"`
	EmailNotifications bool   `json:"email_notifications"`
}

type AddEducationRequest struct {
	SchoolName string   `json:"school_name"`
	Major      string   `json:"major"`
	Degree     string   `json:"degree"`
	GPA        *float64 `json:"gpa"`
	StartDate  string   `json:"start_date"`
	EndDate    string   `json:"end_date"`
}

type AddSkillRequest struct {
	Skill string `json:"skill"`
}

type AddWorkExperienceRequest struct {
	Company   string `json:"company"`
	JobTitle  string `json:"job_title"`
	JobType   string `json:"job_type"`
	Location  string `json:"location"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type AddWorkExperienceBulletRequest struct {
	Bullet   string `json:"bullet"`
	Position int    `json:"position"`
}

var verifier = emailverifier.NewVerifier()

type ProfileRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))

	return err == nil
}

// Compared against on a lookup miss so login timing doesn't leak whether an email exists.
var DummyPasswordHash = mustHashPassword("this is not anybody's real password")

func mustHashPassword(password string) string {
	hash, err := HashPassword(password)

	if err != nil {
		panic(fmt.Errorf("hash dummy password: %w", err))
	}

	return hash
}

var ErrInvalidProfile = errors.New("invalid profile request")
var errEmailVerificationTimeout = errors.New("email verification timed out")

const emailVerificationTimeout = 8 * time.Second

func verifyEmail(email string) (*emailverifier.Result, error) {
	type outcome struct {
		result *emailverifier.Result
		err    error
	}

	done := make(chan outcome, 1)

	go func() {
		result, err := verifier.Verify(email)
		done <- outcome{result, err}
	}()

	select {
	case o := <-done:
		return o.result, o.err
	case <-time.After(emailVerificationTimeout):
		return nil, errEmailVerificationTimeout
	}
}

func GetProfileByEmail(email string) (int64, string, error) {
	db, err := GetDb()

	if err != nil {
		return 0, "", err
	}

	var id int64
	var password string

	if err := db.QueryRow("SELECT id, password FROM profiles WHERE email=$1", email).Scan(&id, &password); err != nil {
		return 0, "", err
	}

	return id, password, nil
}

func CreateProfile(req *ProfileRequest) (int64, error) {
	if len(req.Email) == 0 || len(req.Password) == 0 {
		return 0, fmt.Errorf("%w: email and password are required", ErrInvalidProfile)
	}
	if len(req.Password) < 8 {
		return 0, fmt.Errorf("%w: password must be at least 8 characters", ErrInvalidProfile)
	}

	result, err := verifyEmail(req.Email)
	if errors.Is(err, errEmailVerificationTimeout) {
		if !verifier.ParseAddress(req.Email).Valid {
			return 0, fmt.Errorf("%w: email is invalid or undeliverable", ErrInvalidProfile)
		}
	} else if err != nil {
		return 0, fmt.Errorf("verify email: %w", err)
	} else if !result.Syntax.Valid || !result.HasMxRecords || result.Disposable {
		return 0, fmt.Errorf("%w: email is invalid or undeliverable", ErrInvalidProfile)
	}

	db, err := GetDb()

	if err != nil {
		return 0, err
	}

	if err := CreateTables(); err != nil {
		return 0, err
	}

	tx, err := db.Begin()

	if err != nil {
		return 0, err
	}

	defer tx.Rollback()

	hashedPassword, err := HashPassword(req.Password)

	if err != nil {
		return 0, err
	}

	var profileID int64

	if err := tx.QueryRow(insertStatements["profiles"], req.Email, hashedPassword).Scan(&profileID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("%w: email already registered", ErrInvalidProfile)
		}

		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return profileID, nil
}

func ProfileExists(ctx context.Context, id int64) (bool, error) {
	db, err := GetDb()
	if err != nil {
		return false, err
	}

	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM profiles WHERE id = $1)`, id).Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}

func GetProfile(ctx context.Context, id int64) (*Profile, error) {
	db, err := GetDb()

	if err != nil {
		return nil, err
	}

	profile := &Profile{ID: id}

	row := db.QueryRowContext(ctx, `SELECT email, COALESCE(name, ''), COALESCE(address, ''),
		COALESCE(linked_in, ''), COALESCE(github, ''), COALESCE(portfolio, ''), email_notifications_enabled
		FROM profiles WHERE id = $1`, id)

	if err := row.Scan(&profile.Email, &profile.Name, &profile.Address, &profile.LinkedIn, &profile.GitHub,
		&profile.Portfolio, &profile.EmailNotifications); err != nil {
		return nil, err
	}

	education, err := listEducation(ctx, db, id)

	if err != nil {
		return nil, fmt.Errorf("list education: %w", err)
	}

	profile.Education = education

	skills, err := listSkills(ctx, db, id)

	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}

	profile.Skills = skills

	workExperience, err := listWorkExperience(ctx, db, id)

	if err != nil {
		return nil, fmt.Errorf("list work experience: %w", err)
	}

	profile.WorkExperience = workExperience

	resumes, err := ListResumes(ctx, id)

	if err != nil {
		return nil, fmt.Errorf("list resumes: %w", err)
	}

	profile.Resumes = resumes

	return profile, nil
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

// AddResume marks the new resume active if the profile has no active resume yet
// (i.e. this is their first resume), so relevance sorting has something to work
// with immediately without requiring an extra manual step.
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

// SetActiveResume marks resumeID as the profile's one active resume, used as the
// relevance signal for job search, and unmarks any previously active resume.
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
	// Embedding is the resume's semantic vector, used to rank job matches by
	// meaning instead of keyword overlap. Nil until the background embedding
	// step (kicked off after extraction completes) finishes — callers should
	// fall back to keyword-based matching when it's nil, not treat it as an error.
	Embedding *pgvector.Vector `json:"-"`
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

// SaveResumeEmbedding stores the semantic vector for an already-completed resume
// extraction. Called best-effort after extraction succeeds — a failure here
// should be logged and swallowed by the caller, not surfaced as an extraction
// failure, since the structured extraction itself is unaffected and search/digest
// both fall back to keyword matching when no embedding is present.
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
// active resume, or its extraction isn't completed yet — both are normal,
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

func UpdateProfile(ctx context.Context, id int64, req *UpdateProfileRequest) error {
	db, err := GetDb()

	if err != nil {
		return err
	}

	result, err := db.ExecContext(ctx, `UPDATE profiles SET name = $1, address = $2, linked_in = $3, github = $4,
		portfolio = $5, email_notifications_enabled = $6 WHERE id = $7`,
		req.Name, req.Address, req.LinkedIn, req.GitHub, req.Portfolio, req.EmailNotifications, id)

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

func listEducation(ctx context.Context, db *sql.DB, profileID int64) ([]ProfileEducation, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, school_name, major, degree, gpa, start_date, end_date
		FROM profiles_education WHERE profile_id = $1 ORDER BY start_date DESC NULLS LAST, id`, profileID)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var education []ProfileEducation

	for rows.Next() {
		var entry ProfileEducation
		var gpa sql.NullFloat64
		var startDate, endDate sql.NullTime

		if err := rows.Scan(&entry.ID, &entry.SchoolName, &entry.Major, &entry.Degree, &gpa, &startDate, &endDate); err != nil {
			return nil, err
		}

		if gpa.Valid {
			entry.GPA = &gpa.Float64
		}

		if startDate.Valid {
			formatted := startDate.Time.Format(dateLayout)
			entry.StartDate = &formatted
		}

		if endDate.Valid {
			formatted := endDate.Time.Format(dateLayout)
			entry.EndDate = &formatted
		}

		education = append(education, entry)
	}

	return education, rows.Err()
}

func AddEducation(ctx context.Context, profileID int64, req *AddEducationRequest) (int64, error) {
	if req.SchoolName == "" || req.Major == "" || req.Degree == "" {
		return 0, fmt.Errorf("school_name, major, and degree are required")
	}

	startDate, err := parseOptionalDate(req.StartDate)

	if err != nil {
		return 0, fmt.Errorf("invalid start_date: %w", err)
	}

	endDate, err := parseOptionalDate(req.EndDate)

	if err != nil {
		return 0, fmt.Errorf("invalid end_date: %w", err)
	}

	db, err := GetDb()

	if err != nil {
		return 0, err
	}

	var educationID int64

	err = db.QueryRowContext(ctx, insertStatements["profiles_education"],
		profileID, req.SchoolName, req.Major, req.Degree, req.GPA, startDate, endDate).Scan(&educationID)

	if err != nil {
		return 0, err
	}

	return educationID, nil
}

func UpdateEducation(ctx context.Context, profileID, educationID int64, req *AddEducationRequest) error {
	if req.SchoolName == "" || req.Major == "" || req.Degree == "" {
		return fmt.Errorf("school_name, major, and degree are required")
	}

	startDate, err := parseOptionalDate(req.StartDate)

	if err != nil {
		return fmt.Errorf("invalid start_date: %w", err)
	}

	endDate, err := parseOptionalDate(req.EndDate)

	if err != nil {
		return fmt.Errorf("invalid end_date: %w", err)
	}

	db, err := GetDb()

	if err != nil {
		return err
	}

	result, err := db.ExecContext(ctx, `UPDATE profiles_education
		SET school_name = $1, major = $2, degree = $3, gpa = $4, start_date = $5, end_date = $6
		WHERE id = $7 AND profile_id = $8`,
		req.SchoolName, req.Major, req.Degree, req.GPA, startDate, endDate, educationID, profileID)

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

func DeleteEducation(ctx context.Context, profileID, educationID int64) error {
	db, err := GetDb()

	if err != nil {
		return err
	}

	result, err := db.ExecContext(ctx, `DELETE FROM profiles_education WHERE id = $1 AND profile_id = $2`, educationID, profileID)

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

func listSkills(ctx context.Context, db *sql.DB, profileID int64) ([]ProfileSkill, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, skill FROM profiles_skills WHERE profile_id = $1 ORDER BY skill`, profileID)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var skills []ProfileSkill

	for rows.Next() {
		var skill ProfileSkill

		if err := rows.Scan(&skill.ID, &skill.Skill); err != nil {
			return nil, err
		}

		skills = append(skills, skill)
	}

	return skills, rows.Err()
}

func AddSkill(ctx context.Context, profileID int64, req *AddSkillRequest) (int64, error) {
	if req.Skill == "" {
		return 0, fmt.Errorf("skill is required")
	}

	db, err := GetDb()

	if err != nil {
		return 0, err
	}

	var skillID int64

	err = db.QueryRowContext(ctx, insertStatements["profiles_skills"], profileID, req.Skill).Scan(&skillID)

	if err != nil {
		return 0, err
	}

	return skillID, nil
}

func DeleteSkill(ctx context.Context, profileID, skillID int64) error {
	db, err := GetDb()

	if err != nil {
		return err
	}

	result, err := db.ExecContext(ctx, `DELETE FROM profiles_skills WHERE id = $1 AND profile_id = $2`, skillID, profileID)

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

func listWorkExperienceBullets(ctx context.Context, db *sql.DB, workExperienceID int64) ([]ProfileWorkExperienceBullet, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, bullet, position FROM profiles_work_experience_bullets
		WHERE work_experience_id = $1 ORDER BY position, id`, workExperienceID)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var bullets []ProfileWorkExperienceBullet

	for rows.Next() {
		var bullet ProfileWorkExperienceBullet

		if err := rows.Scan(&bullet.ID, &bullet.Bullet, &bullet.Position); err != nil {
			return nil, err
		}

		bullets = append(bullets, bullet)
	}

	return bullets, rows.Err()
}

func listWorkExperience(ctx context.Context, db *sql.DB, profileID int64) ([]ProfileWorkExperience, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, company, job_title, job_type, location, start_date, end_date
		FROM profiles_work_experience WHERE profile_id = $1 ORDER BY start_date DESC NULLS LAST, id`, profileID)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var entries []ProfileWorkExperience

	for rows.Next() {
		var entry ProfileWorkExperience
		var location sql.NullString
		var startDate, endDate sql.NullTime

		if err := rows.Scan(&entry.ID, &entry.Company, &entry.JobTitle, &entry.JobType, &location, &startDate, &endDate); err != nil {
			return nil, err
		}

		if location.Valid {
			entry.Location = &location.String
		}

		if startDate.Valid {
			formatted := startDate.Time.Format(dateLayout)
			entry.StartDate = &formatted
		}

		if endDate.Valid {
			formatted := endDate.Time.Format(dateLayout)
			entry.EndDate = &formatted
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range entries {
		bullets, err := listWorkExperienceBullets(ctx, db, entries[i].ID)

		if err != nil {
			return nil, err
		}

		entries[i].Bullets = bullets
	}

	return entries, nil
}

func AddWorkExperience(ctx context.Context, profileID int64, req *AddWorkExperienceRequest) (int64, error) {
	if req.Company == "" || req.JobTitle == "" {
		return 0, fmt.Errorf("company and job_title are required")
	}

	jobType, ok := ParseJobType(req.JobType)

	if !ok {
		return 0, fmt.Errorf("invalid job_type: %s", req.JobType)
	}

	startDate, err := parseOptionalDate(req.StartDate)

	if err != nil {
		return 0, fmt.Errorf("invalid start_date: %w", err)
	}

	endDate, err := parseOptionalDate(req.EndDate)

	if err != nil {
		return 0, fmt.Errorf("invalid end_date: %w", err)
	}

	db, err := GetDb()

	if err != nil {
		return 0, err
	}

	var location *string

	if req.Location != "" {
		location = &req.Location
	}

	var workExperienceID int64

	err = db.QueryRowContext(ctx, insertStatements["profiles_work_experience"],
		profileID, req.Company, req.JobTitle, int(jobType), location, startDate, endDate).Scan(&workExperienceID)

	if err != nil {
		return 0, err
	}

	return workExperienceID, nil
}

func UpdateWorkExperience(ctx context.Context, profileID, workExperienceID int64, req *AddWorkExperienceRequest) error {
	if req.Company == "" || req.JobTitle == "" {
		return fmt.Errorf("company and job_title are required")
	}

	jobType, ok := ParseJobType(req.JobType)

	if !ok {
		return fmt.Errorf("invalid job_type: %s", req.JobType)
	}

	startDate, err := parseOptionalDate(req.StartDate)

	if err != nil {
		return fmt.Errorf("invalid start_date: %w", err)
	}

	endDate, err := parseOptionalDate(req.EndDate)

	if err != nil {
		return fmt.Errorf("invalid end_date: %w", err)
	}

	db, err := GetDb()

	if err != nil {
		return err
	}

	var location *string

	if req.Location != "" {
		location = &req.Location
	}

	result, err := db.ExecContext(ctx, `UPDATE profiles_work_experience
		SET company = $1, job_title = $2, job_type = $3, location = $4, start_date = $5, end_date = $6
		WHERE id = $7 AND profile_id = $8`,
		req.Company, req.JobTitle, int(jobType), location, startDate, endDate, workExperienceID, profileID)

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

func DeleteWorkExperience(ctx context.Context, profileID, workExperienceID int64) error {
	db, err := GetDb()

	if err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `DELETE FROM profiles_work_experience_bullets WHERE work_experience_id = $1`, workExperienceID); err != nil {
		return err
	}

	result, err := db.ExecContext(ctx, `DELETE FROM profiles_work_experience WHERE id = $1 AND profile_id = $2`, workExperienceID, profileID)

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

func AddWorkExperienceBullet(ctx context.Context, profileID, workExperienceID int64, req *AddWorkExperienceBulletRequest) (int64, error) {
	if req.Bullet == "" {
		return 0, fmt.Errorf("bullet is required")
	}

	db, err := GetDb()

	if err != nil {
		return 0, err
	}

	var owned bool

	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM profiles_work_experience WHERE id = $1 AND profile_id = $2)`,
		workExperienceID, profileID).Scan(&owned); err != nil {
		return 0, err
	}

	if !owned {
		return 0, sql.ErrNoRows
	}

	var bulletID int64

	err = db.QueryRowContext(ctx, insertStatements["profiles_work_experience_bullets"],
		workExperienceID, req.Bullet, req.Position).Scan(&bulletID)

	if err != nil {
		return 0, err
	}

	return bulletID, nil
}

func DeleteWorkExperienceBullet(ctx context.Context, profileID, workExperienceID, bulletID int64) error {
	db, err := GetDb()

	if err != nil {
		return err
	}

	result, err := db.ExecContext(ctx, `DELETE FROM profiles_work_experience_bullets
		WHERE id = $1 AND work_experience_id = $2
		AND work_experience_id IN (SELECT id FROM profiles_work_experience WHERE profile_id = $3)`,
		bulletID, workExperienceID, profileID)

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

func parseOptionalDate(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}

	parsed, err := time.Parse(dateLayout, raw)

	if err != nil {
		return nil, err
	}

	return &parsed, nil
}
