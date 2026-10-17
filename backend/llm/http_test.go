package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeOpenRouter answers each chat call with content wrapped in a completion.
func fakeOpenRouter(t *testing.T, content string) *[]openRouterChatRequest {
	t.Helper()

	return fakeOpenRouterRaw(t, http.StatusOK, func() string {
		raw, _ := json.Marshal(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": content}}},
		})
		return string(raw)
	})
}

func fakeOpenRouterRaw(t *testing.T, status int, body func() string) *[]openRouterChatRequest {
	t.Helper()

	var reqs []openRouterChatRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openRouterChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode chat request: %v", err)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer or-key" {
			t.Errorf("Authorization = %q", got)
		}
		reqs = append(reqs, req)

		w.WriteHeader(status)
		_, _ = io.WriteString(w, body())
	}))
	t.Cleanup(srv.Close)

	t.Setenv("OPENROUTER_API_URL", srv.URL)
	t.Setenv("OPENROUTER_API_KEY", "or-key")
	t.Setenv("OPENROUTER_MODEL", "")

	return &reqs
}

func TestPostJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("bad request: %s %v", r.Method, r.Header)
		}

		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"a":1}` {
			t.Errorf("body = %s", body)
		}

		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("short"))
	}))
	defer srv.Close()

	status, body, err := postJSON(t.Context(), srv.URL, "k", map[string]int{"a": 1})
	if err != nil || status != http.StatusTeapot || string(body) != "short" {
		t.Errorf("postJSON = %d, %q, %v", status, body, err)
	}
}

func TestPostJSONErrors(t *testing.T) {
	if _, _, err := postJSON(t.Context(), "http://x", "k", make(chan int)); err == nil {
		t.Error("unencodable body: want error")
	}

	if _, _, err := postJSON(t.Context(), "://bad", "k", nil); err == nil {
		t.Error("bad url: want error")
	}

	if _, _, err := postJSON(t.Context(), "http://127.0.0.1:1", "k", nil); err == nil {
		t.Error("unreachable: want error")
	}
}

func TestErrorBodyTruncates(t *testing.T) {
	if got := errorBody([]byte("tiny")); got != "tiny" {
		t.Errorf("errorBody(tiny) = %q", got)
	}

	long := strings.Repeat("x", maxErrorBodyLen+100)
	got := errorBody([]byte(long))

	if !strings.HasSuffix(got, "…") || len(got) != maxErrorBodyLen+len("…") {
		t.Errorf("errorBody(long) len = %d", len(got))
	}
}

func TestEnvDefaults(t *testing.T) {
	t.Setenv("OPENROUTER_API_URL", "")
	t.Setenv("OPENROUTER_MODEL", "")
	t.Setenv("JINA_API_URL", "")
	t.Setenv("JINA_EMBED_MODEL", "")

	if openRouterAPIURL() != defaultOpenRouterAPIURL || openRouterModel() != defaultOpenRouterModel {
		t.Error("openrouter defaults not used")
	}
	if jinaAPIURL() != defaultJinaAPIURL || jinaEmbedModel() != defaultJinaEmbedModel {
		t.Error("jina defaults not used")
	}

	t.Setenv("OPENROUTER_MODEL", "m")
	t.Setenv("JINA_EMBED_MODEL", "e")

	if openRouterModel() != "m" || jinaEmbedModel() != "e" {
		t.Error("model overrides ignored")
	}
}

func TestCallOpenRouterChat(t *testing.T) {
	reqs := fakeOpenRouter(t, `{"ok":true}`)
	t.Setenv("OPENROUTER_MODEL", "test/model")

	got, err := callOpenRouterChat(t.Context(), "Say hi", map[string]any{"type": "object"}, 0.7)
	if err != nil {
		t.Fatal(err)
	}

	if got != `{"ok":true}` {
		t.Errorf("content = %q", got)
	}

	req := (*reqs)[0]
	if req.Model != "test/model" || req.Temperature != 0.7 || req.ResponseFormat.Type != "json_object" || req.Reasoning.Enabled {
		t.Errorf("request = %+v", req)
	}

	if len(req.Messages) != 1 || !strings.HasPrefix(req.Messages[0].Content, "Say hi") ||
		!strings.Contains(req.Messages[0].Content, `{"type":"object"}`) {
		t.Errorf("prompt should carry the schema: %q", req.Messages[0].Content)
	}
}

func TestCallOpenRouterChatFailures(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"non-200", http.StatusBadGateway, "upstream down", "502"},
		{"bad json", http.StatusOK, "not json", "decode"},
		{"error field", http.StatusOK, `{"error":{"message":"quota"}}`, "quota"},
		{"no choices", http.StatusOK, `{"choices":[]}`, "no choices"},
		{"blank content", http.StatusOK, `{"choices":[{"message":{"content":"   "}}]}`, "empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeOpenRouterRaw(t, tt.status, func() string { return tt.body })

			_, err := callOpenRouterChat(t.Context(), "p", nil, 0)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestCallOpenRouterChatNoKey(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")

	if _, err := callOpenRouterChat(t.Context(), "p", nil, 0); err == nil || !strings.Contains(err.Error(), "OPENROUTER_API_KEY") {
		t.Errorf("err = %v", err)
	}
}

// fakeJinaServer serves embeddings via respond and counts calls.
func fakeJinaServer(t *testing.T, respond func(call int32, input []string) (int, any)) *atomic.Int32 {
	t.Helper()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jinaEmbedRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		status, body := respond(calls.Add(1), req.Input)
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)

	t.Setenv("JINA_API_URL", srv.URL)
	t.Setenv("JINA_API_KEY", "jina-key")

	prev := jinaRateLimitWait
	jinaRateLimitWait = time.Millisecond
	t.Cleanup(func() { jinaRateLimitWait = prev })

	return &calls
}

type embedDatum struct {
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

func TestEmbedTextsReordersByIndex(t *testing.T) {
	fakeJinaServer(t, func(_ int32, input []string) (int, any) {
		// Reverse order on the wire.
		var data []embedDatum
		for i := len(input) - 1; i >= 0; i-- {
			data = append(data, embedDatum{Index: i, Embedding: []float32{float32(i)}})
		}
		return http.StatusOK, map[string]any{"data": data}
	})

	got, err := EmbedTexts(t.Context(), []string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}

	for i, v := range got {
		if len(v) != 1 || v[0] != float32(i) {
			t.Errorf("vector %d = %v", i, v)
		}
	}
}

func TestEmbedTextsEmptySkipsCall(t *testing.T) {
	calls := fakeJinaServer(t, func(int32, []string) (int, any) { return http.StatusOK, nil })

	got, err := EmbedTexts(t.Context(), nil)
	if got != nil || err != nil || calls.Load() != 0 {
		t.Errorf("EmbedTexts(nil) = %v, %v; calls %d", got, err, calls.Load())
	}
}

func TestEmbedTextsRetriesRateLimit(t *testing.T) {
	calls := fakeJinaServer(t, func(call int32, input []string) (int, any) {
		if call == 1 {
			return http.StatusTooManyRequests, map[string]string{"detail": "slow down"}
		}
		return http.StatusOK, map[string]any{"data": []embedDatum{{Index: 0, Embedding: []float32{1}}}}
	})

	if _, err := EmbedTexts(t.Context(), []string{"a"}); err != nil {
		t.Fatalf("EmbedTexts: %v", err)
	}

	if n := calls.Load(); n != 2 {
		t.Errorf("calls = %d, want 2", n)
	}
}

func TestEmbedTextsGivesUpAfterMaxAttempts(t *testing.T) {
	calls := fakeJinaServer(t, func(int32, []string) (int, any) {
		return http.StatusTooManyRequests, "slow down"
	})

	_, err := EmbedTexts(t.Context(), []string{"a"})
	if !errors.Is(err, errJinaRateLimited) {
		t.Errorf("err = %v, want errJinaRateLimited", err)
	}

	if n := calls.Load(); n != jinaMaxEmbedAttempts {
		t.Errorf("calls = %d, want %d", n, jinaMaxEmbedAttempts)
	}
}

func TestEmbedTextsCancelledDuringBackoff(t *testing.T) {
	fakeJinaServer(t, func(int32, []string) (int, any) { return http.StatusTooManyRequests, "" })
	jinaRateLimitWait = time.Hour

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	if _, err := EmbedTexts(ctx, []string{"a"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want deadline exceeded", err)
	}
}

func TestCallJinaEmbedFailures(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   any
		want   string
	}{
		{"server error", http.StatusInternalServerError, "boom", "500"},
		{"bad json", http.StatusOK, "not an object", "decode"},
		{"count mismatch", http.StatusOK, map[string]any{"data": []embedDatum{}}, "0 embeddings for 1"},
		{"bad index", http.StatusOK, map[string]any{"data": []embedDatum{{Index: 5}}}, "out-of-range"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeJinaServer(t, func(int32, []string) (int, any) { return tt.status, tt.body })

			_, err := callJinaEmbed(t.Context(), []string{"a"})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestCallJinaEmbedNoKey(t *testing.T) {
	t.Setenv("JINA_API_KEY", "")

	if _, err := callJinaEmbed(t.Context(), []string{"a"}); err == nil || !strings.Contains(err.Error(), "JINA_API_KEY") {
		t.Errorf("err = %v", err)
	}
}

func TestGenerateApplicationContent(t *testing.T) {
	reqs := fakeOpenRouter(t, `{"cover_letter":"Dear Acme","tailored_bullets":["Built APIs"]}`)

	got, err := GenerateApplicationContent(t.Context(),
		ResumeProfile{FullName: "Jane", Skills: []string{"Go"}},
		JobPosting{Title: "Backend Engineer", Company: "Acme"})
	if err != nil {
		t.Fatal(err)
	}

	if got.CoverLetter != "Dear Acme" || len(got.TailoredBullets) != 1 {
		t.Errorf("content = %+v", got)
	}

	prompt := (*reqs)[0].Messages[0].Content
	if !strings.Contains(prompt, "Jane") || !strings.Contains(prompt, "Backend Engineer") {
		t.Errorf("prompt missing resume or job: %q", prompt)
	}
}

func TestGenerateApplicationContentBadJSON(t *testing.T) {
	fakeOpenRouter(t, `{"cover_letter":`)

	if _, err := GenerateApplicationContent(t.Context(), ResumeProfile{}, JobPosting{}); err == nil {
		t.Error("want decode error")
	}
}

func TestGenerateApplicationContentUpstreamError(t *testing.T) {
	fakeOpenRouterRaw(t, http.StatusInternalServerError, func() string { return "down" })

	if _, err := GenerateApplicationContent(t.Context(), ResumeProfile{}, JobPosting{}); err == nil {
		t.Error("want upstream error")
	}
}

func TestEmbeddingTextMatchesPromptFormat(t *testing.T) {
	p := ResumeProfile{FullName: "Jane", Skills: []string{"Go"}}

	if EmbeddingText(p) != formatResumeProfile(p) {
		t.Error("EmbeddingText should reuse formatResumeProfile")
	}
}

func TestTailorResume(t *testing.T) {
	fakeOpenRouter(t, `{"summary":"Go engineer","skills":["Go"],
		"work_experience":[{"source":"W0","bullets":[{"text":"Built Go APIs","source":"W0.B0"}]}],
		"projects":[]}`)

	got, err := TailorResume(t.Context(), testExtractedResume(), JobPosting{Title: "Go Dev"})
	if err != nil {
		t.Fatal(err)
	}

	if got.FullName != "Jane Doe" || got.Summary != "Go engineer" {
		t.Errorf("tailored = %+v", got)
	}

	if len(got.WorkExperience) == 0 || got.WorkExperience[0].Company != "Acme" {
		t.Errorf("work experience should be hydrated from the resume: %+v", got.WorkExperience)
	}
}

func TestTailorResumeBadJSON(t *testing.T) {
	fakeOpenRouter(t, `[]`)

	if _, err := TailorResume(t.Context(), testExtractedResume(), JobPosting{}); err == nil {
		t.Error("want decode error")
	}
}

func TestGenerateTrivia(t *testing.T) {
	q := validQuestion("What is a goroutine?")
	bad := TriviaQuestion{Question: "Bad", Options: []string{"only one"}}
	raw, _ := json.Marshal(triviaResponse{Questions: []TriviaQuestion{q, bad, validQuestion("Second?"), validQuestion("Third?")}})

	reqs := fakeOpenRouter(t, string(raw))

	got, err := GenerateTrivia(t.Context(), TriviaRequest{Prompt: "Go", Count: 2, Difficulty: "easy"})
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 2 || got[0].Question != "What is a goroutine?" || got[1].Question != "Second?" {
		t.Fatalf("questions = %+v", got)
	}

	for _, question := range got {
		if question.Options[question.CorrectIndex] != "C" {
			t.Errorf("shuffle lost the correct answer: %+v", question)
		}
	}

	if (*reqs)[0].Temperature != triviaTemperature {
		t.Errorf("temperature = %v, want %v", (*reqs)[0].Temperature, triviaTemperature)
	}
}

func TestGenerateTriviaNoValidQuestions(t *testing.T) {
	fakeOpenRouter(t, `{"questions":[{"question":"","options":[]}]}`)

	if _, err := GenerateTrivia(t.Context(), TriviaRequest{Prompt: "Go", Count: 3}); err == nil {
		t.Error("want error when every question is malformed")
	}
}

func TestGenerateTriviaBadJSON(t *testing.T) {
	fakeOpenRouter(t, `nope`)

	if _, err := GenerateTrivia(t.Context(), TriviaRequest{Prompt: "Go", Count: 3}); err == nil {
		t.Error("want decode error")
	}
}
