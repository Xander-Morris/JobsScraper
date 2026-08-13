package jobs

import (
	"strconv"
	"testing"
)

func assertJobEqual(t *testing.T, got, want Job) {
	t.Helper()

	if got.Title != want.Title {
		t.Errorf("Title = %q, want %q", got.Title, want.Title)
	}

	if got.Company != want.Company {
		t.Errorf("Company = %q, want %q", got.Company, want.Company)
	}

	if got.Location != want.Location {
		t.Errorf("Location = %q, want %q", got.Location, want.Location)
	}

	if got.WorkplaceType != want.WorkplaceType {
		t.Errorf("WorkplaceType = %v, want %v", got.WorkplaceType, want.WorkplaceType)
	}

	if !equalStringSlices(got.Tags, want.Tags) {
		t.Errorf("Tags = %v, want %v", got.Tags, want.Tags)
	}

	if got.URL != want.URL {
		t.Errorf("URL = %q, want %q", got.URL, want.URL)
	}

	if got.Description != want.Description {
		t.Errorf("Description = %q, want %q", got.Description, want.Description)
	}

	if !got.PostedAt.Equal(want.PostedAt) {
		t.Errorf("PostedAt = %v, want %v", got.PostedAt, want.PostedAt)
	}

	if !equalIntPtr(got.SalaryMin, want.SalaryMin) {
		t.Errorf("SalaryMin = %s, want %s", intPtrString(got.SalaryMin), intPtrString(want.SalaryMin))
	}

	if !equalIntPtr(got.SalaryMax, want.SalaryMax) {
		t.Errorf("SalaryMax = %s, want %s", intPtrString(got.SalaryMax), intPtrString(want.SalaryMax))
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func equalIntPtr(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}

func intPtrString(p *int) string {
	if p == nil {
		return "<nil>"
	}

	return strconv.Itoa(*p)
}

func intPtr(v int) *int {
	return &v
}
