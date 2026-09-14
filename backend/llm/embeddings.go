package llm

import (
	"context"
	"errors"
	"time"
)

// Free Jina keys allow 100k tokens a minute, so a 429 clears after about a minute.
const (
	jinaRateLimitWait    = time.Minute
	jinaMaxEmbedAttempts = 3
)

// EmbedTexts embeds a batch of texts in one Jina request, one vector per input
// in the same order. Treat failures as non-fatal; search and digest fall back
// to keyword matching when there's no embedding.
func EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	for attempt := 1; ; attempt++ {
		vectors, err := callJinaEmbed(ctx, texts)
		if !errors.Is(err, errJinaRateLimited) || attempt == jinaMaxEmbedAttempts {
			return vectors, err
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(jinaRateLimitWait):
		}
	}
}
