package database

import (
	"context"
)

func MarkJobApplied(ctx context.Context, profileID, jobID int64) error {
	var id int64

	return db().QueryRowContext(ctx, `INSERT INTO profile_job_applications (profile_id, job_id) VALUES ($1, $2)
		ON CONFLICT (profile_id, job_id) DO UPDATE SET applied_at = NOW()
		RETURNING id`, profileID, jobID).Scan(&id)
}

func UnmarkJobApplied(ctx context.Context, profileID, jobID int64) error {
	_, err := db().ExecContext(ctx, `DELETE FROM profile_job_applications WHERE profile_id = $1 AND job_id = $2`, profileID, jobID)

	return err
}
