package jobs

import (
	"main/utils"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const remotiveEndpoint = "https://remotive.com/api/remote-jobs"

var _ JobSource = (*Remotive)(nil)

type Remotive struct {
	feed
}

func NewRemotive(userAgent string) *Remotive {
	return &Remotive{newFeed(userAgent, remotiveEndpoint)}
}

type remotiveResponse struct {
	Jobs []remotiveJob `json:"jobs"`
}

type remotiveJob struct {
	ID                        int    `json:"id"`
	Title                     string `json:"title"`
	CompanyName               string `json:"company_name"`
	CompanyLogo               string `json:"company_logo"`
	Category                  string `json:"category"`
	JobType                   string `json:"job_type"`
	Date                      string `json:"date"`
	URL                       string `json:"url"`
	Description               string `json:"description"`
	Salary                    string `json:"salary"`
	CandidateRequiredLocation string `json:"candidate_required_location"`
}

func (r *Remotive) FetchJobs() ([]Job, error) {
	var parsed remotiveResponse

	if err := r.getJSON(r.Endpoint, &parsed); err != nil {
		return nil, err
	}

	rawJobs := parsed.Jobs
	result := make([]Job, 0, len(rawJobs))

	for _, raw := range rawJobs {
		if raw.ID == 0 || raw.Title == "" {
			continue
		}

		result = append(result, raw.toJob())
	}

	return result, nil
}

func (raw remotiveJob) toJob() Job {
	// job_type comes as e.g. "full_time"; spaced so it reads like other sources' type tags.
	var tags = []string{raw.Category, strings.ReplaceAll(raw.JobType, "_", " ")}

	job := Job{
		Title:         raw.Title,
		Company:       raw.CompanyName,
		Location:      raw.CandidateRequiredLocation,
		WorkplaceType: Remote,
		Tags:          utils.CleanTags(tags),
		URL:           raw.URL,
		Description:   utils.StripHTML(raw.Description),
	}

	var chars []rune

	for _, char := range raw.Salary {
		if unicode.IsDigit(char) {
			chars = append(chars, char)
		} else {
			s := string(chars)
			clear(chars)
			num, err := strconv.Atoi(s)

			if err != nil {
				continue
			}

			if job.SalaryMin == nil {
				job.SalaryMin = &num
			}

			if job.SalaryMax == nil {
				job.SalaryMax = &num
			}
		}
	}

	if postedAt, err := time.Parse(time.RFC3339, raw.Date); err == nil {
		job.PostedAt = postedAt
	}

	return job
}
