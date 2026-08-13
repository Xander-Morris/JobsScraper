package jobs

import (
	"encoding/json"
	"fmt"
	"main/utils"
	"net/http"
	"strconv"
	"time"
	"unicode"
)

const remotiveEndpoint = "https://remotive.com/api/remote-jobs"

var _ JobSource = (*Remotive)(nil)

type Remotive struct {
	HTTPClient *http.Client
	UserAgent  string
	Endpoint   string
}

func NewRemotive(userAgent string) *Remotive {
	return &Remotive{
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		UserAgent:  userAgent,
		Endpoint:   remotiveEndpoint,
	}
}

type remotiveResponse struct {
	Jobs []RemotiveJob `json:"jobs"`
}

type RemotiveJob struct {
	ID                        int    `json:"id"`
	Title                     string `json:"title"`
	CompanyName               string `json:"company_name"`
	CompanyLogo               string `json:"company_logo"`
	Category                  string `json:"category"`
	JobType                   string `json:"full_time"`
	Date                      string `json:"date"`
	URL                       string `json:"url"`
	Description               string `json:"description"`
	Salary                    string `json:"salary"`
	CandidateRequiredLocation string `json:"candidate_required_location"`
}

func (r *Remotive) FetchJobs() ([]Job, error) {
	req, err := http.NewRequest("GET", r.Endpoint, nil)

	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("User-Agent", r.UserAgent)
	resp, err := r.HTTPClient.Do(req)

	if err != nil {
		return nil, fmt.Errorf("fetch remotive feed: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remoteok feed returned status %d", resp.StatusCode)
	}

	var parsed remotiveResponse

	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode remotive feed: %w", err)
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

func (raw RemotiveJob) toJob() Job {
	var tags = []string{raw.Category}

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
