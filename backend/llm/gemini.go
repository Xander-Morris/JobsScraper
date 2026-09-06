package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	defaultGeminiModel      = "gemini-2.5-flash"
	defaultGeminiEmbedModel = "text-embedding-004"
	geminiAPIBase           = "https://generativelanguage.googleapis.com/v1beta"
)

func geminiAPIKey() string {
	return os.Getenv("GEMINI_API_KEY")
}

func geminiModel() string {
	if model := os.Getenv("GEMINI_MODEL"); model != "" {
		return model
	}

	return defaultGeminiModel
}

func geminiEmbedModel() string {
	if model := os.Getenv("GEMINI_EMBED_MODEL"); model != "" {
		return model
	}

	return defaultGeminiEmbedModel
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerateRequest struct {
	Contents         []geminiContent        `json:"contents"`
	GenerationConfig geminiGenerationConfig `json:"generationConfig"`
}

type geminiGenerationConfig struct {
	ResponseMimeType string         `json:"responseMimeType"`
	ResponseSchema   map[string]any `json:"responseSchema"`
	Temperature      float64        `json:"temperature"`
}

type geminiGenerateResponse struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
}

// callGeminiChat sends a single-turn prompt to Gemini constrained to the given
// JSON-Schema-shaped map (the same shape our callers already build for Ollama's
// structured output), and returns the raw JSON text Gemini generated.
func callGeminiChat(ctx context.Context, prompt string, schema map[string]any) (string, error) {
	apiKey := geminiAPIKey()
	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY is not set")
	}

	reqBody := geminiGenerateRequest{
		Contents: []geminiContent{{Role: "user", Parts: []geminiPart{{Text: prompt}}}},
		GenerationConfig: geminiGenerationConfig{
			ResponseMimeType: "application/json",
			ResponseSchema:   toGeminiSchema(schema),
			Temperature:      0,
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("encode request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent", geminiAPIBase, geminiModel())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call gemini: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read gemini response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini returned %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed geminiGenerateResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("decode gemini response: %w", err)
	}

	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini returned no candidates")
	}

	text := parsed.Candidates[0].Content.Parts[0].Text
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("gemini returned an empty response")
	}

	return text, nil
}

// toGeminiSchema recursively upper-cases the "type" values in a JSON-Schema-shaped
// map to match Gemini's OpenAPI-subset Schema format, which is otherwise the same
// shape ("properties", "items", "required" all mean the same thing).
func toGeminiSchema(schema map[string]any) map[string]any {
	out := make(map[string]any, len(schema))

	for k, v := range schema {
		switch k {
		case "type":
			if s, ok := v.(string); ok {
				out[k] = strings.ToUpper(s)
				continue
			}

			out[k] = v
		case "properties":
			if props, ok := v.(map[string]any); ok {
				converted := make(map[string]any, len(props))

				for pk, pv := range props {
					if pm, ok := pv.(map[string]any); ok {
						converted[pk] = toGeminiSchema(pm)
					} else {
						converted[pk] = pv
					}
				}

				out[k] = converted
				continue
			}

			out[k] = v
		case "items":
			if im, ok := v.(map[string]any); ok {
				out[k] = toGeminiSchema(im)
				continue
			}

			out[k] = v
		default:
			out[k] = v
		}
	}

	return out
}

type geminiEmbedContentRequest struct {
	Model   string        `json:"model"`
	Content geminiContent `json:"content"`
}

type geminiEmbedRequest struct {
	Requests []geminiEmbedContentRequest `json:"requests"`
}

type geminiEmbedResponse struct {
	Embeddings []struct {
		Values []float32 `json:"values"`
	} `json:"embeddings"`
}

// callGeminiEmbed embeds a batch of texts in a single Gemini request, returning
// one vector per input text in the same order.
func callGeminiEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	apiKey := geminiAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is not set")
	}

	model := "models/" + geminiEmbedModel()

	requests := make([]geminiEmbedContentRequest, len(texts))
	for i, text := range texts {
		requests[i] = geminiEmbedContentRequest{
			Model:   model,
			Content: geminiContent{Parts: []geminiPart{{Text: text}}},
		}
	}

	payload, err := json.Marshal(geminiEmbedRequest{Requests: requests})
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	url := fmt.Sprintf("%s/%s:batchEmbedContents", geminiAPIBase, model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call gemini: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read gemini response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini returned %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed geminiEmbedResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("decode gemini response: %w", err)
	}

	if len(parsed.Embeddings) != len(texts) {
		return nil, fmt.Errorf("gemini returned %d embeddings for %d inputs", len(parsed.Embeddings), len(texts))
	}

	out := make([][]float32, len(texts))
	for i, e := range parsed.Embeddings {
		out[i] = e.Values
	}

	return out, nil
}
