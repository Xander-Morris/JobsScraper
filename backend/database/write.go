package database

import (
	"database/sql"
	"fmt"
	"main/jobs"
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

	if err := runMigrations(db); err != nil {
		return err
	}

	createdTables = true

	return nil
}

// writeJobsChunkSize bounds how many jobs go into each bulk statement.
const writeJobsChunkSize = 500

// Keeps a known post date when a later scrape of the same URL has none.
const upsertJobsSQL = `INSERT INTO jobs (title, company, location, workplace_type, salary_min, salary_max, posted_at, url, description,
		is_intern, is_part_time, is_full_time)
	SELECT * FROM unnest($1::text[], $2::text[], $3::text[], $4::int[], $5::int[], $6::int[], $7::timestamptz[], $8::text[], $9::text[],
		$10::bool[], $11::bool[], $12::bool[])
	ON CONFLICT (url) DO UPDATE SET
		title = excluded.title,
		company = excluded.company,
		location = excluded.location,
		workplace_type = excluded.workplace_type,
		salary_min = excluded.salary_min,
		salary_max = excluded.salary_max,
		posted_at = COALESCE(excluded.posted_at, jobs.posted_at),
		description = excluded.description,
		is_intern = excluded.is_intern,
		is_part_time = excluded.is_part_time,
		is_full_time = excluded.is_full_time
	RETURNING id, url`

func writeJobs(tx *sql.Tx, batch []jobs.Job) error {
	batch = dedupeJobsByURL(batch)

	for start := 0; start < len(batch); start += writeJobsChunkSize {
		chunk := batch[start:min(start+writeJobsChunkSize, len(batch))]

		jobIDs, err := upsertJobs(tx, chunk)
		if err != nil {
			return err
		}

		if err := replaceJobTags(tx, chunk, jobIDs); err != nil {
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

func upsertJobs(tx *sql.Tx, chunk []jobs.Job) (map[string]int64, error) {
	n := len(chunk)
	titles, companies, locations := make([]string, n), make([]string, n), make([]string, n)
	urls, descriptions := make([]string, n), make([]string, n)
	workplaceTypes := make([]int32, n)
	salaryMins, salaryMaxes := make([]*int32, n), make([]*int32, n)
	postedAts := make([]*time.Time, n)
	interns, partTimes, fullTimes := make([]bool, n), make([]bool, n), make([]bool, n)

	for i, job := range chunk {
		titles[i], companies[i], locations[i] = job.Title, job.Company, job.Location
		urls[i], descriptions[i] = job.URL, job.Description
		workplaceTypes[i] = int32(job.WorkplaceType)
		salaryMins[i], salaryMaxes[i] = int32Ptr(job.SalaryMin), int32Ptr(job.SalaryMax)

		flags := jobTypeFlagsFor(job)
		interns[i], partTimes[i], fullTimes[i] = flags.intern, flags.partTime, flags.fullTime

		if !job.PostedAt.IsZero() {
			postedAt := job.PostedAt.UTC()
			postedAts[i] = &postedAt
		}
	}

	rows, err := tx.Query(upsertJobsSQL, titles, companies, locations, workplaceTypes, salaryMins, salaryMaxes, postedAts, urls, descriptions,
		interns, partTimes, fullTimes)
	if err != nil {
		return nil, fmt.Errorf("upsert jobs: %w", err)
	}
	defer rows.Close()

	ids := make(map[string]int64, n)

	for rows.Next() {
		var id int64
		var url string

		if err := rows.Scan(&id, &url); err != nil {
			return nil, fmt.Errorf("scan upserted job: %w", err)
		}

		ids[url] = id
	}

	return ids, rows.Err()
}

func replaceJobTags(tx *sql.Tx, chunk []jobs.Job, jobIDs map[string]int64) error {
	ids := make([]int64, 0, len(jobIDs))
	for _, id := range jobIDs {
		ids = append(ids, id)
	}

	if _, err := tx.Exec("DELETE FROM job_tags WHERE job_id = ANY($1::int[])", ids); err != nil {
		return fmt.Errorf("clear job tags: %w", err)
	}

	var uniqueTags []string
	seen := make(map[string]bool)

	for _, job := range chunk {
		for _, tag := range job.Tags {
			if !seen[tag] {
				seen[tag] = true
				uniqueTags = append(uniqueTags, tag)
			}
		}
	}

	if len(uniqueTags) == 0 {
		return nil
	}

	tagIDs, err := upsertTags(tx, uniqueTags)
	if err != nil {
		return err
	}

	var pairJobIDs, pairTagIDs []int64

	for _, job := range chunk {
		for _, tag := range job.Tags {
			pairJobIDs = append(pairJobIDs, jobIDs[job.URL])
			pairTagIDs = append(pairTagIDs, tagIDs[tag])
		}
	}

	if _, err := tx.Exec(`INSERT INTO job_tags (job_id, tag_id) SELECT * FROM unnest($1::int[], $2::int[])
		ON CONFLICT (job_id, tag_id) DO NOTHING`, pairJobIDs, pairTagIDs); err != nil {
		return fmt.Errorf("insert job tags: %w", err)
	}

	return nil
}

func upsertTags(tx *sql.Tx, tags []string) (map[string]int64, error) {
	rows, err := tx.Query(`INSERT INTO tags (tag) SELECT unnest($1::text[])
		ON CONFLICT (tag) DO UPDATE SET tag = excluded.tag
		RETURNING id, tag`, tags)
	if err != nil {
		return nil, fmt.Errorf("upsert tags: %w", err)
	}
	defer rows.Close()

	ids := make(map[string]int64, len(tags))

	for rows.Next() {
		var id int64
		var tag string

		if err := rows.Scan(&id, &tag); err != nil {
			return nil, fmt.Errorf("scan upserted tag: %w", err)
		}

		ids[tag] = id
	}

	return ids, rows.Err()
}

func int32Ptr(v *int) *int32 {
	if v == nil {
		return nil
	}

	n := int32(*v)

	return &n
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
