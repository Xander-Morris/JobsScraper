package jobs

import (
	"testing"
	"time"
)

func TestLeverJobToJob(t *testing.T) {
	tests := []struct {
		name string
		raw  leverJob
		want Job
	}{
		{
			name: "onsite workplace, commitment tag, and lists folded into description",
			raw: leverJob{
				Text:             "Software Engineer",
				HostedURL:        "https://jobs.lever.co/acme/1",
				CreatedAt:        1788000000000,
				WorkplaceType:    "onsite",
				DescriptionPlain: "About the role",
				AdditionalPlain:  "Benefits",
				Lists:            []leverList{{Text: "Requirements", Content: "<li>Go</li><li>SQL</li>"}},
				Categories:       leverCategories{Commitment: "Intern", Location: "London, United Kingdom", Team: "Engineering"},
			},
			want: Job{
				Title:         "Software Engineer",
				Company:       "Acme",
				Location:      "London, United Kingdom",
				WorkplaceType: InPerson,
				Tags:          []string{"Engineering", "Intern"},
				URL:           "https://jobs.lever.co/acme/1",
				Description:   "About the role\n\nRequirements\n• Go\n\n• SQL\n\nBenefits",
				PostedAt:      time.UnixMilli(1788000000000),
			},
		},
		{
			name: "unspecified workplace falls back to a remote location",
			raw: leverJob{
				Text:          "Support Specialist",
				HostedURL:     "https://jobs.lever.co/acme/2",
				WorkplaceType: "unspecified",
				Categories:    leverCategories{Location: "Remote"},
			},
			want: Job{
				Title:         "Support Specialist",
				Company:       "Acme",
				Location:      "Remote",
				WorkplaceType: Remote,
				URL:           "https://jobs.lever.co/acme/2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertJobEqual(t, tt.raw.toJob("Acme"), tt.want)
		})
	}
}
