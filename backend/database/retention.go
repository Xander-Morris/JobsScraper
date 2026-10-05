package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// JobMaxAge is how long a posting stays listed before it's deleted.
const JobMaxAge = 30 * 24 * time.Hour

// MaxJobs caps how many unprotected jobs are stored; the oldest past it are deleted.
const MaxJobs = 5000

// jobAgeColumn falls back to first_seen_at for sources that don't give a post date.
const jobAgeColumn = "COALESCE(j.posted_at, j.first_seen_at)"

// unprotectedJob excludes jobs a user marked applied or tailored a resume for.
const unprotectedJob = `NOT EXISTS (SELECT 1 FROM profile_job_applications a WHERE a.job_id = j.id)
	AND NOT EXISTS (SELECT 1 FROM profile_tailored_resumes r WHERE r.job_id = j.id)`

// DeleteExpiredJobs deletes unprotected jobs older than JobMaxAge or beyond the newest MaxJobs,
// then drops tags no job uses anymore. Returns how many jobs were deleted.
func DeleteExpiredJobs(ctx context.Context) (int64, error) {
	tx, err := db().BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin delete expired jobs: %w", err)
	}
	defer tx.Rollback()

	// Supabase flips the database to read-only when it's full; deleting is how it recovers.
	if _, err := tx.ExecContext(ctx, "SET TRANSACTION READ WRITE"); err != nil {
		return 0, fmt.Errorf("set read write: %w", err)
	}

	expired, err := tx.ExecContext(ctx, `DELETE FROM jobs j WHERE `+jobAgeColumn+` < $1 AND `+unprotectedJob,
		time.Now().Add(-JobMaxAge))
	if err != nil {
		return 0, fmt.Errorf("delete expired jobs: %w", err)
	}

	overCap, err := tx.ExecContext(ctx, `DELETE FROM jobs WHERE id IN (
		SELECT j.id FROM jobs j WHERE `+unprotectedJob+`
		ORDER BY `+jobAgeColumn+` DESC NULLS LAST, j.id DESC
		OFFSET $1)`, MaxJobs)
	if err != nil {
		return 0, fmt.Errorf("delete jobs over cap: %w", err)
	}

	var deleted int64
	for _, result := range []sql.Result{expired, overCap} {
		n, err := result.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("count deleted jobs: %w", err)
		}
		deleted += n
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM tags t
		WHERE NOT EXISTS (SELECT 1 FROM job_tags jt WHERE jt.tag_id = t.id)`); err != nil {
		return 0, fmt.Errorf("delete unused tags: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit delete expired jobs: %w", err)
	}

	return deleted, nil
}
