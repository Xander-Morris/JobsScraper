package database

import (
	"context"
	"database/sql"
	"fmt"
	"main/jobs"
	"strings"
	"time"

	"github.com/pgvector/pgvector-go"
)

type SortOrder string

const (
	SortRelevance SortOrder = "relevance"
	SortDate      SortOrder = "date"
)

type JobSearchParams struct {
	SearchQuery string
	// ResumeQuery is a fallback keyword score, used only when ResumeEmbedding is nil.
	ResumeQuery string
	// ResumeEmbedding, when set, scores jobs by cosine similarity to the active
	// resume instead of ResumeQuery's keyword overlap.
	ResumeEmbedding *pgvector.Vector
	ProfileID       int64
	PostedAfter     *time.Time
	Tags            []string
	WorkplaceType   jobs.WorkplaceType
	MinSalary       int
	MaxSalary       int
	Sort            SortOrder
	Limit           int
	Offset          int
}

type JobDetailParams struct {
	ResumeQuery     string
	ResumeEmbedding *pgvector.Vector
	ProfileID       int64
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

	var appliedMap map[int64]bool
	if params.ProfileID != 0 {
		appliedMap, err = AppliedJobIDs(ctx, params.ProfileID, jobIDs)
		if err != nil {
			return nil, fmt.Errorf("fetch applied jobs: %w", err)
		}
	}

	for _, jobID := range jobOrder {
		job := scannedRows[jobID]

		if tags, ok := tagMap[jobID]; ok {
			job.Tags = tags
		}

		if appliedMap[jobID] {
			job.Applied = true
		}

		results = append(results, job)
	}

	return &SearchResult{Jobs: results, Total: total}, nil
}

func GetJobByID(ctx context.Context, id int64, params JobDetailParams) (*jobs.Job, error) {
	db, err := GetDb()

	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	matchScoreColumn := "NULL::real"
	args := []any{id}

	switch {
	case params.ResumeEmbedding != nil:
		args = append(args, *params.ResumeEmbedding)
		matchScoreColumn = fmt.Sprintf("CASE WHEN j.embedding IS NOT NULL THEN 1 - (j.embedding <=> $%d) ELSE NULL END", len(args))
	case params.ResumeQuery != "":
		args = append(args, params.ResumeQuery)
		matchScoreColumn = fmt.Sprintf("ts_rank(j.search_vector, websearch_to_tsquery('english', $%d), 1)", len(args))
	}

	query := fmt.Sprintf(`SELECT j.id, j.title, j.company, COALESCE(j.location, ''), j.workplace_type,
		j.salary_min, j.salary_max, j.posted_at, j.url, COALESCE(j.description, ''), %s AS match_score
		FROM jobs j WHERE j.id = $1`, matchScoreColumn)

	job, jobID, err := scanJobRow(db.QueryRowContext(ctx, query, args...))

	if err != nil {
		return nil, err
	}

	tagMap, err := fetchTagsForJobs(ctx, db, []int64{jobID})

	if err != nil {
		return nil, fmt.Errorf("fetch tags for job: %w", err)
	}

	job.Tags = tagMap[jobID]

	if params.ProfileID != 0 {
		applied, err := IsJobApplied(ctx, params.ProfileID, jobID)

		if err != nil {
			return nil, fmt.Errorf("check job applied: %w", err)
		}

		job.Applied = applied
	}

	return &job, nil
}

func countJobSearchResults(ctx context.Context, db *sql.DB, from string, args []any) (int, error) {
	var total int

	query := "SELECT COUNT(*) " + from
	err := db.QueryRowContext(ctx, query, args...).Scan(&total)

	return total, err
}

