package database

import (
	"context"
	"fmt"
	"time"
)

// JobMaxAge is how long a posting stays listed before it's deleted.
const JobMaxAge = 60 * 24 * time.Hour

// jobAgeColumn falls back to first_seen_at for sources that don't give a post date.
const jobAgeColumn = "COALESCE(j.posted_at, j.first_seen_at)"

// DeleteExpiredJobs deletes jobs older than JobMaxAge, except ones a user marked applied or tailored a resume for,
// then drops tags no job uses anymore. Returns how many jobs were deleted.
func DeleteExpiredJobs(ctx context.Context) (int64, error) {
	db, err := GetDb()
	if err != nil {
		return 0, err
	}

	result, err := db.ExecContext(ctx, `DELETE FROM jobs j
		WHERE `+jobAgeColumn+` < $1
			AND NOT EXISTS (SELECT 1 FROM profile_job_applications a WHERE a.job_id = j.id)
			AND NOT EXISTS (SELECT 1 FROM profile_tailored_resumes r WHERE r.job_id = j.id)`,
		time.Now().Add(-JobMaxAge))
	if err != nil {
		return 0, fmt.Errorf("delete expired jobs: %w", err)
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count deleted jobs: %w", err)
	}

	if _, err := db.ExecContext(ctx, `DELETE FROM tags t
		WHERE NOT EXISTS (SELECT 1 FROM job_tags jt WHERE jt.tag_id = t.id)`); err != nil {
		return deleted, fmt.Errorf("delete unused tags: %w", err)
	}

	return deleted, nil
}
