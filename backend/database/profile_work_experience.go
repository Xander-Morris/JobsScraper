package database

import (
	"context"
	"database/sql"
	"fmt"
)

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
