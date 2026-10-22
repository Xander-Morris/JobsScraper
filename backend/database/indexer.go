package database

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"main/jobs"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/pgvector/pgvector-go"
)

type SortOrder string

const (
	SortRelevance SortOrder = "relevance"
	SortDate      SortOrder = "date"
)

type JobTypeFilter string

const (
	JobTypeFilterIntern   JobTypeFilter = "intern"
	JobTypeFilterPartTime JobTypeFilter = "part_time"
	JobTypeFilterFullTime JobTypeFilter = "full_time"
)

// Case-insensitive word matches against a job's title and tags. \b keeps "internal" and "international" out.
// Mirrors the \m...\M regexes migration 000006 backfilled with.
var jobTypePatterns = map[JobTypeFilter]*regexp.Regexp{
	JobTypeFilterIntern:   regexp.MustCompile(`(?i)\bintern(s|ships?)?\b`),
	JobTypeFilterPartTime: regexp.MustCompile(`(?i)\bpart[-_ ]?time\b`),
	JobTypeFilterFullTime: regexp.MustCompile(`(?i)\bfull[-_ ]?time\b`),
}

var jobTypeColumns = map[JobTypeFilter]string{
	JobTypeFilterIntern:   "j.is_intern",
	JobTypeFilterPartTime: "j.is_part_time",
	JobTypeFilterFullTime: "j.is_full_time",
}

// jobTypeFlags are stored on each job at write time, so search filters on a column instead of a regex per row.
type jobTypeFlags struct {
	intern, partTime, fullTime bool
}

func jobTypeFlagsFor(job jobs.Job) jobTypeFlags {
	matches := func(filter JobTypeFilter) bool {
		pattern := jobTypePatterns[filter]
		return pattern.MatchString(job.Title) || slices.ContainsFunc(job.Tags, pattern.MatchString)
	}

	return jobTypeFlags{
		intern:   matches(JobTypeFilterIntern),
		partTime: matches(JobTypeFilterPartTime),
		fullTime: matches(JobTypeFilterFullTime),
	}
}

