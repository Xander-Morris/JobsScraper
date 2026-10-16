package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"main/utils"
)

const (
	defaultJinaEmbedModel = "jina-embeddings-v2-base-en"
	jinaAPIURL            = "https://api.jina.ai/v1/embeddings"
)

var errJinaRateLimited = errors.New("jina rate limited")

func jinaAPIKey() string {
	return utils.Getenv("JINA_API_KEY")
}

func jinaEmbedModel() string {
	if model := utils.Getenv("JINA_EMBED_MODEL"); model != "" {
		return model
	}

	return defaultJinaEmbedModel
}

type jinaEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type jinaEmbedResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// callJinaEmbed embeds a batch of texts in a single Jina AI request, returning
// one vector per input text in the same order. jina-embeddings-v2-base-en
// outputs 768-dim vectors, matching the `vector(768)` columns already in place.
func callJinaEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	apiKey := jinaAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("JINA_API_KEY is not set")
	}

	status, respBody, err := postJSON(ctx, jinaAPIURL, apiKey, jinaEmbedRequest{Model: jinaEmbedModel(), Input: texts})
	if err != nil {
		return nil, fmt.Errorf("call jina: %w", err)
	}

	if status == http.StatusTooManyRequests {
		return nil, fmt.Errorf("%w: %s", errJinaRateLimited, errorBody(respBody))
	}

	if status != http.StatusOK {
		return nil, fmt.Errorf("jina returned %d: %s", status, errorBody(respBody))
	}

	var parsed jinaEmbedResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("decode jina response: %w", err)
	}

	if len(parsed.Data) != len(texts) {
		return nil, fmt.Errorf("jina returned %d embeddings for %d inputs", len(parsed.Data), len(texts))
	}

	out := make([][]float32, len(texts))
	for _, d := range parsed.Data {
		if d.Index < 0 || d.Index >= len(out) {
			return nil, fmt.Errorf("jina returned out-of-range embedding index %d", d.Index)
		}

		out[d.Index] = d.Embedding
	}

	return out, nil
}
