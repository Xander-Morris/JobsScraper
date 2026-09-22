package jobs

import (
	"main/utils"
	"time"
)

const remoteOKEndpoint = "https://remoteok.com/api"

var _ JobSource = (*RemoteOK)(nil)

type RemoteOK struct {
	feed
}

func NewRemoteOK(userAgent string) *RemoteOK {
	return &RemoteOK{newFeed(userAgent, remoteOKEndpoint)}
}

type remoteOKJob struct {
	ID          string   `json:"id"`
	Position    string   `json:"position"`
	Company     string   `json:"company"`
	Location    string   `json:"location"`
	Tags        []string `json:"tags"`
	SalaryMin   int      `json:"salary_min"`
	SalaryMax   int      `json:"salary_max"`
	Date        string   `json:"date"`
	URL         string   `json:"url"`
	Description string   `json:"description"`
}

func (r *RemoteOK) FetchJobs() ([]Job, error) {
	var rawJobs []remoteOKJob

	if err := r.getJSON(r.Endpoint, &rawJobs); err != nil {
		return nil, err
	}

	result := make([]Job, 0, len(rawJobs))

	for _, raw := range rawJobs {
		if raw.ID == "" || raw.Position == "" {
			continue
		}

		result = append(result, raw.toJob())
	}

	return result, nil
}

func (raw remoteOKJob) toJob() Job {
	job := Job{
		Title:         raw.Position,
		Company:       raw.Company,
		Location:      raw.Location,
		WorkplaceType: Remote,
		Tags:          utils.CleanTags(raw.Tags),
		URL:           raw.URL,
		Description:   utils.StripHTML(raw.Description),
	}

	if raw.SalaryMin > 0 {
		job.SalaryMin = &raw.SalaryMin
	}

	if raw.SalaryMax > 0 {
		job.SalaryMax = &raw.SalaryMax
	}

	if postedAt, err := time.Parse(time.RFC3339, raw.Date); err == nil {
		job.PostedAt = postedAt
	}

	return job
}
