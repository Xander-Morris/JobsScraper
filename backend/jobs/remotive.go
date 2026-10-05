package jobs

import (
	"main/utils"
	"regexp"
	"strconv"
	"strings"
	"time"
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

// salaryAmount matches "50k", "50,000" or "50000.00".
var salaryAmount = regexp.MustCompile(`(\d+(?:,\d{3})*(?:\.\d+)?)\s*([kK])?`)

// parseSalaryRange reads free text like "$50k - $70k" as a yearly range. Hourly rates and
// amounts under MinSalary are skipped, since they'd render as a misleading "$0k".
func parseSalaryRange(text string) (*int, *int) {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "hour") || strings.Contains(lower, "/hr") {
		return nil, nil
	}

	var amounts []int

	for _, match := range salaryAmount.FindAllStringSubmatch(text, -1) {
		amount, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
		if err != nil {
			continue
		}

		if match[2] != "" {
			amount *= 1000
		}

		if amount >= MinSalary {
			amounts = append(amounts, int(amount))
		}
	}

	if len(amounts) == 0 {
		return nil, nil
	}

	low, high := amounts[0], amounts[0]
	if len(amounts) > 1 {
		low, high = min(amounts[0], amounts[1]), max(amounts[0], amounts[1])
	}

	return &low, &high
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

	job.SalaryMin, job.SalaryMax = parseSalaryRange(raw.Salary)

	if postedAt, err := time.Parse(time.RFC3339, raw.Date); err == nil {
		job.PostedAt = postedAt
	}

	return job
}