func buildJobSearchFromWhere(params *JobSearchParams) (string, []any) {
	from := "FROM jobs j"
	var conditions []string
	var args []any

	if params.SearchQuery != "" {
		args = append(args, params.SearchQuery)
		conditions = append(conditions, fmt.Sprintf("j.search_vector @@ plainto_tsquery('english', $%d)", len(args)))
	}

	if params.WorkplaceType != jobs.Unknown {
		args = append(args, params.WorkplaceType)
		conditions = append(conditions, fmt.Sprintf("j.workplace_type = $%d", len(args)))
	}

	if params.MinSalary > 0 {
		args = append(args, params.MinSalary)
		conditions = append(conditions, fmt.Sprintf("j.salary_max >= $%d", len(args)))
	}

	if params.MaxSalary > 0 {
		args = append(args, params.MaxSalary)
		conditions = append(conditions, fmt.Sprintf("j.salary_min <= $%d", len(args)))
	}

	if params.PostedAfter != nil {
		args = append(args, *params.PostedAfter)
		conditions = append(conditions, fmt.Sprintf("j.posted_at > $%d", len(args)))
	}

	if len(params.Tags) > 0 {
		placeholders := make([]string, len(params.Tags))

		for i, tag := range params.Tags {
			args = append(args, tag)
			placeholders[i] = fmt.Sprintf("$%d", len(args))
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

	args := make([]any, len(whereArgs), len(whereArgs)+5)
	copy(args, whereArgs)

	rankable := params.Sort != SortDate
	hasSearchQuery := params.SearchQuery != ""

	// match_score is independent of sort order, shown as a fit signal even when
	// sorting by date. resumeScoreExpr is reused below in ORDER BY when blending
	// with a typed search query.
	matchScoreColumn := "NULL::real"
	var resumeScoreExpr string

	switch {
	case params.ResumeEmbedding != nil:
		args = append(args, *params.ResumeEmbedding)
		resumeScoreExpr = fmt.Sprintf("CASE WHEN j.embedding IS NOT NULL THEN 1 - (j.embedding <=> $%d) ELSE NULL END", len(args))
	case params.ResumeQuery != "":
		args = append(args, params.ResumeQuery)
		resumeScoreExpr = fmt.Sprintf("ts_rank(j.search_vector, websearch_to_tsquery('english', $%d), 1)", len(args))
	}

	if resumeScoreExpr != "" {
		matchScoreColumn = resumeScoreExpr
	}

	hasResumeScore := resumeScoreExpr != ""
	query := fmt.Sprintf("SELECT %s, %s AS match_score %s", jobColumns, matchScoreColumn, from)

	switch {
	case rankable && hasSearchQuery && hasResumeScore:
		// Typed search stays the primary signal; the resume nudges ties toward jobs
		// matching the candidate's skills/experience without overriding an explicit query.
		args = append(args, params.SearchQuery)
		query += fmt.Sprintf(
			" ORDER BY (ts_rank(j.search_vector, plainto_tsquery('english', $%d)) + 0.5 * coalesce(%s, 0)) DESC",
			len(args), resumeScoreExpr)
	case rankable && hasSearchQuery:
		args = append(args, params.SearchQuery)
		query += fmt.Sprintf(" ORDER BY ts_rank(j.search_vector, plainto_tsquery('english', $%d)) DESC", len(args))
	case rankable && hasResumeScore:
		query += fmt.Sprintf(" ORDER BY coalesce(%s, 0) DESC", resumeScoreExpr)
	default:
		query += " ORDER BY j.posted_at DESC"
	}

	limit := params.Limit
	if limit <= 0 || limit > MaxSearchLimit {
		limit = DefaultSearchLimit
	}

	args = append(args, limit, max(params.Offset, 0))
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	return query, args
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJobRow(row rowScanner) (jobs.Job, int64, error) {
	var job jobs.Job
	var jobID int64
	var salaryMin, salaryMax sql.NullInt64
	var postedAtRaw any
	var matchScore sql.NullFloat64

	err := row.Scan(&jobID, &job.Title, &job.Company, &job.Location, &job.WorkplaceType,
		&salaryMin, &salaryMax, &postedAtRaw, &job.URL, &job.Description, &matchScore)

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

	if matchScore.Valid {
		score := matchScore.Float64
		job.MatchScore = &score
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

	placeholders := make([]string, len(jobIDs))
	args := make([]any, len(jobIDs))

	for i, v := range jobIDs {
		args[i] = v
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	query := fmt.Sprintf(
		"SELECT jt.job_id, t.tag FROM tags t JOIN job_tags jt ON jt.tag_id = t.id WHERE jt.job_id IN (%s)",
		strings.Join(placeholders, ","),
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