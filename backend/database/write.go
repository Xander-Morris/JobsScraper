package database

import (
	"context"
	"fmt"
	"math"
	"slices"
	"sync"
	"time"

	pg "github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"

	"main/internal/gen/model"
	"main/internal/gen/table"
	"main/jobs"
)

var (
	migrateMu sync.Mutex
	migrated  bool
)

// CreateTables applies pending migrations once per process; a failure is retried on the next call.
func CreateTables() error {
	migrateMu.Lock()
	defer migrateMu.Unlock()

	if migrated {
		return nil
	}

	if err := runMigrations(db()); err != nil {
		return err
	}

	migrated = true

	return nil
}

// writeJobsChunkSize bounds how many jobs go into each bulk statement, keeping params under Postgres's 65535.
const writeJobsChunkSize = 500

// writeTagsChunkSize bounds the tag and job-tag rows per bulk statement.
const writeTagsChunkSize = 2000

// jobWriteColumns are the scraped columns; the rest are generated or filled in later.
var jobWriteColumns = table.Jobs.MutableColumns.Except(table.Jobs.Embedding, table.Jobs.FirstSeenAt, table.Jobs.SearchVector)

func writeJobs(ctx context.Context, tx qrm.DB, batch []jobs.Job) error {
	for chunk := range slices.Chunk(dedupeJobsByURL(batch), writeJobsChunkSize) {
		jobIDs, err := upsertJobs(ctx, tx, chunk)
		if err != nil {
			return err
		}

		if err := replaceJobTags(ctx, tx, chunk, jobIDs); err != nil {
			return err
		}
	}

	return nil
}

// dedupeJobsByURL keeps the last job per URL, since one bulk upsert can't touch the same row twice.
func dedupeJobsByURL(batch []jobs.Job) []jobs.Job {
	index := make(map[string]int, len(batch))
	deduped := make([]jobs.Job, 0, len(batch))

	for _, job := range batch {
		if i, ok := index[job.URL]; ok {
			deduped[i] = job
			continue
		}

		index[job.URL] = len(deduped)
		deduped = append(deduped, job)
	}

	return deduped
}

func jobRow(job jobs.Job) model.Jobs {
	flags := jobTypeFlagsFor(job)
	row := model.Jobs{
		Title:         job.Title,
		Company:       job.Company,
		Location:      &job.Location,
		WorkplaceType: int32(job.WorkplaceType),
		SalaryMin:     salaryPtr(job.SalaryMin),
		SalaryMax:     salaryPtr(job.SalaryMax),
		URL:           job.URL,
		Description:   &job.Description,
		IsIntern:      flags.intern,
		IsPartTime:    flags.partTime,
		IsFullTime:    flags.fullTime,
	}

	// A post date in the future is bad source data, so it's stored as unknown.
	if !job.PostedAt.IsZero() && job.PostedAt.Before(time.Now().Add(maxPostedAtSkew)) {
		postedAt := job.PostedAt.UTC()
		row.PostedAt = &postedAt
	}

	return row
}

func upsertJobs(ctx context.Context, tx qrm.DB, chunk []jobs.Job) (map[string]int64, error) {
	rows := make([]model.Jobs, len(chunk))
	for i, job := range chunk {
		rows[i] = jobRow(job)
	}

	jobs := table.Jobs
	stmt := jobs.INSERT(jobWriteColumns).MODELS(rows).
		ON_CONFLICT(jobs.URL).DO_UPDATE(pg.SET(
		setExcluded(jobWriteColumns, jobs.EXCLUDED.AllColumns, jobs.URL, jobs.PostedAt),
		// Keeps a known post date when a later scrape of the same URL has none.
		jobs.PostedAt.SET(pg.TimestampzExp(pg.COALESCE(jobs.EXCLUDED.PostedAt, jobs.PostedAt))),
	)).
		RETURNING(jobs.ID, jobs.URL)

	var upserted []model.Jobs
	if err := stmt.QueryContext(ctx, tx, &upserted); err != nil {
		return nil, fmt.Errorf("upsert jobs: %w", err)
	}

	ids := make(map[string]int64, len(upserted))
	for _, row := range upserted {
		ids[row.URL] = int64(row.ID)
	}

	return ids, nil
}

