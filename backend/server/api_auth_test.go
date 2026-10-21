package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"main/database"
)

func TestAPISignupLoginAndProfile(t *testing.T) {
	a := newAPI(t)

	a.signup("api-auth@example.com")

	if c := a.refreshCookie(); c == nil || c.Value == "" {
		t.Fatal("signup set no refresh cookie")
	}

	if p := a.profile(); p.Email != "api-auth@example.com" {
		t.Errorf("profile email = %q", p.Email)
	}

	anon := a.as()
	creds := func(email, password string) map[string]any {
		return map[string]any{"email": email, "password": password}
	}

	// Duplicate and invalid signups are client errors.
	anon.must(http.StatusBadRequest, "POST", "/api/profile/create", creds("api-auth@example.com", "securepassword123"), nil)
	anon.must(http.StatusBadRequest, "POST", "/api/profile/create", creds("api-short@example.com", "short"), nil)
	anon.must(http.StatusBadRequest, "POST", "/api/profile/create", "{not json", nil)

	// Wrong password and unknown email get the same answer.
	_, wrongPass := anon.do("POST", "/api/profile/login", creds("api-auth@example.com", "wrongpassword"))
	status, unknown := anon.do("POST", "/api/profile/login", creds("nobody@example.com", "securepassword123"))
	if status != http.StatusUnauthorized || string(wrongPass) != string(unknown) {
		t.Errorf("login failures should match: %d %s vs %s", status, wrongPass, unknown)
	}
	anon.must(http.StatusBadRequest, "POST", "/api/profile/login", "[]", nil)

	var login struct {
		Token string `json:"token"`
	}
	anon.must(http.StatusAccepted, "POST", "/api/profile/login", creds("api-auth@example.com", "securepassword123"), &login)

	anon.token = login.Token
	if p := anon.profile(); p.Email != "api-auth@example.com" {
		t.Errorf("login token profile = %+v", p)
	}
}

func TestAPIRejectsMissingAndBadTokens(t *testing.T) {
	a := newAPI(t)

	for _, auth := range []string{"", "Bearer ", "Bearer not-a-jwt", "Basic abc"} {
		req, _ := http.NewRequest("GET", a.srv.URL+"/api/profile", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}

		if status, _, _ := a.send(req); status != http.StatusUnauthorized {
			t.Errorf("Authorization %q = %d, want 401", auth, status)
		}
	}

	// A token signed with another key is rejected.
	t.Setenv("SECRET_KEY", "other-secret")
	forged := createToken(1)
	t.Setenv("SECRET_KEY", "api-test-secret")

	req, _ := http.NewRequest("GET", a.srv.URL+"/api/profile", nil)
	req.Header.Set("Authorization", "Bearer "+forged)
	if status, _, _ := a.send(req); status != http.StatusUnauthorized {
		t.Errorf("forged token = %d, want 401", status)
	}
}

func TestAPIRefreshRotatesCookie(t *testing.T) {
	a := newAPI(t).signup("api-refresh@example.com")

	first := a.refreshCookie()

	var resp struct {
		Token string `json:"token"`
	}
	a.must(http.StatusOK, "POST", "/api/profile/refresh", nil, &resp)

	if resp.Token == "" {
		t.Fatal("refresh returned no token")
	}

	second := a.refreshCookie()
	if second == nil || second.Value == first.Value {
		t.Fatal("refresh did not rotate the cookie")
	}

	// Replaying the old cookie fails.
	replay := a.as()
	u, _ := url.Parse(a.srv.URL + "/api/profile")
	replay.client.Jar.SetCookies(u, []*http.Cookie{{Name: refreshCookieName, Value: first.Value, Path: "/api/profile"}})
	replay.must(http.StatusUnauthorized, "POST", "/api/profile/refresh", nil, nil)

	// No cookie at all fails.
	a.as().must(http.StatusUnauthorized, "POST", "/api/profile/refresh", nil, nil)

	a.token = resp.Token
	a.profile()
}

func TestAPILogoutRevokesRefresh(t *testing.T) {
	a := newAPI(t).signup("api-logout@example.com")

	cookie := a.refreshCookie()

	if status, _ := a.do("POST", "/api/profile/logout", nil); status != http.StatusNoContent {
		t.Fatalf("logout = %d", status)
	}

	if c := a.refreshCookie(); c != nil {
		t.Errorf("logout left cookie %q", c.Value)
	}

	replay := a.as()
	u, _ := url.Parse(a.srv.URL + "/api/profile")
	replay.client.Jar.SetCookies(u, []*http.Cookie{{Name: refreshCookieName, Value: cookie.Value, Path: "/api/profile"}})
	replay.must(http.StatusUnauthorized, "POST", "/api/profile/refresh", nil, nil)

	// Logging out without a session is fine.
	if status, _ := a.as().do("POST", "/api/profile/logout", nil); status != http.StatusNoContent {
		t.Errorf("anonymous logout = %d", status)
	}
}

