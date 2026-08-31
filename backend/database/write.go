package database

import (
	"database/sql"
	"fmt"
	"main/jobs"
)

var createdTables bool

func CreateTables() error {
	if createdTables {
		return nil
	}

	db, err := GetDb()

	if err != nil {
		return err
	}

	if err := runMigrations(db); err != nil {
		return err
	}

	createdTables = true

	return nil
}

func writeJobs(tx *sql.Tx, jobs []jobs.Job) error {
	for _, job := range jobs {
		var jobID int64

		if err := tx.QueryRow(insertStatements["jobs"],
			job.Title, job.Company, job.Location, job.WorkplaceType, job.SalaryMin, job.SalaryMax, job.PostedAt.UTC(), job.URL, job.Description,
		).Scan(&jobID); err != nil {
			return err
		}

		if _, err := tx.Exec("DELETE FROM job_tags WHERE job_id = $1", jobID); err != nil {
			return err
		}

		for _, tag := range job.Tags {
			var tagID int64

			if err := tx.QueryRow(insertStatements["tags"], tag).Scan(&tagID); err != nil {
				return err
			}

			if _, err := tx.Exec(insertStatements["job_tags"], jobID, tagID); err != nil {
				return err
			}
		}
	}

	return nil
}

func WriteJobsToDatabase(jobs []jobs.Job) error {
	db, err := GetDb()

	if err != nil {
		return err
	}

	if err := CreateTables(); err != nil {
		return err
	}

	tx, err := db.Begin()

	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	defer tx.Rollback()

	if err := writeJobs(tx, jobs); err != nil {
		return fmt.Errorf("failed to write jobs: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
