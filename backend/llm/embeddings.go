package llm

import (
	"context"
)

// EmbedTexts embeds a batch of texts in a single Gemini request, returning one
// vector per input text in the same order. Callers should treat a failure here
// as non-fatal — embeddings are a best-effort enhancement to resume/job matching,
// not a hard dependency (search and digest both fall back to keyword matching
// when an embedding isn't available).
func EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	return callJinaEmbed(ctx, texts)
}
