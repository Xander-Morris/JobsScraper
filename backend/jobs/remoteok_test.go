package jobs

import (
	"testing"
	"time"
)

func TestRemoteOKJobToJob(t *testing.T) {
	validPosted, err := time.Parse(time.RFC3339, "2023-11-14T10:00:00Z")

	if err != nil {
		t.Fatalf("test setup: parse reference time: %v", err)
	}

	tests := []struct {
		name string
		raw  remoteOKJob
		want Job
	}{
		{
			name: "full job carries tags and salary range as-is, parses date",
			raw: remoteOKJob{
				ID:          "123",
				Position:    "Frontend Engineer",
				Company:     "RemoteOK Co",
				Location:    "Worldwide",
				Tags:        []string{"react", "typescript"},
				SalaryMin:   70000,
				SalaryMax:   110000,
				Date:        "2023-11-14T10:00:00Z",
				URL:         "https://remoteok.com/jobs/123",
				Description: "<p>Great <b>role</b></p>",
			},
			want: Job{
				Title:         "Frontend Engineer",
				Company:       "RemoteOK Co",
				Location:      "Worldwide",
				WorkplaceType: Remote,
				Tags:          []string{"react", "typescript"},
				URL:           "https://remoteok.com/jobs/123",
				Description:   "Great role",
				SalaryMin:     intPtr(70000),
				SalaryMax:     intPtr(110000),
				PostedAt:      validPosted,
			},
		},
		{
			name: "zero salary and unparsable date leave those fields unset",
			raw: remoteOKJob{
				ID:          "456",
				Position:    "QA Tester",
				Company:     "RemoteOK Co",
				Location:    "Worldwide",
				Date:        "not-a-date",
				URL:         "https://remoteok.com/jobs/456",
				Description: "Test things",
			},
			want: Job{
				Title:         "QA Tester",
				Company:       "RemoteOK Co",
				Location:      "Worldwide",
				WorkplaceType: Remote,
				URL:           "https://remoteok.com/jobs/456",
				Description:   "Test things",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertJobEqual(t, tt.raw.toJob(), tt.want)
		})
	}
}
