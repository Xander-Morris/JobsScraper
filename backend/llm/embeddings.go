package llm

import (
	"context"
)

// EmbedTexts embeds a batch of texts in one Jina request, one vector per input
// in the same order. Treat failures as non-fatal — search and digest fall back
// to keyword matching when there's no embedding.
func EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	return callJinaEmbed(ctx, texts)
}
