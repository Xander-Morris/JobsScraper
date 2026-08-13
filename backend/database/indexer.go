package database

import (
	"context"
	"database/sql"
	"fmt"
	"main/jobs"
	"strings"
	"time"
)

type SortOrder string

const (
	SortRelevance SortOrder = "relevance"
	SortDate      SortOrder = "date"
)

type JobSearchParams struct {
	SearchQuery   string
	Tags          []string
	WorkplaceType jobs.WorkplaceType
	MinSalary     int
	MaxSalary     int
	Sort          SortOrder
	Limit         int
	Offset        int
}

const (
	DefaultSearchLimit = 20
	MaxSearchLimit     = 50
)

type SearchResult struct {
	Jobs  []jobs.Job
	Total int
}

func SearchForJobs(ctx context.Context, params *JobSearchParams) (*SearchResult, error) {
	db, err := GetDb() 

	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	from, whereArgs := buildJobSearchFromWhere(params)

	total, err := countJobSearchResults(ctx, db, from, whereArgs)

	if err != nil {
		return nil, fmt.Errorf("count jobs: %w", err)
	}

	query, args := buildJobSearchSelect(params, from, whereArgs)
	rows, err := db.QueryContext(ctx, query, args...)

	if err != nil {
		return nil, fmt.Errorf("search jobs: %w", err)
	}

	defer rows.Close()

	var results []jobs.Job
	scannedRows := make(map[int64]jobs.Job)
	var jobOrder []int64

	for rows.Next() {
		job, jobID, err := scanJobRow(rows)

		if err != nil {
			return nil, fmt.Errorf("scan job row: %w", err)
		}

		scannedRows[jobID] = job
		jobOrder = append(jobOrder, jobID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate job rows: %w", err)
	}

	jobIDs := make([]int64, 0, len(scannedRows))
	for jobID := range scannedRows {
		jobIDs = append(jobIDs, jobID)
	}

	tagMap, err := fetchTagsForJobs(ctx, db, jobIDs)
	if err != nil {
		return nil, fmt.Errorf("fetch tags for jobs: %w", err)
	}

	for _, jobID := range jobOrder {
		job := scannedRows[jobID]

		if tags, ok := tagMap[jobID]; ok {
			job.Tags = tags
		}

		results = append(results, job)
	}

	return &SearchResult{Jobs: results, Total: total}, nil
}

func GetJobByID(ctx context.Context, id int64) (*jobs.Job, error) {
	db, err := GetDb()

	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	const query = `SELECT j.id, j.title, j.company, COALESCE(j.location, ''), j.workplace_type,
		j.salary_min, j.salary_max, j.posted_at, j.url, COALESCE(j.description, '')
		FROM jobs j WHERE j.id = ?`

	job, jobID, err := scanJobRow(db.QueryRowContext(ctx, query, id))

	if err != nil {
		return nil, err
	}

	tagMap, err := fetchTagsForJobs(ctx, db, []int64{jobID})

	if err != nil {
		return nil, fmt.Errorf("fetch tags for job: %w", err)
	}

	job.Tags = tagMap[jobID]

	return &job, nil
}

func countJobSearchResults(ctx context.Context, db *sql.DB, from string, args []any) (int, error) {
	var total int

	query := "SELECT COUNT(*) " + from
	err := db.QueryRowContext(ctx, query, args...).Scan(&total)

	return total, err
}

func buildJobSearchFromWhere(params *JobSearchParams) (string, []any) {
	var from string
	var conditions []string
	var args []any

	if params.SearchQuery != "" {
		from = "FROM jobs_fts JOIN jobs j ON j.id = jobs_fts.rowid"
		conditions = append(conditions, "jobs_fts MATCH ?")
		args = append(args, sanitizeFTSQuery(params.SearchQuery))
	} else {
		from = "FROM jobs j"
	}

	if params.WorkplaceType != jobs.Unknown {
		conditions = append(conditions, "j.workplace_type = ?")
		args = append(args, params.WorkplaceType)
	}

	if params.MinSalary > 0 {
		conditions = append(conditions, "j.salary_max >= ?")
		args = append(args, params.MinSalary)
	}

	if params.MaxSalary > 0 {
		conditions = append(conditions, "j.salary_min <= ?")
		args = append(args, params.MaxSalary)
	}

	if len(params.Tags) > 0 {
		placeholders := make([]string, len(params.Tags))

		for i, tag := range params.Tags {
			placeholders[i] = "?"
			args = append(args, tag)
		}

		conditions = append(conditions, fmt.Sprintf(
			`j.id IN (SELECT jt.job_id FROM job_tags jt JOIN tags t ON t.id = jt.tag_id
				WHERE t.tag IN (%s) GROUP BY jt.job_id HAVING COUNT(DISTINCT t.tag) = %d)`,
			strings.Join(placeholders, ", "), len(params.Tags),
		))
	}

	if len(conditions) > 0 {
		from += " WHERE " + strings.Join(conditions, " AND ")
	}

	return from, args
}

func buildJobSearchSelect(params *JobSearchParams, from string, whereArgs []any) (string, []any) {
	const jobColumns = `j.id, j.title, j.company, COALESCE(j.location, ''), j.workplace_type,
		j.salary_min, j.salary_max, j.posted_at, j.url, COALESCE(j.description, '')`

	query := fmt.Sprintf("SELECT %s %s", jobColumns, from)

	if params.SearchQuery != "" && params.Sort != SortDate {
		query += " ORDER BY rank"
	} else {
		query += " ORDER BY j.posted_at DESC"
	}

	limit := params.Limit
	if limit <= 0 || limit > MaxSearchLimit {
		limit = DefaultSearchLimit
	}

	query += " LIMIT ? OFFSET ?"

	args := make([]any, len(whereArgs), len(whereArgs)+2)
	copy(args, whereArgs)
	args = append(args, limit, max(params.Offset, 0))

	return query, args
}

func sanitizeFTSQuery(query string) string {
	fields := strings.Fields(query)
	quoted := make([]string, len(fields))

	for i, field := range fields {
		quoted[i] = `"` + strings.ReplaceAll(field, `"`, `""`) + `"`
	}

	return strings.Join(quoted, " ")
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJobRow(row rowScanner) (jobs.Job, int64, error) {
	var job jobs.Job
	var jobID int64
	var salaryMin, salaryMax sql.NullInt64
	var postedAtRaw any

	err := row.Scan(&jobID, &job.Title, &job.Company, &job.Location, &job.WorkplaceType,
		&salaryMin, &salaryMax, &postedAtRaw, &job.URL, &job.Description)

	if err != nil {
		return jobs.Job{}, 0, err
	}

	job.ID = jobID

	if salaryMin.Valid {
		min := int(salaryMin.Int64)
		job.SalaryMin = &min
	}

	if salaryMax.Valid {
		max := int(salaryMax.Int64)
		job.SalaryMax = &max
	}

	switch v := postedAtRaw.(type) {
	case time.Time:
		job.PostedAt = v
	case string:
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			job.PostedAt = t
		}
	case []byte:
		if t, err := time.Parse(time.RFC3339, string(v)); err == nil {
			job.PostedAt = t
		}
	}

	return job, jobID, nil
}

func FetchAllUniqueTags(ctx context.Context) ([]string, error) {
	db, err := GetDb()
	var res []string 

	if err != nil {
		return res, err
	}

	query := "SELECT DISTINCT tag FROM tags"
	rows, err := db.QueryContext(ctx, query)

	if err != nil {
		return nil, err	
	}

	for rows.Next() {
		var tag string
		
		if err := rows.Scan(&tag); err != nil {
			return res, err
		}

		if rows.Err() != nil {
			return res, err
		}

		res = append(res, tag)
	}

	return res, nil 
}

func fetchTagsForJobs(ctx context.Context, db *sql.DB, jobIDs []int64) (map[int64][]string, error) {
	if len(jobIDs) == 0 {
		return make(map[int64][]string), nil
	}

	placeholders := strings.Repeat("?,", len(jobIDs))
	placeholders = placeholders[:len(placeholders)-1] 
	args := make([]any, len(jobIDs))

	for i, v := range jobIDs {
		args[i] = v
	}

	query := fmt.Sprintf(
		"SELECT jt.job_id, t.tag FROM tags t JOIN job_tags jt ON jt.tag_id = t.id WHERE jt.job_id IN (%s)",
		placeholders,
	)
	rows, err := db.QueryContext(ctx, query, args...)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	tagsMappings := make(map[int64][]string)

	for rows.Next() {
		var jobID int64
		var tag string
		
		if err := rows.Scan(&jobID, &tag); err != nil {
			return nil, err
		}

		tagsMappings[jobID] = append(tagsMappings[jobID], tag)
	}

	return tagsMappings, rows.Err()
}