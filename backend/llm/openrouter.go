package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"main/utils"
)

// defaultOpenRouterModels are free-tier fallbacks OpenRouter tries in order, smoke-tested
// 2026-10-21 against triviaSchema() and resumeSchema(). Free models come and go, so if
// these die, grab live ones from https://openrouter.ai/api/v1/models (pricing.prompt ==
// "0") and set OPENROUTER_MODEL (comma-separated) rather than editing this.
var defaultOpenRouterModels = []string{
	"dots-studio/dots-3-note-preview:free",
	"nvidia/nemotron-3-ultra-550b-a55b:free",
	"xingchen-agi/xing4.0-29b-a4b:free",
}

// maxOpenRouterModels is OpenRouter's cap on the models fallback list.
const maxOpenRouterModels = 3

// ErrUpstreamBusy means OpenRouter rate limited or was overloaded; retrying later can work.
var ErrUpstreamBusy = errors.New("openrouter is busy")

const defaultOpenRouterAPIURL = "https://openrouter.ai/api/v1/chat/completions"

func openRouterAPIKey() string {
	return utils.Getenv("OPENROUTER_API_KEY")
}

func openRouterAPIURL() string {
	if url := utils.Getenv("OPENROUTER_API_URL"); url != "" {
		return url
	}

	return defaultOpenRouterAPIURL
}

func openRouterModels() []string {
	var models []string

	for model := range strings.SplitSeq(utils.Getenv("OPENROUTER_MODEL"), ",") {
		if model = strings.TrimSpace(model); model != "" {
			models = append(models, model)
		}
	}

	if len(models) == 0 {
		return defaultOpenRouterModels
	}

	return models[:min(len(models), maxOpenRouterModels)]
}

type openRouterChatRequest struct {
	Model          string                   `json:"model,omitempty"`
	Models         []string                 `json:"models,omitempty"`
	Messages       []openRouterChatMessage  `json:"messages"`
	ResponseFormat openRouterResponseFormat `json:"response_format"`
	Temperature    float64                  `json:"temperature"`
	Reasoning      openRouterReasoning      `json:"reasoning"`
}

// openRouterReasoning is off: thinking tokens tripled latency and pushed resume extraction past its timeout.
type openRouterReasoning struct {
	Enabled bool `json:"enabled"`
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
// chat completions endpoint. response_format only guarantees valid JSON, not a
// specific shape, so the schema gets rendered into the prompt too, the same trick
// the old Groq client used.
func callOpenRouterChat(ctx context.Context, prompt string, schema map[string]any, temperature float64) (string, error) {
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
		Messages: []openRouterChatMessage{
			{Role: "user", Content: fullPrompt},
		},
		ResponseFormat: openRouterResponseFormat{Type: "json_object"},
		Temperature:    temperature,
	}

	// A single model goes in model; several go in models so OpenRouter falls back on failure.
	if models := openRouterModels(); len(models) == 1 {
		reqBody.Model = models[0]
	} else {
		reqBody.Models = models
	}

	status, respBody, err := postJSON(ctx, openRouterAPIURL(), apiKey, reqBody)
	if err != nil {
		return "", fmt.Errorf("call openrouter: %w", err)
	}

	if status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable {
		return "", fmt.Errorf("%w: openrouter returned %d: %s", ErrUpstreamBusy, status, errorBody(respBody))
	}

	if status != http.StatusOK {
		return "", fmt.Errorf("openrouter returned %d: %s", status, errorBody(respBody))
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
