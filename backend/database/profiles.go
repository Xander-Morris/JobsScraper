package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	emailverifier "github.com/AfterShip/email-verifier"
	"golang.org/x/crypto/bcrypt"
)

const dateLayout = "2006-01-02"

type Profile struct {
	ID             int64                   `json:"id"`
	Email          string                  `json:"email"`
	Name           string                  `json:"name"`
	Address        string                  `json:"address"`
	LinkedIn       string                  `json:"linked_in"`
	GitHub         string                  `json:"github"`
	Portfolio      string                  `json:"portfolio"`
	Education      []ProfileEducation      `json:"education"`
	Skills         []ProfileSkill          `json:"skills"`
	WorkExperience []ProfileWorkExperience `json:"work_experience"`
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
	Name      string `json:"name"`
	Address   string `json:"address"`
	LinkedIn  string `json:"linked_in"`
	GitHub    string `json:"github"`
	Portfolio string `json:"portfolio"`
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
	// Basic validation of email and password first
	if len(req.Email) == 0 || len(req.Password) == 0 {
		return 0, fmt.Errorf("Email and password cannot be empty!")
	}

	result, err := verifier.Verify(req.Email)
	if err != nil {
		return 0, err
	}
	if !result.Syntax.Valid || !result.HasMxRecords || result.Disposable {
		return 0, fmt.Errorf("Email is invalid or undeliverable")
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

	insertStatements, err := prepareInsertStatements(tx)

	if err != nil {
		return 0, err
	}

	hashedPassword, err := HashPassword(req.Password)

	if err != nil {
		return 0, err
	}

	var profileID int64

	if err := insertStatements["profiles"].QueryRow(req.Email, hashedPassword).Scan(&profileID); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return profileID, nil
}

func GetProfile(ctx context.Context, id int64) (*Profile, error) {
	db, err := GetDb()

	if err != nil {
		return nil, err
	}

	profile := &Profile{ID: id}

	row := db.QueryRowContext(ctx, `SELECT email, COALESCE(name, ''), COALESCE(address, ''),
		COALESCE(linked_in, ''), COALESCE(github, ''), COALESCE(portfolio, '') FROM profiles WHERE id = $1`, id)

	if err := row.Scan(&profile.Email, &profile.Name, &profile.Address, &profile.LinkedIn, &profile.GitHub, &profile.Portfolio); err != nil {
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

	return profile, nil
}

func UpdateProfile(ctx context.Context, id int64, req *UpdateProfileRequest) error {
	db, err := GetDb()

	if err != nil {
		return err
	}

	result, err := db.ExecContext(ctx, `UPDATE profiles SET name = $1, address = $2, linked_in = $3, github = $4, portfolio = $5 WHERE id = $6`,
		req.Name, req.Address, req.LinkedIn, req.GitHub, req.Portfolio, id)

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

	err = db.QueryRowContext(ctx, tables["profiles_education"].InsertStatement,
		profileID, req.SchoolName, req.Major, req.Degree, req.GPA, startDate, endDate).Scan(&educationID)

	if err != nil {
		return 0, err
	}

	return educationID, nil
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

	err = db.QueryRowContext(ctx, tables["profiles_skills"].InsertStatement, profileID, req.Skill).Scan(&skillID)

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

	err = db.QueryRowContext(ctx, tables["profiles_work_experience"].InsertStatement,
		profileID, req.Company, req.JobTitle, int(jobType), location, startDate, endDate).Scan(&workExperienceID)

	if err != nil {
		return 0, err
	}

	return workExperienceID, nil
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

	err = db.QueryRowContext(ctx, tables["profiles_work_experience_bullets"].InsertStatement,
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
