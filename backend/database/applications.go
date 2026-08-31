package database

import (
	"context"
	"fmt"
	"strings"
)

func MarkJobApplied(ctx context.Context, profileID, jobID int64) error {
	db, err := GetDb()

	if err != nil {
		return err
	}

	var id int64

	return db.QueryRowContext(ctx, insertStatements["profile_job_applications"], profileID, jobID).Scan(&id)
}

func UnmarkJobApplied(ctx context.Context, profileID, jobID int64) error {
	db, err := GetDb()

	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, `DELETE FROM profile_job_applications WHERE profile_id = $1 AND job_id = $2`, profileID, jobID)

	return err
}

func IsJobApplied(ctx context.Context, profileID, jobID int64) (bool, error) {
	db, err := GetDb()

	if err != nil {
		return false, err
	}

	var applied bool
	err = db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM profile_job_applications WHERE profile_id = $1 AND job_id = $2)`,
		profileID, jobID).Scan(&applied)

	return applied, err
}

// AppliedJobIDs returns the subset of jobIDs the profile has marked applied.
func AppliedJobIDs(ctx context.Context, profileID int64, jobIDs []int64) (map[int64]bool, error) {
	result := make(map[int64]bool, len(jobIDs))

	if len(jobIDs) == 0 {
		return result, nil
	}

	db, err := GetDb()

	if err != nil {
		return nil, err
	}

	placeholders := make([]string, len(jobIDs))
	args := make([]any, len(jobIDs)+1)
	args[0] = profileID

	for i, id := range jobIDs {
		args[i+1] = id
		placeholders[i] = fmt.Sprintf("$%d", i+2)
	}

	query := fmt.Sprintf(`SELECT job_id FROM profile_job_applications WHERE profile_id = $1 AND job_id IN (%s)`,
		strings.Join(placeholders, ","))

	rows, err := db.QueryContext(ctx, query, args...)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var jobID int64

		if err := rows.Scan(&jobID); err != nil {
			return nil, err
		}

		result[jobID] = true
	}

	return result, rows.Err()
}
