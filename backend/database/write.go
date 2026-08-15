package database

import (
	"database/sql"
	"fmt"
	"main/jobs"
	"strings"
	"time"
)

func createTables(db *sql.DB) error {
	for tableName, tableInfo := range tables {
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

		for _, indexCmd := range tableInfo.Indexes {
			if _, err := db.Exec(indexCmd); err != nil {
				return fmt.Errorf("create index %s: %w", indexCmd, err)
			}
		}
	}

	return nil
}

func prepareInsertStatements(tx *sql.Tx) (map[string]*sql.Stmt, error) {
	tableNameToInsertStmt := make(map[string]*sql.Stmt)

	for tableName, tableInfo := range tables {
		stmt, err := tx.Prepare(tableInfo.InsertStatement)

		if err != nil {
			return nil, fmt.Errorf("prepare insert for %s: %w", tableName, err)
		}

		tableNameToInsertStmt[tableName] = stmt
	}

	return tableNameToInsertStmt, nil
}

func writeJobs(jobs []jobs.Job, tableNameToInsertStmt map[string]*sql.Stmt, deleteJobTagsStmt *sql.Stmt) error {
	for _, job := range jobs {
		var jobID int64

		postedAt := job.PostedAt.UTC().Format(time.RFC3339)

		if err := tableNameToInsertStmt["jobs"].QueryRow(job.Title, job.Company, job.Location, job.WorkplaceType, job.SalaryMin, job.SalaryMax, postedAt, job.URL, job.Description).Scan(&jobID); err != nil {
			return err
		}

		if _, err := deleteJobTagsStmt.Exec(jobID); err != nil {
			return err
		}

		for _, tag := range job.Tags {
			var tagID int64

			if err := tableNameToInsertStmt["tags"].QueryRow(tag).Scan(&tagID); err != nil {
				return err
			}

			if _, err := tableNameToInsertStmt["job_tags"].Exec(jobID, tagID); err != nil {
				return err
			}
		}
	}

	return nil
}

func WriteToDatabase(jobs []jobs.Job) error {
	db, err := GetDb()

	if err != nil {
		return err
	}

	if err := createTables(db); err != nil {
		return err
	}

	tx, err := db.Begin()

	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	defer tx.Rollback()

	tableNameToInsertStmt, err := prepareInsertStatements(tx)

	if err != nil {
		return err
	}

	for _, stmt := range tableNameToInsertStmt {
		defer stmt.Close()
	}

	deleteJobTagsStmt, err := tx.Prepare("DELETE FROM job_tags WHERE job_id = $1")

	if err != nil {
		return fmt.Errorf("prepare delete job_tags: %w", err)
	}

	defer deleteJobTagsStmt.Close()

	if err := writeJobs(jobs, tableNameToInsertStmt, deleteJobTagsStmt); err != nil {
		return fmt.Errorf("failed to write jobs: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
