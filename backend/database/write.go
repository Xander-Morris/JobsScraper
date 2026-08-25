package database

import (
	"database/sql"
	"fmt"
	"main/jobs"
	"strings"
	"time"
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

	if len(tableCreationOrder) != len(tables) {
		return fmt.Errorf("tableCreationOrder (%d entries) is out of sync with tables (%d entries)", len(tableCreationOrder), len(tables))
	}

	for _, tableName := range tableCreationOrder {
		tableInfo := tables[tableName]
		var colDefs []string

		for _, colMap := range tableInfo.Columns {
			for colName, colType := range colMap {
				colDefs = append(colDefs, fmt.Sprintf("%s %s", colName, colType))
			}
		}

		query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s);", tableName, strings.Join(colDefs, ", "))

		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("create table %s: %w", tableName, err)
		}

		for _, colMap := range tableInfo.Columns {
			for colName, colType := range colMap {
				alterQuery := fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s;", tableName, colName, colType)

				if _, err := db.Exec(alterQuery); err != nil {
					return fmt.Errorf("add column %s.%s: %w", tableName, colName, err)
				}
			}
		}

		for _, indexCmd := range tableInfo.Indexes {
			if _, err := db.Exec(indexCmd); err != nil {
				return fmt.Errorf("create index %s: %w", indexCmd, err)
			}
		}
	}

	createdTables = true 

	return nil
}

func writeJobs(tx *sql.Tx, jobs []jobs.Job) error {
	for _, job := range jobs {
		var jobID int64

		postedAt := job.PostedAt.UTC().Format(time.RFC3339)

		if err := tx.QueryRow(tables["jobs"].InsertStatement,
			job.Title, job.Company, job.Location, job.WorkplaceType, job.SalaryMin, job.SalaryMax, postedAt, job.URL, job.Description,
		).Scan(&jobID); err != nil {
			return err
		}

		if _, err := tx.Exec("DELETE FROM job_tags WHERE job_id = $1", jobID); err != nil {
			return err
		}

		for _, tag := range job.Tags {
			var tagID int64

			if err := tx.QueryRow(tables["tags"].InsertStatement, tag).Scan(&tagID); err != nil {
				return err
			}

			if _, err := tx.Exec(tables["job_tags"].InsertStatement, jobID, tagID); err != nil {
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
