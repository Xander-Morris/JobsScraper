package jobs

import (
	"testing"
	"time"
)

func TestRemotiveJobToJob(t *testing.T) {
	validPosted, err := time.Parse(time.RFC3339, "2023-11-14T10:00:00Z")

	if err != nil {
		t.Fatalf("test setup: parse reference time: %v", err)
	}

	tests := []struct {
		name string
		raw  remotiveJob
		want Job
	}{
		{
			name: "single k salary yields same min and max",
			raw: remotiveJob{
				ID:                        1,
				Title:                     "Backend Engineer",
				CompanyName:               "RemoteCo",
				Category:                  "Software Development",
				JobType:                   "full_time",
				Date:                      "2023-11-14T10:00:00Z",
				URL:                       "https://remotive.com/jobs/1",
				Description:               "<p>Great <b>role</b></p>",
				Salary:                    "$50k",
				CandidateRequiredLocation: "USA",
			},
			want: Job{
				Title:         "Backend Engineer",
				Company:       "RemoteCo",
				Location:      "USA",
				WorkplaceType: Remote,
				Tags:          []string{"Software Development", "full time"},
				URL:           "https://remotive.com/jobs/1",
				Description:   "Great role",
				SalaryMin:     intPtr(50000),
				SalaryMax:     intPtr(50000),
				PostedAt:      validPosted,
			},
		},
		{
			name: "empty salary and unparsable date leave those fields unset",
			raw: remotiveJob{
				ID:                        2,
				Title:                     "Support Engineer",
				CompanyName:               "RemoteCo",
				Category:                  "Customer Support",
				Date:                      "not-a-date",
				URL:                       "https://remotive.com/jobs/2",
				Description:               "Help customers",
				Salary:                    "",
				CandidateRequiredLocation: "Worldwide",
			},
			want: Job{
				Title:         "Support Engineer",
				Company:       "RemoteCo",
				Location:      "Worldwide",
				WorkplaceType: Remote,
				Tags:          []string{"Customer Support"},
				URL:           "https://remotive.com/jobs/2",
				Description:   "Help customers",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertJobEqual(t, tt.raw.toJob(), tt.want)
		})
	}
}

func TestParseSalaryRange(t *testing.T) {
	tests := []struct {
		text     string
		min, max *int
	}{
		{"$100k - $150k", intPtr(100000), intPtr(150000)},
		{"$80,000 – $120,000 USD", intPtr(80000), intPtr(120000)},
		{"$120K-$90K", intPtr(90000), intPtr(120000)},
		{"$95000", intPtr(95000), intPtr(95000)},
		{"$40 - $60 per hour", nil, nil},
		{"$50/hr", nil, nil},
		{"50", nil, nil},
		{"Competitive", nil, nil},
		{"", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			min, max := parseSalaryRange(tt.text)

			if !intPtrEqual(min, tt.min) || !intPtrEqual(max, tt.max) {
				t.Errorf("parseSalaryRange(%q) = %v, %v, want %v, %v", tt.text, deref(min), deref(max), deref(tt.min), deref(tt.max))
			}
		})
	}
}

func intPtrEqual(a, b *int) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func deref(p *int) any {
	if p == nil {
		return nil
	}

	return *p
}
