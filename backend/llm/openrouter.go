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

// defaultOpenRouterModel is a free-tier model (":free" suffix, no cost, rate
// limited), verified against the actual resumeSchema() shape. Takes ~20-30s
// for a full resume extraction, comfortably under resumeExtractionTimeout.
// OpenRouter's lineup of free models turns over often (some go paid-only,
// others get upstream-rate-limited across all free users sharing that slot).
// If this one stops working, pick a current one from
// https://openrouter.ai/api/v1/models (filter for pricing.prompt == "0") and
// set OPENROUTER_MODEL instead of changing this default.
const defaultOpenRouterModel = "minimax/minimax-m2.7:free"

const openRouterAPIURL = "https://openrouter.ai/api/v1/chat/completions"

func openRouterAPIKey() string {
	return os.Getenv("OPENROUTER_API_KEY")
}

func openRouterModel() string {
	if model := os.Getenv("OPENROUTER_MODEL"); model != "" {
		return model
	}

	return defaultOpenRouterModel
}

type openRouterChatRequest struct {
	Model          string                   `json:"model"`
	Messages       []openRouterChatMessage  `json:"messages"`
	ResponseFormat openRouterResponseFormat `json:"response_format"`
	Temperature    float64                  `json:"temperature"`
}

type openRouterChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterResponseFormat struct {
	Type string `json:"type"`
}

type openRouterChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// callOpenRouterChat sends a single-turn prompt to OpenRouter's OpenAI-compatible
// chat completions endpoint, constrained to valid JSON via response_format
// (json_object mode guarantees syntactically valid JSON, not adherence to a
// specific shape, so schema is rendered into the prompt itself as the actual
// constraint on field names/types, same approach as the Groq client this
// replaced).
func callOpenRouterChat(ctx context.Context, prompt string, schema map[string]any) (string, error) {
	apiKey := openRouterAPIKey()
	if apiKey == "" {
		return "", fmt.Errorf("OPENROUTER_API_KEY is not set")
	}

	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		return "", fmt.Errorf("encode schema: %w", err)
	}

	fullPrompt := prompt + "\n\nRespond with a single JSON object matching this schema exactly " +
		"(same field names and types, no extra commentary, no markdown fences):\n" + string(schemaJSON)

	reqBody := openRouterChatRequest{
		Model: openRouterModel(),
		Messages: []openRouterChatMessage{
			{Role: "user", Content: fullPrompt},
		},
		ResponseFormat: openRouterResponseFormat{Type: "json_object"},
		Temperature:    0,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterAPIURL, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call openrouter: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read openrouter response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openrouter returned %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed openRouterChatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("decode openrouter response: %w", err)
	}

	if parsed.Error != nil {
		return "", fmt.Errorf("openrouter error: %s", parsed.Error.Message)
	}

	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("openrouter returned no choices")
	}

	text := parsed.Choices[0].Message.Content
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("openrouter returned an empty response")
	}

	return text, nil
}
