package jobs

import (
	"testing"
	"time"
)

func TestJobicyJobToJob(t *testing.T) {
	validPosted, err := time.Parse(time.RFC3339, "2023-11-14T10:00:00Z")

	if err != nil {
		t.Fatalf("test setup: parse reference time: %v", err)
	}

	tests := []struct {
		name string
		raw  jobicyJob
		want Job
	}{
		{
			name: "full job merges industry and type tags, parses salary and date",
			raw: jobicyJob{
				ID:             1,
				URL:            "https://jobicy.com/jobs/1",
				JobTitle:       "Data Analyst",
				CompanyName:    "Jobicy Co",
				JobIndustry:    []string{"Dev"},
				JobType:        []string{"Full-time"},
				JobGeo:         "Worldwide",
				JobDescription: "<p>Nice <b>role</b></p>",
				PubDate:        "2023-11-14T10:00:00Z",
				SalaryMin:      60000,
				SalaryMax:      100000,
			},
			want: Job{
				Title:         "Data Analyst",
				Company:       "Jobicy Co",
				Location:      "Worldwide",
				WorkplaceType: Remote,
				Tags:          []string{"Dev", "Full-time"},
				URL:           "https://jobicy.com/jobs/1",
				Description:   "Nice role",
				SalaryMin:     intPtr(60000),
				SalaryMax:     intPtr(100000),
				PostedAt:      validPosted,
			},
		},
		{
			name: "job with zero salary and unparsable date leaves those fields unset",
			raw: jobicyJob{
				ID:             2,
				URL:            "https://jobicy.com/jobs/2",
				JobTitle:       "Junior Designer",
				CompanyName:    "Jobicy Co",
				JobGeo:         "Worldwide",
				JobDescription: "Entry level role",
				PubDate:        "not-a-date",
			},
			want: Job{
				Title:         "Junior Designer",
				Company:       "Jobicy Co",
				Location:      "Worldwide",
				WorkplaceType: Remote,
				URL:           "https://jobicy.com/jobs/2",
				Description:   "Entry level role",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertJobEqual(t, tt.raw.toJob(), tt.want)
		})
	}
}
