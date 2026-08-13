package jobs

import (
	"testing"
	"time"
)

func TestHimalayasJobToJob(t *testing.T) {
	tests := []struct {
		name string
		raw  himalayasJob
		want Job
	}{
		{
			name: "full job uses location restrictions, application link, and salary range",
			raw: himalayasJob{
				Title:                "Senior Backend Engineer",
				Guid:                 "https://himalayas.app/jobs/123",
				CompanyName:          "Himalayas Inc",
				MinSalary:            50000,
				MaxSalary:            90000,
				Categories:           []string{"Engineering"},
				LocationRestrictions: []string{"USA", "Canada"},
				Description:          "<p>Remote <b>job</b></p>",
				PubDate:              1700000000,
				ApplicationLink:      "https://apply.example.com/123",
			},
			want: Job{
				Title:         "Senior Backend Engineer",
				Company:       "Himalayas Inc",
				Location:      "USA, Canada",
				WorkplaceType: Remote,
				Tags:          []string{"Engineering"},
				URL:           "https://apply.example.com/123",
				Description:   "Remote job",
				SalaryMin:     intPtr(50000),
				SalaryMax:     intPtr(90000),
				PostedAt:      time.Unix(1700000000, 0),
			},
		},
		{
			name: "job with no restrictions or link falls back to worldwide and guid, no salary or date",
			raw: himalayasJob{
				Title:       "Support Specialist",
				Guid:        "https://himalayas.app/jobs/456",
				CompanyName: "Himalayas Inc",
				Description: "Worldwide role",
			},
			want: Job{
				Title:         "Support Specialist",
				Company:       "Himalayas Inc",
				Location:      "Worldwide",
				WorkplaceType: Remote,
				URL:           "https://himalayas.app/jobs/456",
				Description:   "Worldwide role",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertJobEqual(t, tt.raw.toJob(), tt.want)
		})
	}
}
