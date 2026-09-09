package database

import (
	"context"
	"database/sql"
	"fmt"
)

type ProfileEducation struct {
	ID         int64    `json:"id"`
	SchoolName string   `json:"school_name"`
	Major      string   `json:"major"`
	Degree     string   `json:"degree"`
	GPA        *float64 `json:"gpa"`
	StartDate  *string  `json:"start_date"`
	EndDate    *string  `json:"end_date"`
}

type AddEducationRequest struct {
	SchoolName string   `json:"school_name"`
	Major      string   `json:"major"`
	Degree     string   `json:"degree"`
	GPA        *float64 `json:"gpa"`
	StartDate  string   `json:"start_date"`
	EndDate    string   `json:"end_date"`
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
