package database

import (
	"context"
	"fmt"

	pg "github.com/go-jet/jet/v2/postgres"
	"github.com/pgvector/pgvector-go"

	"main/internal/gen/model"
	"main/internal/gen/table"
	"main/llm"
)

const embedJobsBatchSize = 50

// maxEmbeddingChars caps the text sent to Jina per job; the stored description stays whole.
const maxEmbeddingChars = 2000

// EmbedPendingJobs embeds every job with a NULL embedding, in batches, until
// none are left.
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
	var pending []model.Jobs

	jobs := table.Jobs
	stmt := pg.SELECT(jobs.ID, jobs.Title, jobs.Description).FROM(jobs).
		WHERE(jobs.Embedding.IS_NULL().AND(pg.StringExp(pg.COALESCE(jobs.Description, pg.String(""))).NOT_EQ(pg.String("")))).
		LIMIT(embedJobsBatchSize)

	if err := stmt.QueryContext(ctx, db(), &pending); err != nil {
		return 0, fmt.Errorf("query jobs pending embedding: %w", err)
	}

	if len(pending) == 0 {
		return 0, nil
	}

	texts := make([]string, len(pending))
	for i, job := range pending {
		texts[i] = jobEmbeddingText(job.Title, deref(job.Description))
	}

	embeddings, err := llm.EmbedTexts(ctx, texts)

	if err != nil {
		return 0, fmt.Errorf("embed jobs: %w", err)
	}

	for i, job := range pending {
		if _, err := exec(ctx, db(), jobs.UPDATE().
			SET(jobs.Embedding.SET(pg.StringExp(arg(pgvector.NewVector(embeddings[i]), "vector")))).
			WHERE(jobs.ID.EQ(pg.Int32(job.ID)))); err != nil {
			return 0, fmt.Errorf("save job embedding: %w", err)
		}
	}

	return len(pending), nil
}

// jobEmbeddingText is the title plus the opening of the description, where role, stack, and requirements usually are.
func jobEmbeddingText(title, description string) string {
	text := []rune(title + "\n\n" + description)

	if len(text) > maxEmbeddingChars {
		text = text[:maxEmbeddingChars]
	}

	return string(text)
}
