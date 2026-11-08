package database

import (
	"context"
	"fmt"
	"slices"

	pg "github.com/go-jet/jet/v2/postgres"
	"github.com/pgvector/pgvector-go"

	"main/internal/gen/model"
	"main/internal/gen/table"
	"main/jobs"
	"main/llm"
)

const embedJobsBatchSize = 50

// maxEmbeddingChars caps the text sent to Jina per job.
const maxEmbeddingChars = 2000

// EmbedPendingJobs embeds the scraped jobs that have no embedding yet, from their in-memory
// descriptions since those aren't stored. Jobs missed here retry on the next scrape.
func EmbedPendingJobs(ctx context.Context, scraped []jobs.Job) error {
	byURL := make(map[string]jobs.Job, len(scraped))
	urls := make([]string, 0, len(scraped))

	for _, job := range scraped {
		if job.Description != "" {
			byURL[job.URL] = job
			urls = append(urls, job.URL)
		}
	}

	for chunk := range slices.Chunk(urls, writeJobsChunkSize) {
		var pending []model.Jobs

		j := table.Jobs
		stmt := pg.SELECT(j.ID, j.URL).FROM(j).WHERE(j.Embedding.IS_NULL().AND(eqAny(j.URL, chunk, "text[]")))

		if err := stmt.QueryContext(ctx, db(), &pending); err != nil {
			return fmt.Errorf("query jobs pending embedding: %w", err)
		}

		for batch := range slices.Chunk(pending, embedJobsBatchSize) {
			if err := embedJobBatch(ctx, batch, byURL); err != nil {
				return err
			}
		}
	}

	return nil
}

func embedJobBatch(ctx context.Context, batch []model.Jobs, byURL map[string]jobs.Job) error {
	texts := make([]string, len(batch))
	for i, row := range batch {
		job := byURL[row.URL]
		texts[i] = jobEmbeddingText(job.Title, job.Description)
	}

	embeddings, err := llm.EmbedTexts(ctx, texts)
	if err != nil {
		return fmt.Errorf("embed jobs: %w", err)
	}

	j := table.Jobs

	for i, row := range batch {
		if _, err := exec(ctx, db(), j.UPDATE().
			SET(j.Embedding.SET(pg.StringExp(arg(pgvector.NewVector(embeddings[i]), "vector")))).
			WHERE(j.ID.EQ(pg.Int32(row.ID)))); err != nil {
			return fmt.Errorf("save job embedding: %w", err)
		}
	}

	return nil
}

// jobEmbeddingText is the title plus the opening of the description, where role, stack, and requirements usually are.
func jobEmbeddingText(title, description string) string {
	text := []rune(title + "\n\n" + description)

	if len(text) > maxEmbeddingChars {
		text = text[:maxEmbeddingChars]
	}

	return string(text)
}
