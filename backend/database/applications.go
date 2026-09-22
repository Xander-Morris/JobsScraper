package database

import (
	"context"
)

func MarkJobApplied(ctx context.Context, profileID, jobID int64) error {
	var id int64

	return db().QueryRowContext(ctx, insertStatements["profile_job_applications"], profileID, jobID).Scan(&id)
}

func UnmarkJobApplied(ctx context.Context, profileID, jobID int64) error {
	_, err := db().ExecContext(ctx, `DELETE FROM profile_job_applications WHERE profile_id = $1 AND job_id = $2`, profileID, jobID)

	return err
}
