package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	pg "github.com/go-jet/jet/v2/postgres"

	"main/internal/gen/table"
)

// JobMaxAge is how long a posting stays listed before it's deleted.
const JobMaxAge = 30 * 24 * time.Hour

// MaxJobs caps how many unprotected jobs are stored; the oldest past it are deleted.
const MaxJobs = 5000

// jobAge falls back to first_seen_at for sources that don't give a post date.
func jobAge() pg.TimestampzExpression {
	return pg.TimestampzExp(pg.COALESCE(table.Jobs.PostedAt, table.Jobs.FirstSeenAt))
}

// unprotectedJob excludes jobs a user marked applied or tailored a resume for.
func unprotectedJob() pg.BoolExpression {
	applications, tailored := table.ProfileJobApplications, table.ProfileTailoredResumes

	return pg.NOT(pg.EXISTS(pg.SELECT(applications.ID).FROM(applications).WHERE(applications.JobID.EQ(table.Jobs.ID)))).
		AND(pg.NOT(pg.EXISTS(pg.SELECT(tailored.ID).FROM(tailored).WHERE(tailored.JobID.EQ(table.Jobs.ID)))))
}

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

	jobs, jobTags, tags := table.Jobs, table.JobTags, table.Tags

	expired, err := exec(ctx, tx, jobs.DELETE().
		WHERE(jobAge().LT(pg.TimestampzT(time.Now().Add(-JobMaxAge))).AND(unprotectedJob())))
	if err != nil {
		return 0, fmt.Errorf("delete expired jobs: %w", err)
	}

	overCap, err := exec(ctx, tx, jobs.DELETE().WHERE(jobs.ID.IN(
		pg.SELECT(jobs.ID).FROM(jobs).
			WHERE(unprotectedJob()).
			ORDER_BY(jobAge().DESC().NULLS_LAST(), jobs.ID.DESC()).
			OFFSET(MaxJobs))))
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

	if _, err := exec(ctx, tx, tags.DELETE().WHERE(pg.NOT(pg.EXISTS(
		pg.SELECT(jobTags.TagID).FROM(jobTags).WHERE(jobTags.TagID.EQ(tags.ID)))))); err != nil {
		return 0, fmt.Errorf("delete unused tags: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit delete expired jobs: %w", err)
	}

	forgetTermWeights()

	return deleted, nil
}
