package database

import (
	"context"
	"database/sql"
	"encoding/json"
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

type UpdateWorkExperienceBulletRequest struct {
	Bullet string `json:"bullet"`
}

func listWorkExperience(ctx context.Context, profileID int64) ([]ProfileWorkExperience, error) {
	// Bullets ride along as JSON so the profile loads in one query, not one per entry.
	rows, err := db().QueryContext(ctx, `SELECT w.id, w.company, w.job_title, w.job_type, w.location, w.start_date, w.end_date,
		(SELECT json_agg(json_build_object('id', b.id, 'bullet', b.bullet, 'position', b.position) ORDER BY b.position, b.id)
			FROM profiles_work_experience_bullets b WHERE b.work_experience_id = w.id)
		FROM profiles_work_experience w WHERE w.profile_id = $1 ORDER BY w.start_date DESC NULLS LAST, w.id`, profileID)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var entries []ProfileWorkExperience

	for rows.Next() {
		var entry ProfileWorkExperience
		var location sql.NullString
		var startDate, endDate sql.NullTime
		var bullets []byte

		if err := rows.Scan(&entry.ID, &entry.Company, &entry.JobTitle, &entry.JobType, &location, &startDate, &endDate, &bullets); err != nil {
			return nil, err
		}

		// json_agg is NULL with no bullets, leaving Bullets nil as before.
		if bullets != nil {
			if err := json.Unmarshal(bullets, &entry.Bullets); err != nil {
				return nil, fmt.Errorf("decode bullets: %w", err)
			}
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

	return entries, rows.Err()
}

func AddWorkExperience(ctx context.Context, profileID int64, req *AddWorkExperienceRequest) (int64, error) {
	if req.Company == "" || req.JobTitle == "" {
		return 0, invalidInput("company and job_title are required")
	}

	jobType, ok := ParseJobType(req.JobType)

	if !ok {
		return 0, invalidInput("invalid job_type: %s", req.JobType)
	}

	startDate, err := parseOptionalDate(req.StartDate)

	if err != nil {
		return 0, invalidInput("invalid start_date: %s", err)
	}

	endDate, err := parseOptionalDate(req.EndDate)

	if err != nil {
		return 0, invalidInput("invalid end_date: %s", err)
	}

	var location *string

	if req.Location != "" {
		location = &req.Location
	}

	var workExperienceID int64

	err = db().QueryRowContext(ctx, `INSERT INTO profiles_work_experience (profile_id, company, job_title, job_type, location, start_date, end_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (profile_id, company, job_title, start_date) DO UPDATE SET
			job_type = excluded.job_type,
			location = excluded.location,
			end_date = excluded.end_date
		RETURNING id`,
		profileID, req.Company, req.JobTitle, int(jobType), location, startDate, endDate).Scan(&workExperienceID)

	if err != nil {
		return 0, err
	}

	return workExperienceID, nil
}

func UpdateWorkExperience(ctx context.Context, profileID, workExperienceID int64, req *AddWorkExperienceRequest) error {
	if req.Company == "" || req.JobTitle == "" {
		return invalidInput("company and job_title are required")
	}

	jobType, ok := ParseJobType(req.JobType)

	if !ok {
		return invalidInput("invalid job_type: %s", req.JobType)
	}

	startDate, err := parseOptionalDate(req.StartDate)

	if err != nil {
		return invalidInput("invalid start_date: %s", err)
	}

	endDate, err := parseOptionalDate(req.EndDate)

	if err != nil {
		return invalidInput("invalid end_date: %s", err)
	}

	var location *string

	if req.Location != "" {
		location = &req.Location
	}

	result, err := db().ExecContext(ctx, `UPDATE profiles_work_experience
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
	// Scoped to the owner, so a wrong profile can't wipe someone else's bullets.
	if _, err := db().ExecContext(ctx, `DELETE FROM profiles_work_experience_bullets WHERE work_experience_id IN
		(SELECT id FROM profiles_work_experience WHERE id = $1 AND profile_id = $2)`, workExperienceID, profileID); err != nil {
		return err
	}

	result, err := db().ExecContext(ctx, `DELETE FROM profiles_work_experience WHERE id = $1 AND profile_id = $2`, workExperienceID, profileID)

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
		return 0, invalidInput("bullet is required")
	}

	var owned bool

	if err := db().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM profiles_work_experience WHERE id = $1 AND profile_id = $2)`,
		workExperienceID, profileID).Scan(&owned); err != nil {
		return 0, err
	}

	if !owned {
		return 0, sql.ErrNoRows
	}

	var bulletID int64

	err := db().QueryRowContext(ctx, `INSERT INTO profiles_work_experience_bullets (work_experience_id, bullet, position)
		VALUES ($1, $2, $3)
		ON CONFLICT (work_experience_id, bullet) DO UPDATE SET
			position = excluded.position
		RETURNING id`,
		workExperienceID, req.Bullet, req.Position).Scan(&bulletID)

	if err != nil {
		return 0, err
	}

	return bulletID, nil
}

func UpdateWorkExperienceBullet(ctx context.Context, profileID, workExperienceID, bulletID int64, req *UpdateWorkExperienceBulletRequest) error {
	if req.Bullet == "" {
		return invalidInput("bullet is required")
	}

	result, err := db().ExecContext(ctx, `UPDATE profiles_work_experience_bullets SET bullet = $1
		WHERE id = $2 AND work_experience_id = $3
		AND work_experience_id IN (SELECT id FROM profiles_work_experience WHERE profile_id = $4)`,
		req.Bullet, bulletID, workExperienceID, profileID)

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

func DeleteWorkExperienceBullet(ctx context.Context, profileID, workExperienceID, bulletID int64) error {
	result, err := db().ExecContext(ctx, `DELETE FROM profiles_work_experience_bullets
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

type JobType int

const (
	JobTypeUnknown JobType = iota
	JobTypeContract
	JobTypeInternship
	JobTypePartTime
	JobTypeFullTime
)

func ParseJobType(s string) (JobType, bool) {
	switch s {
	case "", "unknown":
		return JobTypeUnknown, true
	case "contract":
		return JobTypeContract, true
	case "internship":
		return JobTypeInternship, true
	case "part_time":
		return JobTypePartTime, true
	case "full_time":
		return JobTypeFullTime, true
	default:
		return JobTypeUnknown, false
	}
}

func (j JobType) String() string {
	switch j {
	case JobTypeContract:
		return "contract"
	case JobTypeInternship:
		return "internship"
	case JobTypePartTime:
		return "part_time"
	case JobTypeFullTime:
		return "full_time"
	default:
		return "unknown"
	}
}

func (j JobType) MarshalJSON() ([]byte, error) {
	return json.Marshal(j.String())
}

func (j *JobType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	parsed, _ := ParseJobType(s)
	*j = parsed

	return nil
}
