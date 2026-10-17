package notify

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type sentEmail struct {
	header http.Header
	method string
	body   resendEmailRequest
}

// fakeResend points SendEmail at a test server answering with status.
func fakeResend(t *testing.T, status int) *[]sentEmail {
	t.Helper()

	var sent []sentEmail

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)

		var body resendEmailRequest
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("decode request: %v", err)
		}

		sent = append(sent, sentEmail{header: r.Header.Clone(), method: r.Method, body: body})

		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"message":"nope"}`))
	}))
	t.Cleanup(srv.Close)

	t.Setenv("RESEND_API_URL", srv.URL)

	return &sent
}

func TestSendEmailNoKeyIsNoop(t *testing.T) {
	sent := fakeResend(t, http.StatusOK)
	t.Setenv("RESEND_API_KEY", "")

	if err := SendEmail(t.Context(), "a@example.com", "s", "b"); err != nil {
		t.Fatalf("SendEmail() = %v, want nil", err)
	}

	if len(*sent) != 0 {
		t.Errorf("sent %d requests, want 0", len(*sent))
	}
}

func TestSendEmailPostsPayload(t *testing.T) {
	sent := fakeResend(t, http.StatusOK)
	t.Setenv("RESEND_API_KEY", "test-key")
	t.Setenv("RESEND_FROM_ADDRESS", "")

	if err := SendEmail(t.Context(), "to@example.com", "Hello", "Body text"); err != nil {
		t.Fatalf("SendEmail() = %v", err)
	}

	if len(*sent) != 1 {
		t.Fatalf("sent %d requests, want 1", len(*sent))
	}

	got := (*sent)[0]
	if got.method != http.MethodPost {
		t.Errorf("method = %s, want POST", got.method)
	}
	if h := got.header.Get("Authorization"); h != "Bearer test-key" {
		t.Errorf("Authorization = %q", h)
	}
	if h := got.header.Get("Content-Type"); h != "application/json" {
		t.Errorf("Content-Type = %q", h)
	}

	b := got.body
	if b.From != defaultFromAddress || len(b.To) != 1 || b.To[0] != "to@example.com" || b.Subject != "Hello" || b.Text != "Body text" {
		t.Errorf("body = %+v", b)
	}
}

func TestSendEmailCustomFrom(t *testing.T) {
	sent := fakeResend(t, http.StatusOK)
	t.Setenv("RESEND_API_KEY", "test-key")
	t.Setenv("RESEND_FROM_ADDRESS", "Jobs <jobs@example.com>")

	if err := SendEmail(t.Context(), "to@example.com", "s", "b"); err != nil {
		t.Fatalf("SendEmail() = %v", err)
	}

	if got := (*sent)[0].body.From; got != "Jobs <jobs@example.com>" {
		t.Errorf("From = %q", got)
	}
}

func TestResendAPIURLDefault(t *testing.T) {
	t.Setenv("RESEND_API_URL", "")

	if got := resendAPIURL(); got != defaultResendAPIURL {
		t.Errorf("resendAPIURL() = %q, want %q", got, defaultResendAPIURL)
	}
}

func TestSendEmailErrorStatus(t *testing.T) {
	fakeResend(t, http.StatusUnprocessableEntity)
	t.Setenv("RESEND_API_KEY", "test-key")

	err := SendEmail(t.Context(), "to@example.com", "s", "b")
	if err == nil {
		t.Fatal("SendEmail() = nil, want error")
	}

	if !strings.Contains(err.Error(), "422") || !strings.Contains(err.Error(), "nope") {
		t.Errorf("error %q should include status and body", err)
	}
}

func TestSendEmailUnreachable(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "test-key")

	t.Setenv("RESEND_API_URL", "http://127.0.0.1:1")

	if err := SendEmail(t.Context(), "to@example.com", "s", "b"); err == nil {
		t.Fatal("SendEmail() = nil, want error")
	}
}
