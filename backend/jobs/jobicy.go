package jobs

import (
	"main/utils"
	"time"
)

// count=100 returns 100 jobs in one request instead of the default page.
const jobicyEndpoint = "https://jobicy.com/api/v2/remote-jobs?count=100"

var _ JobSource = (*Jobicy)(nil)

type Jobicy struct {
	feed
}

func NewJobicy(userAgent string) *Jobicy {
	return &Jobicy{newFeed(userAgent, jobicyEndpoint)}
}

type jobicyResponse struct {
	Jobs []jobicyJob `json:"jobs"`
}

type jobicyJob struct {
	ID             int      `json:"id"`
	URL            string   `json:"url"`
	JobTitle       string   `json:"jobTitle"`
	CompanyName    string   `json:"companyName"`
	JobIndustry    []string `json:"jobIndustry"`
	JobType        []string `json:"jobType"`
	JobGeo         string   `json:"jobGeo"`
	JobDescription string   `json:"jobDescription"`
	PubDate        string   `json:"pubDate"`
	SalaryMin      int      `json:"salaryMin"`
	SalaryMax      int      `json:"salaryMax"`
}

func (j *Jobicy) FetchJobs() ([]Job, error) {
	var parsed jobicyResponse

	if err := j.getJSON(j.Endpoint, &parsed); err != nil {
		return nil, err
	}

	result := make([]Job, 0, len(parsed.Jobs))

	for _, raw := range parsed.Jobs {
		if raw.ID == 0 || raw.JobTitle == "" {
			continue
		}

		result = append(result, raw.toJob())
	}

	return result, nil
}

func (raw jobicyJob) toJob() Job {
	tags := make([]string, 0, len(raw.JobIndustry)+len(raw.JobType))
	tags = append(tags, raw.JobIndustry...)
	tags = append(tags, raw.JobType...)

	job := Job{
		Title:         raw.JobTitle,
		Company:       raw.CompanyName,
		Location:      raw.JobGeo,
		WorkplaceType: Remote,
		Tags:          utils.CleanTags(tags),
		URL:           raw.URL,
		Description:   utils.StripHTML(raw.JobDescription),
	}

	if raw.SalaryMin > 0 {
		job.SalaryMin = &raw.SalaryMin
	}

	if raw.SalaryMax > 0 {
		job.SalaryMax = &raw.SalaryMax
	}

	if postedAt, err := time.Parse(time.RFC3339, raw.PubDate); err == nil {
		job.PostedAt = postedAt
	}

	return job
}
