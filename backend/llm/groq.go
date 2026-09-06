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
	defaultGroqModel = "llama-3.3-70b-versatile"
	groqAPIURL       = "https://api.groq.com/openai/v1/chat/completions"
)

func groqAPIKey() string {
	return os.Getenv("GROQ_API_KEY")
}

func groqModel() string {
	if model := os.Getenv("GROQ_MODEL"); model != "" {
		return model
	}

	return defaultGroqModel
}

type groqChatRequest struct {
	Model          string             `json:"model"`
	Messages       []groqChatMessage  `json:"messages"`
	ResponseFormat groqResponseFormat `json:"response_format"`
	Temperature    float64            `json:"temperature"`
}

type groqChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqResponseFormat struct {
	Type string `json:"type"`
}

type groqChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// callGroqChat sends a single-turn prompt to Groq's OpenAI-compatible chat
// completions endpoint, constrained to valid JSON via response_format
// (json_object mode guarantees syntactically valid JSON, not adherence to a
// specific shape — so schema is rendered into the prompt itself as the actual
// constraint on field names/types).
func callGroqChat(ctx context.Context, prompt string, schema map[string]any) (string, error) {
	apiKey := groqAPIKey()
	if apiKey == "" {
		return "", fmt.Errorf("GROQ_API_KEY is not set")
	}

	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		return "", fmt.Errorf("encode schema: %w", err)
	}

	fullPrompt := prompt + "\n\nRespond with a single JSON object matching this schema exactly " +
		"(same field names and types, no extra commentary, no markdown fences):\n" + string(schemaJSON)

	reqBody := groqChatRequest{
		Model: groqModel(),
		Messages: []groqChatMessage{
			{Role: "user", Content: fullPrompt},
		},
		ResponseFormat: groqResponseFormat{Type: "json_object"},
		Temperature:    0,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, groqAPIURL, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call groq: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read groq response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("groq returned %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed groqChatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("decode groq response: %w", err)
	}

	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("groq returned no choices")
	}

	text := parsed.Choices[0].Message.Content
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("groq returned an empty response")
	}

	return text, nil
}
