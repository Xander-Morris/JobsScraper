package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pgvector/pgvector-go"

	"main/llm"
)

const embedJobsBatchSize = 50

// EmbedPendingJobs embeds every job whose embedding is still NULL, in batches,
// until none are left. Only ever processes a job once — descriptions are
// effectively static once posted, so re-scraping the same job again later won't
// re-embed it. Best-effort: callers should log and continue on error rather than
// fail the scrape cycle over it, since an unreachable Ollama isn't a hard
// requirement for scraping to keep working (search/digest fall back to keyword
// matching for any job without an embedding).
func EmbedPendingJobs(ctx context.Context) error {
	for {
		n, err := embedNextJobBatch(ctx)

		if err != nil {
			return err
		}

		if n < embedJobsBatchSize {
			return nil
		}
	}
}

func embedNextJobBatch(ctx context.Context) (int, error) {
	db, err := GetDb()

	if err != nil {
		return 0, err
	}

	rows, err := db.QueryContext(ctx,
		`SELECT id, description FROM jobs WHERE embedding IS NULL AND coalesce(description, '') <> '' LIMIT $1`,
		embedJobsBatchSize)

	if err != nil {
		return 0, fmt.Errorf("query jobs pending embedding: %w", err)
	}

	ids, texts, err := scanPendingJobs(rows)

	if err != nil {
		return 0, err
	}

	if len(ids) == 0 {
		return 0, nil
	}

	embeddings, err := llm.EmbedTexts(ctx, texts)

	if err != nil {
		return 0, fmt.Errorf("embed jobs: %w", err)
	}

	for i, id := range ids {
		if _, err := db.ExecContext(ctx, `UPDATE jobs SET embedding = $1 WHERE id = $2`,
			pgvector.NewVector(embeddings[i]), id); err != nil {
			return 0, fmt.Errorf("save job embedding: %w", err)
		}
	}

	return len(ids), nil
}

func scanPendingJobs(rows *sql.Rows) (ids []int64, texts []string, err error) {
	defer rows.Close()

	for rows.Next() {
		var id int64
		var description string

		if err := rows.Scan(&id, &description); err != nil {
			return nil, nil, fmt.Errorf("scan pending job: %w", err)
		}

		ids = append(ids, id)
		texts = append(texts, description)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate pending jobs: %w", err)
	}

	return ids, texts, nil
}
