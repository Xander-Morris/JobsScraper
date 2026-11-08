package database

import (
	"context"
	"fmt"
	"main/jobs"
	"regexp"
	"slices"
	"time"

	pg "github.com/go-jet/jet/v2/postgres"
	"github.com/pgvector/pgvector-go"

	"main/internal/gen/model"
	"main/internal/gen/table"
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

var jobTypeColumns = map[JobTypeFilter]pg.ColumnBool{
	JobTypeFilterIntern:   table.Jobs.IsIntern,
	JobTypeFilterPartTime: table.Jobs.IsPartTime,
	JobTypeFilterFullTime: table.Jobs.IsFullTime,
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
	// SearchEmbedding, when set, blends query-to-job cosine similarity into typed search.
	SearchEmbedding *pgvector.Vector
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

// resumeNudgeWeight scales resume fit when a typed query is the primary ranking.
const resumeNudgeWeight = 0.2

const (
	DefaultSearchLimit = 20
	MaxSearchLimit     = 50
)

type SearchResult struct {
	Jobs  []jobs.Job
	Total int
}

// jobDisplayColumns are shared by search results and job detail.
var jobDisplayColumns = pg.ColumnList{table.Jobs.ID, table.Jobs.Title, table.Jobs.Company, table.Jobs.Location,
	table.Jobs.WorkplaceType, table.Jobs.SalaryMin, table.Jobs.SalaryMax, table.Jobs.PostedAt, table.Jobs.URL}

// Typed NULLs, since a bare NULL projection has no type to scan by.
var (
	nullReal = pg.RawFloat("NULL::real")
	nullInt  = pg.RawInt("NULL::int")
)

// jobResult is one row of job search or detail.
type jobResult = struct {
	model.Jobs
	MatchScore   *float64
	MatchPercent *int32
	TagList      string
	Applied      bool
	Total        int
}

func toJob(r jobResult) (jobs.Job, error) {
	job := jobs.Job{
		ID:            int64(r.ID),
		Title:         r.Title,
		Company:       r.Company,
		Location:      deref(r.Location),
		WorkplaceType: jobs.WorkplaceType(r.WorkplaceType),
		SalaryMin:     intPtr(r.SalaryMin),
		SalaryMax:     intPtr(r.SalaryMax),
		PostedAt:      deref(r.PostedAt),
		URL:           r.URL,
		DetailURL:     deref(r.DetailURL),
		MatchScore:    r.MatchScore,
		MatchPercent:  intPtr(r.MatchPercent),
		Applied:       r.Applied,
	}

	if err := decodeJSON(&r.TagList, &job.Tags, "tags"); err != nil {
		return jobs.Job{}, err
	}

	return job, nil
}

func intPtr(v *int32) *int {
	if v == nil {
		return nil
	}

	return new(int(*v))
}

// jobTags is the job's tags as JSON array text, in tag id order.
func jobTags() pg.Expression {
	jobTags, tags := table.JobTags, table.Tags
	agg := pg.CustomExpression(pg.Token("json_agg("), tags.Tag, pg.Token("ORDER BY"), jobTags.TagID, pg.Token(")"))

	return pg.CAST(pg.COALESCE(
		scalar(pg.SELECT(agg).FROM(jobTags.INNER_JOIN(tags, tags.ID.EQ(jobTags.TagID))).WHERE(jobTags.JobID.EQ(table.Jobs.ID))),
		pg.Raw("'[]'"),
	)).AS_TEXT()
}

// applied is whether profileID marked the job applied; always false for anonymous callers.
func applied(profileID int64) pg.BoolExpression {
	if profileID == 0 {
		return pg.RawBool("false")
	}

	applications := table.ProfileJobApplications

	return pg.EXISTS(pg.SELECT(applications.ID).FROM(applications).
		WHERE(applications.ProfileID.EQ(pg.Int(profileID)).AND(applications.JobID.EQ(table.Jobs.ID))))
}

// resumeScore scores the job against the caller's resume; nil when there's no resume to score against.
func resumeScore(embedding *pgvector.Vector, query string) pg.FloatExpression {
	j := table.Jobs

	switch {
	case embedding != nil:
		return pg.FloatExp(pg.CASE().WHEN(j.Embedding.IS_NOT_NULL()).THEN(num(1).SUB(cosineDistance(j.Embedding, *embedding))))
	case query != "":
		return pg.FloatExp(pg.Func("ts_rank", j.SearchVector,
			pg.Raw("(SELECT websearch_to_tsquery('english', #query))", pg.RawArgs{"#query": query}), num(1)))
	default:
		return nil
	}
}

// resumeRank scores every listable job once (scored) and ranks those scores (ranked),
// so match_percent reads the same on any page, filter, or sort.
type resumeRank struct {
	scored, ranked     pg.CommonTableExpression
	scoredID, rankedID pg.ColumnInteger
	score, pct         pg.ColumnFloat
}

func newResumeRank() *resumeRank {
	j := table.Jobs
	r := &resumeRank{scored: pg.CTE("scored"), ranked: pg.CTE("ranked")}
	r.scoredID, r.score = j.ID.From(r.scored), pg.FloatColumn("score").From(r.scored)
	r.rankedID, r.pct = j.ID.From(r.ranked), pg.FloatColumn("pct").From(r.ranked)

	return r
}

// with defines both CTEs, scoring only jobs at least as new as cutoff, the jobs search can list.
func (r *resumeRank) with(score pg.FloatExpression, cutoff time.Time, stmt pg.Statement) pg.Statement {
	j := table.Jobs

	return pg.WITH(
		r.scored.AS_MATERIALIZED(pg.SELECT(j.ID, score.AS("score")).FROM(j).WHERE(jobAge().GT_EQ(pg.TimestampzT(cutoff)))),
		r.ranked.AS(pg.SELECT(r.scoredID, pg.PERCENT_RANK().OVER(pg.ORDER_BY(r.score)).AS("pct")).
			FROM(r.scored).WHERE(r.score.IS_NOT_NULL())),
	)(stmt)
}

func (r *resumeRank) percent() pg.IntegerExpression {
	return pg.CAST(pg.FLOOR(r.pct.MUL(num(100)))).AS_INTEGER()
}

func SearchForJobs(ctx context.Context, params *JobSearchParams) (*SearchResult, error) {
	// Jobs kept past JobMaxAge only because a user acted on them stay out of search.
	cutoff := time.Now().Add(-JobMaxAge)
	where, text := buildJobSearchWhere(params, cutoff)

	if text != nil {
		if err := text.loadWeights(ctx); err != nil {
			return nil, fmt.Errorf("search jobs: %w", err)
		}
	}

	var rows []jobResult
	if err := buildJobSearchSelect(params, cutoff, where, text).QueryContext(ctx, db(), &rows); err != nil {
		return nil, fmt.Errorf("search jobs: %w", err)
	}

	var results []jobs.Job
	total := 0

	for _, row := range rows {
		job, err := toJob(row)
		if err != nil {
			return nil, fmt.Errorf("search jobs: %w", err)
		}

		results = append(results, job)
		total = row.Total
	}

	// Past the last page no row carries the total, so count it separately.
	if len(results) == 0 && params.Offset > 0 {
		var count struct{ Total int }

		stmt := pg.SELECT(pg.COUNT(pg.STAR).AS("total")).FROM(table.Jobs).WHERE(where)
		if err := queryOne(ctx, db(), stmt, &count); err != nil {
			return nil, fmt.Errorf("count jobs: %w", err)
		}

		total = count.Total
	}

	return &SearchResult{Jobs: results, Total: total}, nil
}

func GetJobByID(ctx context.Context, id int64, params JobDetailParams) (*jobs.Job, error) {
	j := table.Jobs
	score := resumeScore(params.ResumeEmbedding, params.ResumeQuery)
	matchScore, percent := pg.FloatExpression(nullReal), pg.IntegerExpression(nullInt)

	var rank *resumeRank
	if score != nil {
		rank = newResumeRank()
		matchScore = score
		percent = pg.IntExp(scalar(pg.SELECT(rank.percent()).FROM(rank.ranked).WHERE(rank.rankedID.EQ(j.ID))))
	}

	var stmt pg.Statement = pg.SELECT(jobDisplayColumns, j.DetailURL, matchScore.AS("match_score"), percent.AS("match_percent"),
		jobTags().AS("tag_list"), applied(params.ProfileID).AS("applied")).
		FROM(j).
		WHERE(j.ID.EQ(pg.Int(id)))

	if rank != nil {
		stmt = rank.with(score, time.Now().Add(-JobMaxAge), stmt)
	}

	var row jobResult
	if err := queryOne(ctx, db(), stmt, &row); err != nil {
		return nil, err
	}

	job, err := toJob(row)
	if err != nil {
		return nil, err
	}

	return &job, nil
}

// buildJobSearchWhere is the search filter, the age cutoff always first, plus the typed query, nil without one.
func buildJobSearchWhere(params *JobSearchParams, cutoff time.Time) (pg.BoolExpression, *textSearch) {
	j := table.Jobs
	conditions := []pg.BoolExpression{jobAge().GT_EQ(pg.TimestampzT(cutoff))}

	var text *textSearch

	if terms := parseSearchQuery(params.SearchQuery); len(terms) > 0 {
		text = newTextSearch(terms, params.SearchEmbedding)
		conditions = append(conditions, text.match())
	}

	if params.WorkplaceType != jobs.Unknown {
		conditions = append(conditions, j.WorkplaceType.EQ(pg.Int(int64(params.WorkplaceType))))
	}

	if column, ok := jobTypeColumns[params.JobType]; ok {
		condition := pg.BoolExpression(column)

		// Intern takes precedence, so part/full-time exclude anything that also reads as an internship.
		if params.JobType != JobTypeFilterIntern {
			condition = condition.AND(pg.NOT(jobTypeColumns[JobTypeFilterIntern]))
		}

		conditions = append(conditions, condition)
	}

	if params.MinSalary > 0 {
		conditions = append(conditions, j.SalaryMax.GT_EQ(pg.Int(int64(params.MinSalary))))
	}

	if params.MaxSalary > 0 {
		conditions = append(conditions, j.SalaryMin.LT_EQ(pg.Int(int64(params.MaxSalary))))
	}

	if params.PostedAfter != nil {
		conditions = append(conditions, j.PostedAt.GT(pg.TimestampzT(*params.PostedAfter)))
	}

	if params.FirstSeenAfter != nil {
		conditions = append(conditions, j.FirstSeenAt.GT(pg.TimestampzT(*params.FirstSeenAfter)))
	}

	if len(params.Tags) > 0 {
		jobTags, tags := table.JobTags, table.Tags
		wanted := make([]pg.Expression, len(params.Tags))

		for i, tag := range params.Tags {
			wanted[i] = pg.String(tag)
		}

		conditions = append(conditions, j.ID.IN(pg.SELECT(jobTags.JobID).
			FROM(jobTags.INNER_JOIN(tags, tags.ID.EQ(jobTags.TagID))).
			WHERE(tags.Tag.IN(wanted...)).
			GROUP_BY(jobTags.JobID).
			HAVING(pg.COUNT(pg.DISTINCT(tags.Tag)).EQ(pg.Int(int64(len(params.Tags)))))))
	}

	return pg.AND(conditions...), text
}

// buildJobSearchSelect sorts and pages over (id, sort_key, score) only, then fetches display
// columns, percent, tags and applied for just the rows on the page.
func buildJobSearchSelect(params *JobSearchParams, cutoff time.Time, where pg.BoolExpression, text *textSearch) pg.Statement {
	j := table.Jobs
	rankable := params.Sort != SortDate

	// match_score shows as a fit signal even when sorting by date.
	score := resumeScore(params.ResumeEmbedding, params.ResumeQuery)
	pageScore := pg.FloatExpression(nullReal)
	from := pg.ReadableTable(j)

	var rank *resumeRank
	if score != nil {
		rank = newResumeRank()
		from = j.INNER_JOIN(rank.scored, rank.scoredID.EQ(j.ID))
		pageScore = rank.score
	}

	var sortKey pg.Expression

	switch {
	case rankable && text != nil && rank != nil:
		// Typed search stays primary; resume score just nudges ties toward a
		// good skill match, doesn't override an explicit query.
		sortKey = text.score().ADD(num(resumeNudgeWeight).MUL(pg.FloatExp(pg.COALESCE(rank.score, num(0)))))
	case rankable && text != nil:
		sortKey = text.score()
	case rankable && rank != nil:
		// NULLS LAST below puts jobs not embedded yet after scored ones.
		sortKey = rank.score
	default:
		sortKey = jobAge()
	}

	limit := params.Limit
	if limit <= 0 || limit > MaxSearchLimit {
		limit = DefaultSearchLimit
	}

	page := pg.SELECT(j.ID, sortKey.AS("sort_key"), pageScore.AS("score"), pg.COUNT(pg.STAR).OVER().AS("total")).
		FROM(from).
		WHERE(where).
		ORDER_BY(pg.FloatColumn("sort_key").DESC().NULLS_LAST(), j.ID.DESC()).
		LIMIT(int64(limit)).
		OFFSET(int64(max(params.Offset, 0))).
		AsTable("page")

	pageID, pageSortKey := j.ID.From(page), pg.FloatColumn("sort_key").From(page)
	source := page.INNER_JOIN(j, j.ID.EQ(pageID))
	percent := pg.IntegerExpression(nullInt)

	if rank != nil {
		source = source.LEFT_JOIN(rank.ranked, rank.rankedID.EQ(pageID))
		percent = rank.percent()
	}

	var stmt pg.Statement = pg.SELECT(jobDisplayColumns,
		pg.FloatColumn("score").From(page).AS("match_score"),
		percent.AS("match_percent"),
		jobTags().AS("tag_list"),
		applied(params.ProfileID).AS("applied"),
		pg.IntegerColumn("total").From(page).AS("total"),
	).FROM(source).ORDER_BY(pageSortKey.DESC().NULLS_LAST(), pageID.DESC())

	if rank != nil {
		stmt = rank.with(score, cutoff, stmt)
	}

	return stmt
}

func FetchAllUniqueTags(ctx context.Context) ([]string, error) {
	var rows []model.Tags

	if err := pg.SELECT(table.Tags.Tag).FROM(table.Tags).ORDER_BY(table.Tags.Tag).QueryContext(ctx, db(), &rows); err != nil {
		return nil, err
	}

	var tags []string

	for _, row := range rows {
		tags = append(tags, row.Tag)
	}

	return tags, nil
}
