package crawler

import (
	"net/url"
	"testing"
)

func TestWorkdaySeed(t *testing.T) {
	seed := workday("https://acme.wd1.myworkdayjobs.com/External", "Acme")

	if seed.URL != "https://acme.wd1.myworkdayjobs.com/External" || seed.Company != "Acme" || seed.Include == nil {
		t.Fatalf("seed = %+v", seed)
	}

	for path, want := range map[string]bool{
		"/External/job/Remote/Engineer_R123": true,
		"/External/login":                    false,
		"/External":                          false,
		"/External/jobs":                     false,
	} {
		if got := seed.Include.MatchString(path); got != want {
			t.Errorf("Include(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestSeedsAreValid(t *testing.T) {
	if len(seeds) == 0 {
		t.Fatal("no seeds")
	}

	seen := map[string]bool{}

	for _, s := range seeds {
		u, err := url.Parse(s.URL)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			t.Errorf("seed %q is not an absolute https URL", s.URL)
		}

		if s.Company == "" {
			t.Errorf("seed %q has no company", s.URL)
		}

		if seen[s.URL] {
			t.Errorf("seed %q listed twice", s.URL)
		}
		seen[s.URL] = true
	}
}
