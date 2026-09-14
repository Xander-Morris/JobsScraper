package jobs

import (
	"testing"
	"time"
)

func TestGreenhouseJobToJob(t *testing.T) {
	posted, err := time.Parse(time.RFC3339, "2026-09-01T10:00:00-04:00")
	if err != nil {
		t.Fatalf("test setup: parse reference time: %v", err)
	}

	tests := []struct {
		name string
		raw  greenhouseJob
		want Job
	}{
		{
			name: "remote location, escaped html content, and departments as tags",
			raw: greenhouseJob{
				Title:          " Staff Engineer ",
				AbsoluteURL:    "https://boards.greenhouse.io/acme/jobs/1",
				FirstPublished: "2026-09-01T10:00:00-04:00",
				Content:        "&lt;p&gt;Build &lt;b&gt;things&lt;/b&gt;&lt;/p&gt;",
				Location:       greenhouseLocation{Name: "Remote - US"},
				Departments:    []greenhouseDepartment{{Name: "Engineering"}},
			},
			want: Job{
				Title:         "Staff Engineer",
				Company:       "Acme",
				Location:      "Remote - US",
				WorkplaceType: Remote,
				Tags:          []string{"Engineering"},
				URL:           "https://boards.greenhouse.io/acme/jobs/1",
				Description:   "Build things",
				PostedAt:      posted,
			},
		},
		{
			name: "office location stays unknown workplace and missing publish date stays zero",
			raw: greenhouseJob{
				Title:       "Recruiter",
				AbsoluteURL: "https://boards.greenhouse.io/acme/jobs/2",
				Location:    greenhouseLocation{Name: "San Francisco, CA"},
			},
			want: Job{
				Title:    "Recruiter",
				Company:  "Acme",
				Location: "San Francisco, CA",
				URL:      "https://boards.greenhouse.io/acme/jobs/2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertJobEqual(t, tt.raw.toJob("Acme"), tt.want)
		})
	}
}
