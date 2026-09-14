package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pgvector/pgvector-go"

	"main/llm"
)

const embedJobsBatchSize = 50

// maxEmbeddingChars caps the text sent to Jina per job; the stored description stays whole.
const maxEmbeddingChars = 2000

// EmbedPendingJobs embeds every job with a NULL embedding, in batches, until
// none are left. Each job only gets embedded once; descriptions don't change
// after posting, so re-scraping won't re-trigger it. Best-effort: log and move
// on if this fails, don't fail the scrape cycle over it. Search/digest just
// fall back to keyword matching for jobs with no embedding.
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
		`SELECT id, title, description FROM jobs WHERE embedding IS NULL AND coalesce(description, '') <> '' LIMIT $1`,
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

// jobEmbeddingText is the title plus the opening of the description, where role, stack, and requirements usually are.
func jobEmbeddingText(title, description string) string {
	text := []rune(title + "\n\n" + description)

	if len(text) > maxEmbeddingChars {
		text = text[:maxEmbeddingChars]
	}

	return string(text)
}

func scanPendingJobs(rows *sql.Rows) (ids []int64, texts []string, err error) {
	defer rows.Close()

	for rows.Next() {
		var id int64
		var title, description string

		if err := rows.Scan(&id, &title, &description); err != nil {
			return nil, nil, fmt.Errorf("scan pending job: %w", err)
		}

		ids = append(ids, id)
		texts = append(texts, jobEmbeddingText(title, description))
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate pending jobs: %w", err)
	}

	return ids, texts, nil
}
