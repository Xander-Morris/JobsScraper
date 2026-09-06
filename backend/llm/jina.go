package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const (
	defaultJinaEmbedModel = "jina-embeddings-v2-base-en"
	jinaAPIURL            = "https://api.jina.ai/v1/embeddings"
)

func jinaAPIKey() string {
	return os.Getenv("JINA_API_KEY")
}

func jinaEmbedModel() string {
	if model := os.Getenv("JINA_EMBED_MODEL"); model != "" {
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

	payload, err := json.Marshal(jinaEmbedRequest{Model: jinaEmbedModel(), Input: texts})
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, jinaAPIURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call jina: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read jina response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jina returned %d: %s", resp.StatusCode, string(respBody))
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
