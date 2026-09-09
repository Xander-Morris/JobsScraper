package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"main/utils"
)

const resendAPIURL = "https://api.resend.com/emails"

const defaultFromAddress = "onboarding@resend.dev"

type resendEmailRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
}

// SendEmail sends a plain-text email via the Resend API. If RESEND_API_KEY isn't
// configured, it logs and no-ops rather than failing. Lets the rest of the
// notification pipeline (matching, digest bookkeeping) run and be tested before a
// real sending credential is wired up.
func SendEmail(ctx context.Context, to, subject, body string) error {
	env := utils.GetEnv()
	apiKey := env["RESEND_API_KEY"]

	if apiKey == "" {
		slog.Warn("notify: RESEND_API_KEY not set, skipping email", "to", to, "subject", subject)
		return nil
	}

	from := env["RESEND_FROM_ADDRESS"]
	if from == "" {
		from = defaultFromAddress
	}

	payload, err := json.Marshal(resendEmailRequest{From: from, To: []string{to}, Subject: subject, Text: body})

	if err != nil {
		return fmt.Errorf("encode resend request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendAPIURL, bytes.NewReader(payload))

	if err != nil {
		return fmt.Errorf("build resend request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return fmt.Errorf("send resend request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("resend request failed: %s: %s", resp.Status, respBody)
	}

	return nil
}
