package jobs

import (
	"encoding/json"
	"fmt"
	"main/utils"
	"net/http"
	"time"
)

const jobicyEndpoint = "https://jobicy.com/api/v2/remote-jobs"

var _ JobSource = (*Jobicy)(nil)

type Jobicy struct {
	HTTPClient *http.Client
	UserAgent  string
	Endpoint   string
}

func NewJobicy(userAgent string) *Jobicy {
	return &Jobicy{
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		UserAgent:  userAgent,
		Endpoint:   jobicyEndpoint,
	}
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
	req, err := http.NewRequest("GET", j.Endpoint, nil)

	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("User-Agent", j.UserAgent)
	resp, err := j.HTTPClient.Do(req)

	if err != nil {
		return nil, fmt.Errorf("fetch jobicy feed: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jobicy feed returned status %d", resp.StatusCode)
	}

	var parsed jobicyResponse

	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode jobicy feed: %w", err)
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
