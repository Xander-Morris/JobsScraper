package jobs

import (
	"net/http"
	"slices"
	"strings"
	"testing"
)

func TestHimalayasKeepsEarlierPagesOnFailure(t *testing.T) {
	srv, _ := feedServer(t, http.StatusOK, func(r *http.Request) string {
		if r.URL.Query().Get("cursor") != "" {
			return "{"
		}
		return `{"jobs":[{"guid":"a","title":"First"},{"guid":"","title":"Skipped"}],"nextCursor":"more"}`
	})

	h := NewHimalayas("test-agent")
	h.Endpoint = srv.URL

	got, err := h.FetchJobs(t.Context())
	if err != nil {
		t.Fatalf("err = %v, want earlier pages kept", err)
	}
	if g := titles(got); !slices.Equal(g, []string{"First"}) {
		t.Errorf("titles = %v", g)
	}
}

func TestCompanyNameFromURLSkipsEmptyWords(t *testing.T) {
	if got := companyNameFromURL("https://himalayas.app/companies/acme--labs/jobs/x"); got != "Acme  Labs" {
		t.Errorf("companyNameFromURL = %q", got)
	}
}

func TestYearlySalaryDailyAndWeekly(t *testing.T) {
	for period, want := range map[string]int{"daily": 260 * 400, "per week": 52 * 400} {
		low, _ := YearlySalary(400, 400, period, "USD")
		if low == nil || *low != want {
			t.Errorf("YearlySalary(400, %q) = %v, want %d", period, low, want)
		}
	}
}

func TestParseSalaryTextRejects(t *testing.T) {
	huge := strings.Repeat("9", 400)
	for _, text := range []string{
		"$50,000 - CA$70,000",
		"$" + huge,
		"$50,000 - $" + huge,
	} {
		if low, high := ParseSalaryText(text, false); low != nil || high != nil {
			t.Errorf("ParseSalaryText(%q) = %v, %v, want nil", text, low, high)
		}
	}
}
