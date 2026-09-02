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

const defaultOllamaEmbedModel = "nomic-embed-text"

func ollamaEmbedModel() string {
	if model := os.Getenv("OLLAMA_EMBED_MODEL"); model != "" {
		return model
	}

	return defaultOllamaEmbedModel
}

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// EmbedTexts embeds a batch of texts in a single Ollama request, returning one
// vector per input text in the same order. Callers should treat a failure here
// as non-fatal — embeddings are a best-effort enhancement to resume/job matching,
// not a hard dependency (search and digest both fall back to keyword matching
// when an embedding isn't available).
func EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	payload, err := json.Marshal(embedRequest{Model: ollamaEmbedModel(), Input: texts})
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ollamaBaseURL()+"/api/embed", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("content-type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call ollama (is `ollama serve` running at %s?): %w", ollamaBaseURL(), err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read ollama response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed embedResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("decode ollama response: %w", err)
	}

	if len(parsed.Embeddings) != len(texts) {
		return nil, fmt.Errorf("ollama returned %d embeddings for %d inputs", len(parsed.Embeddings), len(texts))
	}

	return parsed.Embeddings, nil
}
