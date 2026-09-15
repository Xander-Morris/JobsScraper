package database

import (
	"context"
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
