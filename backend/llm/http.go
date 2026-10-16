package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// maxResponseSize caps a provider response; a full embedding batch is a few MiB.
const maxResponseSize = 32 << 20

// maxErrorBodyLen keeps a provider's error page from flooding the logs.
const maxErrorBodyLen = 512

// postJSON POSTs body as JSON with a bearer token, returning the status and raw response body.
func postJSON(ctx context.Context, url, apiKey string, body any) (int, []byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return 0, nil, fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0, nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return 0, nil, fmt.Errorf("read response: %w", err)
	}

	return resp.StatusCode, respBody, nil
}

func errorBody(body []byte) string {
	if len(body) > maxErrorBodyLen {
		return string(body[:maxErrorBodyLen]) + "…"
	}

	return string(body)
}