func TestAPIRefreshForDeletedProfile(t *testing.T) {
	a := newAPI(t).signup("api-deleted@example.com")
	id := a.profile().ID

	if _, err := database.DB().Exec("DELETE FROM profiles WHERE id = $1", id); err != nil {
		t.Fatal(err)
	}

	a.must(http.StatusUnauthorized, "POST", "/api/profile/refresh", nil, nil)
}

func TestRefreshCookieAttributes(t *testing.T) {
	t.Setenv("COOKIE_SECURE", "")
	t.Setenv("COOKIE_SAME_SITE", "")

	c := refreshCookie("v")
	if c.Name != refreshCookieName || c.Path != "/api/profile" || !c.HttpOnly || c.Secure || c.SameSite != http.SameSiteLaxMode {
		t.Errorf("default cookie = %+v", c)
	}

	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("COOKIE_SAME_SITE", "None")

	c = refreshCookie("v")
	if !c.Secure || c.SameSite != http.SameSiteNoneMode {
		t.Errorf("cross-site cookie = %+v", c)
	}
}

func TestSetRefreshCookiePersistence(t *testing.T) {
	expires := time.Now().Add(time.Hour)

	rec := httptest.NewRecorder()
	setRefreshCookie(rec, "tok", expires, true)
	if h := rec.Header().Get("Set-Cookie"); !strings.Contains(h, "Max-Age=") || !strings.Contains(h, "Expires=") {
		t.Errorf("persistent cookie = %q", h)
	}

	rec = httptest.NewRecorder()
	setRefreshCookie(rec, "tok", expires, false)
	if h := rec.Header().Get("Set-Cookie"); strings.Contains(h, "Max-Age=") || strings.Contains(h, "Expires=") {
		t.Errorf("session cookie should have no expiry: %q", h)
	}

	rec = httptest.NewRecorder()
	clearRefreshCookie(rec)
	if h := rec.Header().Get("Set-Cookie"); !strings.Contains(h, "Max-Age=0") {
		t.Errorf("cleared cookie = %q", h)
	}
}

func TestCreateTokenRoundTrip(t *testing.T) {
	t.Setenv("SECRET_KEY", "round-trip")

	token := createToken(77)

	if id, ok := parseProfileIDFromToken(token); !ok || id != 77 {
		t.Errorf("parse = %d, %v", id, ok)
	}

	if _, ok := parseProfileIDFromToken(token + "x"); ok {
		t.Error("tampered token accepted")
	}
}

type sentResetEmail struct {
	To   []string `json:"to"`
	Text string   `json:"text"`
}

func TestAPIPasswordResetEmail(t *testing.T) {
	a := newAPI(t).signup("api-reset@example.com")

	var (
		mu   sync.Mutex
		sent []sentResetEmail
	)
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var e sentResetEmail
		_ = json.NewDecoder(r.Body).Decode(&e)
		mu.Lock()
		sent = append(sent, e)
		mu.Unlock()
	}))
	t.Cleanup(resend.Close)
	t.Setenv("RESEND_API_URL", resend.URL)
	t.Setenv("RESEND_API_KEY", "test-key")
	t.Setenv("ALLOWED_ORIGIN", "https://app.example.com")

	anon := a.as()
	anon.must(http.StatusAccepted, "POST", "/api/profile/password-reset", map[string]string{"email": "api-reset@example.com"}, nil)
	anon.must(http.StatusAccepted, "POST", "/api/profile/password-reset", map[string]string{"email": "ghost@example.com"}, nil)
	anon.must(http.StatusBadRequest, "POST", "/api/profile/password-reset", map[string]string{}, nil)

	mu.Lock()
	defer mu.Unlock()

	if len(sent) != 1 || sent[0].To[0] != "api-reset@example.com" {
		t.Fatalf("sent = %+v, want one email to the real account", sent)
	}

	_, after, ok := strings.Cut(sent[0].Text, "https://app.example.com/reset-password?token=")
	if !ok {
		t.Fatalf("email lacks reset link: %q", sent[0].Text)
	}
	token, _ := url.QueryUnescape(strings.Fields(after)[0])

	anon.must(http.StatusOK, "POST", "/api/profile/password-reset/confirm", map[string]string{"token": token, "password": "brandnewpass123"}, nil)
	anon.must(http.StatusAccepted, "POST", "/api/profile/login", map[string]any{"email": "api-reset@example.com", "password": "brandnewpass123"}, nil)
}

func TestRenderPasswordResetEmail(t *testing.T) {
	t.Setenv("ALLOWED_ORIGIN", "https://app.example.com/")

	body := renderPasswordResetEmail("a b+c")

	if !strings.Contains(body, "https://app.example.com/reset-password?token=a+b%2Bc") {
		t.Errorf("link not escaped or origin slash kept: %q", body)
	}

	if !strings.Contains(body, strconv.Itoa(int(database.PasswordResetTokenLifetime.Minutes()))+" minutes") {
		t.Errorf("body lacks expiry: %q", body)
	}
}
