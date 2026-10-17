package server

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"golang.org/x/time/rate"

	"main/database"
)

// api drives the full Handler stack (routing, middleware, auth) over real HTTP.
type api struct {
	t      *testing.T
	srv    *httptest.Server
	client *http.Client
	token  string
}

// allowAll is a limiter that never limits, so tests can make many calls.
type allowAll struct{}

func (allowAll) allow(string) bool { return true }

// newAPI starts the server with limits off and every outside service unset.
func newAPI(t *testing.T) *api {
	t.Helper()
	requireTestDB(t)

	t.Setenv("SECRET_KEY", "api-test-secret")
	t.Setenv("ALLOWED_ORIGIN", "")
	t.Setenv("COOKIE_SECURE", "")
	t.Setenv("COOKIE_SAME_SITE", "")
	t.Setenv("JINA_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("RESEND_API_KEY", "")

	prevGlobal, prevAuth, prevLLM := globalLimiter, authLimiter, llmLimiter
	globalLimiter = newIPLimiter(rate.Inf, 1)
	authLimiter, llmLimiter = allowAll{}, allowAll{}
	t.Cleanup(func() { globalLimiter, authLimiter, llmLimiter = prevGlobal, prevAuth, prevLLM })

	srv := httptest.NewServer(Handler())
	t.Cleanup(srv.Close)

	return &api{t: t, srv: srv, client: newClient(t)}
}

func newClient(t *testing.T) *http.Client {
	t.Helper()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}

	return &http.Client{Jar: jar}
}

// as returns a second client on the same server, with its own cookies and token.
func (a *api) as() *api {
	return &api{t: a.t, srv: a.srv, client: newClient(a.t)}
}

func (a *api) send(req *http.Request) (int, []byte, http.Header) {
	a.t.Helper()

	if a.token != "" && req.Header.Get("Authorization") == "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		a.t.Fatalf("%s %s: %v", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	return resp.StatusCode, body, resp.Header
}

// do sends body as JSON (a string is sent raw) and returns status and body.
func (a *api) do(method, path string, body any) (int, []byte) {
	a.t.Helper()

	var reader io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		reader = strings.NewReader(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			a.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, a.srv.URL+path, reader)
	if err != nil {
		a.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	status, raw, _ := a.send(req)

	return status, raw
}

// must calls do and fails unless the status matches, decoding into out when given.
func (a *api) must(want int, method, path string, body any, out any) []byte {
	a.t.Helper()

	status, raw := a.do(method, path, body)
	if status != want {
		a.t.Fatalf("%s %s = %d, want %d: %s", method, path, status, want, raw)
	}

	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			a.t.Fatalf("decode %s %s: %v: %s", method, path, err, raw)
		}
	}

	return raw
}

// upload sends a multipart form with an optional resume file.
func (a *api) upload(method, path, fileName string, content []byte, fields map[string]string) (int, []byte) {
	a.t.Helper()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}

	if fileName != "" {
		fw, err := mw.CreateFormFile("resume", fileName)
		if err != nil {
			a.t.Fatal(err)
		}
		_, _ = fw.Write(content)
	}
	_ = mw.Close()

	req, err := http.NewRequest(method, a.srv.URL+path, &buf)
	if err != nil {
		a.t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	status, raw, _ := a.send(req)

	return status, raw
}

// signup creates a profile and keeps its access token.
func (a *api) signup(email string) *api {
	a.t.Helper()

	var resp struct {
		Token string `json:"token"`
	}
	a.must(http.StatusCreated, "POST", "/api/profile/create",
		map[string]string{"email": email, "password": "securepassword123"}, &resp)

	if resp.Token == "" {
		a.t.Fatal("signup returned no token")
	}
	a.token = resp.Token

	return a
}

func (a *api) profile() database.Profile {
	a.t.Helper()

	var p database.Profile
	a.must(http.StatusOK, "GET", "/api/profile", nil, &p)

	return p
}

func (a *api) refreshCookie() *http.Cookie {
	a.t.Helper()

	req, _ := http.NewRequest("GET", a.srv.URL+"/api/profile", nil)
	for _, c := range a.client.Jar.Cookies(req.URL) {
		if c.Name == refreshCookieName {
			return c
		}
	}

	return nil
}

// fakeLLM points OpenRouter at a server answering with content, and Jina at one returning unit vectors.
func fakeLLM(t *testing.T, content func() string) *fakeLLMCalls {
	t.Helper()

	calls := &fakeLLMCalls{}

	chat := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if len(req.Messages) > 0 {
			calls.setPrompt(req.Messages[0].Content)
		}

		calls.add("chat")
		c := content()
		if c == "" {
			http.Error(w, "upstream down", http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": c}}},
		})
	}))
	t.Cleanup(chat.Close)

	embed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.add("embed")
		vec := make([]float32, 768)
		vec[0] = 1
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"index": 0, "embedding": vec}}})
	}))
	t.Cleanup(embed.Close)

	t.Setenv("OPENROUTER_API_URL", chat.URL)
	t.Setenv("OPENROUTER_API_KEY", "or-key")
	t.Setenv("JINA_API_URL", embed.URL)
	t.Setenv("JINA_API_KEY", "jina-key")

	return calls
}

type fakeLLMCalls struct {
	mu         sync.Mutex
	calls      map[string]int
	lastPrompt string
}

func (c *fakeLLMCalls) setPrompt(p string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastPrompt = p
}

func (c *fakeLLMCalls) prompt() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.lastPrompt
}

func (c *fakeLLMCalls) add(kind string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.calls == nil {
		c.calls = map[string]int{}
	}
	c.calls[kind]++
}

func (c *fakeLLMCalls) count(kind string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.calls[kind]
}

func static(s string) func() string { return func() string { return s } }
