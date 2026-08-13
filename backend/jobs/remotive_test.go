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
		raw  RemotiveJob
		want Job
	}{
		{
			name: "salary string yields same min and max from first parsed number",
			raw: RemotiveJob{
				ID:                        1,
				Title:                     "Backend Engineer",
				CompanyName:               "RemoteCo",
				Category:                  "Software Development",
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
				Tags:          []string{"Software Development"},
				URL:           "https://remotive.com/jobs/1",
				Description:   "Great role",
				SalaryMin:     intPtr(50),
				SalaryMax:     intPtr(50),
				PostedAt:      validPosted,
			},
		},
		{
			name: "empty salary and unparsable date leave those fields unset",
			raw: RemotiveJob{
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
