package jobs

import (
	"testing"
	"time"
)

func TestAshbyJobToJob(t *testing.T) {
	posted, err := time.Parse(time.RFC3339, "2026-04-07T17:12:35.753+00:00")
	if err != nil {
		t.Fatalf("test setup: parse reference time: %v", err)
	}

	tests := []struct {
		name string
		raw  ashbyJob
		want Job
	}{
		{
			name: "hybrid workplace wins over isRemote, employment type spaced into a tag",
			raw: ashbyJob{
				Title:           " Security Engineer ",
				Location:        "New York, NY",
				IsRemote:        true,
				IsListed:        true,
				WorkplaceType:   "Hybrid",
				EmploymentType:  "PartTime",
				Department:      "Engineering",
				Team:            "Security",
				PublishedAt:     "2026-04-07T17:12:35.753+00:00",
				JobURL:          "https://jobs.ashbyhq.com/acme/1",
				DescriptionHTML: "<p>Protect things</p>",
			},
			want: Job{
				Title:         "Security Engineer",
				Company:       "Acme",
				Location:      "New York, NY",
				WorkplaceType: Hybrid,
				Tags:          []string{"Engineering", "Security", "Part Time"},
				URL:           "https://jobs.ashbyhq.com/acme/1",
				Description:   "Protect things",
				PostedAt:      posted,
			},
		},
		{
			name: "missing workplace type falls back to isRemote",
			raw: ashbyJob{
				Title:          "Designer",
				IsRemote:       true,
				IsListed:       true,
				EmploymentType: "FullTime",
				JobURL:         "https://jobs.ashbyhq.com/acme/2",
			},
			want: Job{
				Title:         "Designer",
				Company:       "Acme",
				WorkplaceType: Remote,
				Tags:          []string{"Full Time"},
				URL:           "https://jobs.ashbyhq.com/acme/2",
			},
		},
		{
			name: "OnSite parses as in person",
			raw: ashbyJob{
				Title:         "Hardware Engineer",
				IsListed:      true,
				WorkplaceType: "OnSite",
				JobURL:        "https://jobs.ashbyhq.com/acme/3",
			},
			want: Job{
				Title:         "Hardware Engineer",
				Company:       "Acme",
				WorkplaceType: InPerson,
				URL:           "https://jobs.ashbyhq.com/acme/3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertJobEqual(t, tt.raw.toJob("Acme"), tt.want)
		})
	}
}
