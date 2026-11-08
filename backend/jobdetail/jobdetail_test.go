package jobdetail

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// useServer serves handler and lets the client reach loopback, which production refuses.
func useServer(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	prev := httpClient
	httpClient = srv.Client()
	t.Cleanup(func() { httpClient = prev })

	return srv.URL
}

func TestFetchFromJSONLD(t *testing.T) {
	base := useServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/old":
			http.Redirect(w, r, "/job", http.StatusFound)
		case "/job":
			w.Write([]byte(`<html><script type="application/ld+json">
				{"@context":"https://schema.org","@graph":[{"@type":"JobPosting","title":"Go Dev",
				"description":"&lt;p&gt;Build &lt;b&gt;APIs&lt;/b&gt;&lt;/p&gt;"}]}</script></html>`))
		case "/plain":
			w.Write([]byte(`<html><body>No structured data</body></html>`))
		case "/blocked":
			http.Error(w, "bots not welcome", http.StatusForbidden)
		default:
			http.NotFound(w, r)
		}
	})

	got, err := Fetch(t.Context(), base+"/old", "")
	if err != nil || got != "Build APIs" {
		t.Errorf("Fetch = %q, %v; want Build APIs", got, err)
	}

	for _, path := range []string{"/plain", "/missing", "/blocked"} {
		if _, err := Fetch(t.Context(), base+path, ""); !errors.Is(err, ErrUnavailable) {
			t.Errorf("%s: err = %v, want ErrUnavailable", path, err)
		}
	}
}

func TestFetchFromDetailAPI(t *testing.T) {
	base := useServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/boards/acme/jobs/1":
			w.Write([]byte(`{"id":1,"content":"&lt;ul&gt;&lt;li&gt;Write Go&lt;/li&gt;&lt;/ul&gt;"}`))
		case "/boards/acme/jobs/2":
			w.Write([]byte(`{"id":2,"content":""}`))
		case "/boards/acme/jobs/3":
			w.Write([]byte(`<html>not json</html>`))
		default:
			http.Error(w, "boom", http.StatusInternalServerError)
		}
	})

	got, err := Fetch(t.Context(), "https://unused.example", base+"/boards/acme/jobs/1")
	if err != nil || got != "• Write Go" {
		t.Errorf("Fetch = %q, %v; want bullet", got, err)
	}

	if _, err := Fetch(t.Context(), "", base+"/boards/acme/jobs/2"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("empty content: err = %v, want ErrUnavailable", err)
	}

	if _, err := Fetch(t.Context(), "", base+"/boards/acme/jobs/3"); err == nil || errors.Is(err, ErrUnavailable) {
		t.Errorf("bad JSON: err = %v, want decode error", err)
	}

	// A server error is transient, not "unavailable", so it isn't cached as missing.
	if _, err := Fetch(t.Context(), "", base+"/down"); err == nil || errors.Is(err, ErrUnavailable) {
		t.Errorf("server error: err = %v, want transient error", err)
	}
}

func TestFetchFromHimalayasAPI(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "himalayas.app" || r.URL.Path != "/jobs/api/companies/acme/jobs/go-dev" {
			http.NotFound(w, r)
			return
		}

		w.Write([]byte(`{"job":{"title":"Go Dev","description":"<p>Ship <b>Go</b></p>"}}`))
	}))
	t.Cleanup(srv.Close)

	// Route the real Himalayas host to the test server.
	prev := httpClient
	httpClient = &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
		},
	}}
	t.Cleanup(func() { httpClient = prev })

	got, err := Fetch(t.Context(), "https://himalayas.app/companies/acme/jobs/go-dev?utm_source=api", "")
	if err != nil || got != "Ship Go" {
		t.Errorf("Fetch = %q, %v; want Ship Go", got, err)
	}
}

func TestHimalayasAPIURL(t *testing.T) {
	for posting, want := range map[string]string{
		"https://himalayas.app/companies/acme/jobs/go-dev?utm_source=api": "https://himalayas.app/jobs/api/companies/acme/jobs/go-dev",
		"https://www.himalayas.app/companies/acme/jobs/go-dev":            "https://himalayas.app/jobs/api/companies/acme/jobs/go-dev",
		"https://himalayas.app/jobs/go-dev":                               "",
		"https://jobs.lever.co/companies/acme":                            "",
		"://bad":                                                          "",
	} {
		if got := himalayasAPIURL(posting); got != want {
			t.Errorf("himalayasAPIURL(%q) = %q, want %q", posting, got, want)
		}
	}
}

func TestFetchRejectsBadURLs(t *testing.T) {
	for _, raw := range []string{"", "ftp://example.com/job", "file:///etc/passwd", "://bad"} {
		if _, err := Fetch(t.Context(), raw, ""); !errors.Is(err, ErrUnavailable) {
			t.Errorf("%q: err = %v, want ErrUnavailable", raw, err)
		}
	}
}

func TestPublicOnly(t *testing.T) {
	for address, wantOK := range map[string]bool{
		"93.184.216.34:443": true, "127.0.0.1:80": false, "10.0.0.5:443": false, "192.168.1.1:80": false,
		"169.254.169.254:80": false, "[::1]:443": false, "0.0.0.0:80": false, "[fd00::1]:443": false, "no-port": false,
	} {
		if err := publicOnly("tcp", address, nil); (err == nil) != wantOK {
			t.Errorf("%s: err = %v, want ok %v", address, err, wantOK)
		}
	}
}

func TestDefaultClientRefusesLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"content":"secret"}`))
	}))
	t.Cleanup(srv.Close)

	if _, err := Fetch(t.Context(), "", srv.URL); err == nil {
		t.Error("fetched a loopback address")
	}
}