func replaceJobTags(ctx context.Context, tx qrm.DB, chunk []jobs.Job, jobIDs map[string]int64) error {
	ids := make([]int64, 0, len(jobIDs))
	for _, id := range jobIDs {
		ids = append(ids, id)
	}

	jobTags := table.JobTags
	if _, err := exec(ctx, tx, jobTags.DELETE().WHERE(eqAny(jobTags.JobID, ids, "int[]"))); err != nil {
		return fmt.Errorf("clear job tags: %w", err)
	}

	var uniqueTags []model.Tags
	seen := make(map[string]bool)

	for _, job := range chunk {
		for _, tag := range job.Tags {
			if !seen[tag] {
				seen[tag] = true
				uniqueTags = append(uniqueTags, model.Tags{Tag: tag})
			}
		}
	}

	if len(uniqueTags) == 0 {
		return nil
	}

	tagIDs, err := upsertTags(ctx, tx, uniqueTags)
	if err != nil {
		return err
	}

	var pairs []model.JobTags

	for _, job := range chunk {
		jobID := int32(jobIDs[job.URL])
		for _, tag := range job.Tags {
			pairs = append(pairs, model.JobTags{JobID: &jobID, TagID: new(tagIDs[tag])})
		}
	}

	for batch := range slices.Chunk(pairs, writeTagsChunkSize) {
		stmt := jobTags.INSERT(jobTags.AllColumns).MODELS(batch).ON_CONFLICT(jobTags.JobID, jobTags.TagID).DO_NOTHING()
		if _, err := exec(ctx, tx, stmt); err != nil {
			return fmt.Errorf("insert job tags: %w", err)
		}
	}

	return nil
}

func upsertTags(ctx context.Context, tx qrm.DB, tags []model.Tags) (map[string]int32, error) {
	ids := make(map[string]int32, len(tags))

	for batch := range slices.Chunk(tags, writeTagsChunkSize) {
		var upserted []model.Tags

		stmt := table.Tags.INSERT(table.Tags.Tag).MODELS(batch).
			ON_CONFLICT(table.Tags.Tag).DO_UPDATE(pg.SET(setExcluded(pg.ColumnList{table.Tags.Tag}, table.Tags.EXCLUDED.AllColumns))).
			RETURNING(table.Tags.AllColumns)

		if err := stmt.QueryContext(ctx, tx, &upserted); err != nil {
			return nil, fmt.Errorf("upsert tags: %w", err)
		}

		for _, row := range upserted {
			ids[row.Tag] = row.ID
		}
	}

	return ids, nil
}

// maxPostedAtSkew allows for source clocks and time zones running a little ahead.
const maxPostedAtSkew = 24 * time.Hour

// salaryPtr drops amounts too small to be a yearly salary, or too big for the column.
func salaryPtr(v *int) *int32 {
	if v == nil || *v < jobs.MinSalary || *v > math.MaxInt32 {
		return nil
	}

	n := int32(*v)

	return &n
}

// KnownJobURLs reports which of urls are already stored, so the crawler can skip refetching them.
func KnownJobURLs(ctx context.Context, urls []string) (map[string]bool, error) {
	var rows []model.Jobs

	stmt := pg.SELECT(table.Jobs.URL).FROM(table.Jobs).WHERE(eqAny(table.Jobs.URL, urls, "text[]"))
	if err := stmt.QueryContext(ctx, db(), &rows); err != nil {
		return nil, fmt.Errorf("query known job urls: %w", err)
	}

	known := make(map[string]bool, len(rows))
	for _, row := range rows {
		known[row.URL] = true
	}

	return known, nil
}

func WriteJobsToDatabase(ctx context.Context, jobs []jobs.Job) error {
	if err := CreateTables(); err != nil {
		return err
	}

	tx, err := db().BeginTx(ctx, nil)

	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	defer tx.Rollback()

	if err := writeJobs(ctx, tx, jobs); err != nil {
		return fmt.Errorf("failed to write jobs: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