type JobSearchParams struct {
	SearchQuery string
	// ResumeQuery is a fallback keyword score, used only when ResumeEmbedding is nil.
	ResumeQuery string
	// ResumeEmbedding, when set, scores jobs by cosine similarity instead of
	// ResumeQuery's keyword overlap.
	ResumeEmbedding *pgvector.Vector
	ProfileID       int64
	PostedAfter     *time.Time
	// FirstSeenAfter filters on when we scraped the job, not its post date.
	FirstSeenAfter *time.Time
	Tags           []string
	WorkplaceType  jobs.WorkplaceType
	JobType        JobTypeFilter
	MinSalary      int
	MaxSalary      int
	Sort           SortOrder
	Limit          int
	Offset         int
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

// Display columns shared by search results and job detail; detail adds description.
const jobColumns = `j.id, j.title, j.company, COALESCE(j.location, ''), j.workplace_type,
	j.salary_min, j.salary_max, j.posted_at, j.url`

const jobTagsColumn = `COALESCE((SELECT json_agg(t.tag ORDER BY jt.tag_id) FROM job_tags jt
	JOIN tags t ON t.id = jt.tag_id WHERE jt.job_id = j.id), '[]')`

func SearchForJobs(ctx context.Context, params *JobSearchParams) (*SearchResult, error) {
	where, whereArgs := buildJobSearchWhere(params)
	query, args := buildJobSearchSelect(params, where, whereArgs)

	results, total, err := queryJobPage(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("search jobs: %w", err)
	}

	// Past the last page no row carries the total, so count it separately.
	if len(results) == 0 && params.Offset > 0 {
		if total, err = countJobSearchResults(ctx, where, whereArgs); err != nil {
			return nil, fmt.Errorf("count jobs: %w", err)
		}
	}

	return &SearchResult{Jobs: results, Total: total}, nil
}

func GetJobByID(ctx context.Context, id int64, params JobDetailParams) (*jobs.Job, error) {
	args := []any{id}
	resumeScore := resumeScoreExpr(&args, params.ResumeEmbedding, params.ResumeQuery)
	matchScoreColumn := cmp.Or(resumeScore, "NULL::real")
	percentColumn := "NULL::int"
	with := ""

	if resumeScore != "" {
		args = append(args, time.Now().Add(-JobMaxAge))
		with = scoredCTEs(resumeScore, len(args))
		percentColumn = "(SELECT floor(r.pct * 100)::int FROM ranked r WHERE r.id = j.id)"
	}

	appliedColumn := appliedExpr(&args, params.ProfileID)

	query := fmt.Sprintf(`%sSELECT %s, %s AS match_score, %s, %s, %s, COALESCE(j.description, '')
		FROM jobs j WHERE j.id = $1`, with, jobColumns, matchScoreColumn, percentColumn, jobTagsColumn, appliedColumn)

	job, err := scanJob(db().QueryRowContext(ctx, query, args...), true)

	if err != nil {
		return nil, err
	}

	return &job, nil
}

// queryJobPage scans a search page whose rows each end with the pre-LIMIT total.
func queryJobPage(ctx context.Context, query string, args []any) ([]jobs.Job, int, error) {
	rows, err := db().QueryContext(ctx, query, args...)

	if err != nil {
		return nil, 0, err
	}

	defer rows.Close()

	var results []jobs.Job
	var total int

	for rows.Next() {
		job, err := scanJob(rows, false, &total)

		if err != nil {
			return nil, 0, fmt.Errorf("scan job row: %w", err)
		}

		results = append(results, job)
	}

	return results, total, rows.Err()
}

func countJobSearchResults(ctx context.Context, where string, args []any) (int, error) {
	var total int

	err := db().QueryRowContext(ctx, "SELECT COUNT(*) FROM jobs j"+where, args...).Scan(&total)

	return total, err
}

// buildJobSearchWhere returns " WHERE ..." with the age cutoff always first, as $1.
func buildJobSearchWhere(params *JobSearchParams) (string, []any) {
	var conditions []string
	var args []any

	// Jobs kept past JobMaxAge only because a user acted on them stay out of search.
	args = append(args, time.Now().Add(-JobMaxAge))
	conditions = append(conditions, fmt.Sprintf("%s >= $%d", jobAgeColumn, len(args)))

	if params.SearchQuery != "" {
		args = append(args, params.SearchQuery)
		conditions = append(conditions, fmt.Sprintf("j.search_vector @@ plainto_tsquery('english', $%d)", len(args)))
	}

	if params.WorkplaceType != jobs.Unknown {
		args = append(args, params.WorkplaceType)
		conditions = append(conditions, fmt.Sprintf("j.workplace_type = $%d", len(args)))
	}

	if column, ok := jobTypeColumns[params.JobType]; ok {
		condition := column

		// Intern takes precedence, so part/full-time exclude anything that also reads as an internship.
		if params.JobType != JobTypeFilterIntern {
			condition += " AND NOT " + jobTypeColumns[JobTypeFilterIntern]
		}

		conditions = append(conditions, condition)
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

	if params.FirstSeenAfter != nil {
		args = append(args, *params.FirstSeenAfter)
		conditions = append(conditions, fmt.Sprintf("j.first_seen_at > $%d", len(args)))
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

	return " WHERE " + strings.Join(conditions, " AND "), args
}

// buildJobSearchSelect sorts and pages over (id, sort_key, score) only, then fetches display
// columns, percent, tags and applied for just the rows on the page.
func buildJobSearchSelect(params *JobSearchParams, where string, whereArgs []any) (string, []any) {
	args := slices.Clone(whereArgs)

	rankable := params.Sort != SortDate
	hasSearchQuery := params.SearchQuery != ""

	// match_score shows as a fit signal even when sorting by date.
	resumeScore := resumeScoreExpr(&args, params.ResumeEmbedding, params.ResumeQuery)
	hasResumeScore := resumeScore != ""

	with, scoredJoin, score, percentJoin, percentColumn := "", "", "NULL::real", "", "NULL::int"

	if hasResumeScore {
		// $1 is the age cutoff, so scored covers exactly the jobs search can list.
		with = scoredCTEs(resumeScore, 1)
		scoredJoin = " JOIN scored s ON s.id = j.id"
		score = "s.score"
		percentJoin = " LEFT JOIN ranked r ON r.id = page.id"
		percentColumn = "floor(r.pct * 100)::int"
	}

	var sortKey string

	switch {
	case rankable && hasSearchQuery && hasResumeScore:
		// Typed search stays primary; resume score just nudges ties toward a
		// good skill match, doesn't override an explicit query.
		args = append(args, params.SearchQuery)
		sortKey = fmt.Sprintf("ts_rank(j.search_vector, (SELECT plainto_tsquery('english', $%d))) + 0.5 * coalesce(s.score, 0)", len(args))
	case rankable && hasSearchQuery:
		args = append(args, params.SearchQuery)
		sortKey = fmt.Sprintf("ts_rank(j.search_vector, (SELECT plainto_tsquery('english', $%d)))", len(args))
	case rankable && hasResumeScore:
		// NULLS LAST below puts jobs not embedded yet after scored ones.
		sortKey = "s.score"
	default:
		sortKey = jobAgeColumn
	}

	limit := params.Limit
	if limit <= 0 || limit > MaxSearchLimit {
		limit = DefaultSearchLimit
	}

	args = append(args, limit, max(params.Offset, 0))
	page := fmt.Sprintf(`SELECT j.id, %s AS sort_key, %s AS score, count(*) OVER () AS total FROM jobs j%s%s
		ORDER BY sort_key DESC NULLS LAST, j.id DESC LIMIT $%d OFFSET $%d`,
		sortKey, score, scoredJoin, where, len(args)-1, len(args))

	appliedColumn := appliedExpr(&args, params.ProfileID)

	query := fmt.Sprintf(`%sSELECT %s, page.score AS match_score, %s, %s, %s, page.total
		FROM (%s) page JOIN jobs j ON j.id = page.id%s
		ORDER BY page.sort_key DESC NULLS LAST, page.id DESC`,
		with, jobColumns, percentColumn, jobTagsColumn, appliedColumn, page, percentJoin)

	return query, args
}

// scoredCTEs scores every listable job once (scored) and ranks those scores (ranked),
// so match_percent reads the same on any page, filter, or sort. ageArg is the cutoff's $n.
func scoredCTEs(score string, ageArg int) string {
	return fmt.Sprintf(`WITH scored AS MATERIALIZED (
			SELECT j.id, %s AS score FROM jobs j WHERE %s >= $%d),
		ranked AS (SELECT id, percent_rank() OVER (ORDER BY score) AS pct FROM scored WHERE score IS NOT NULL)
		`, score, jobAgeColumn, ageArg)
}

// resumeScoreExpr scores j against the caller's resume, appending its arg to args.
// Empty when there's no resume to score against.
func resumeScoreExpr(args *[]any, embedding *pgvector.Vector, query string) string {
	switch {
	case embedding != nil:
		*args = append(*args, *embedding)
		return fmt.Sprintf("CASE WHEN j.embedding IS NOT NULL THEN 1 - (j.embedding <=> $%d) ELSE NULL END", len(*args))
	case query != "":
		*args = append(*args, query)
		return fmt.Sprintf("ts_rank(j.search_vector, (SELECT websearch_to_tsquery('english', $%d)), 1)", len(*args))
	default:
		return ""
	}
}

// appliedExpr is whether profileID marked j applied; always false for anonymous callers.
func appliedExpr(args *[]any, profileID int64) string {
	if profileID == 0 {
		return "false"
	}

	*args = append(*args, profileID)

	return fmt.Sprintf("EXISTS (SELECT 1 FROM profile_job_applications a WHERE a.profile_id = $%d AND a.job_id = j.id)", len(*args))
}

type rowScanner interface {
	Scan(dest ...any) error
}

// scanJob reads jobColumns, match_score, match_percent, tags and applied, plus description
// when withDescription, then any extra trailing columns into extra.
func scanJob(row rowScanner, withDescription bool, extra ...any) (jobs.Job, error) {
	var job jobs.Job
	var salaryMin, salaryMax, matchPercent sql.NullInt64
	var postedAt sql.NullTime
	var matchScore sql.NullFloat64
	var tags []byte

	dest := []any{&job.ID, &job.Title, &job.Company, &job.Location, &job.WorkplaceType,
		&salaryMin, &salaryMax, &postedAt, &job.URL, &matchScore, &matchPercent, &tags, &job.Applied}

	if withDescription {
		dest = append(dest, &job.Description)
	}

	if err := row.Scan(append(dest, extra...)...); err != nil {
		return jobs.Job{}, err
	}

	if err := json.Unmarshal(tags, &job.Tags); err != nil {
		return jobs.Job{}, fmt.Errorf("decode tags: %w", err)
	}

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

	if matchPercent.Valid {
		percent := int(matchPercent.Int64)
		job.MatchPercent = &percent
	}

	if postedAt.Valid {
		job.PostedAt = postedAt.Time
	}

	return job, nil
}

func FetchAllUniqueTags(ctx context.Context) ([]string, error) {
	rows, err := db().QueryContext(ctx, "SELECT tag FROM tags ORDER BY tag")

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var res []string

	for rows.Next() {
		var tag string

		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}

		res = append(res, tag)
	}

	return res, rows.Err()
}
