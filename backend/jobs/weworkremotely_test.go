package jobs

import (
	"testing"
	"time"
)

func TestWeWorkRemotelyItemToJob(t *testing.T) {
	validPosted, err := time.Parse(time.RFC1123Z, "Tue, 14 Nov 2023 10:00:00 +0000")

	if err != nil {
		t.Fatalf("test setup: parse reference time: %v", err)
	}

	tests := []struct {
		name string
		raw  weWorkRemotelyItem
		want Job
	}{
		{
			name: "title splits into company and title, location falls back to country, tags combine category/type/skills",
			raw: weWorkRemotelyItem{
				Title:       "Acme Corp: Senior Go Engineer",
				Region:      "Europe",
				Country:     "Germany",
				State:       "",
				Skills:      "Go, Kubernetes, AWS",
				Category:    "Programming",
				Type:        "Full-Time",
				Description: "<p>Great <b>role</b></p>",
				PubDate:     "Tue, 14 Nov 2023 10:00:00 +0000",
				Link:        "https://weworkremotely.com/jobs/1",
			},
			want: Job{
				Title:         "Senior Go Engineer",
				Company:       "Acme Corp",
				Location:      "Germany",
				WorkplaceType: Remote,
				Tags:          []string{"Programming", "Full-Time", "Go", "Kubernetes", "AWS"},
				URL:           "https://weworkremotely.com/jobs/1",
				Description:   "Great role",
				PostedAt:      validPosted,
			},
		},
		{
			name: "title without colon has no company, location prefers state, empty skills/category/type/date leave those unset",
			raw: weWorkRemotelyItem{
				Title:       "Just A Title No Colon",
				Region:      "North America",
				Country:     "USA",
				State:       "California",
				Description: "No structured metadata",
				PubDate:     "",
				Link:        "https://weworkremotely.com/jobs/2",
			},
			want: Job{
				Title:         "Just A Title No Colon",
				Company:       "",
				Location:      "California",
				WorkplaceType: Remote,
				URL:           "https://weworkremotely.com/jobs/2",
				Description:   "No structured metadata",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertJobEqual(t, tt.raw.toJob(), tt.want)
		})
	}
}
