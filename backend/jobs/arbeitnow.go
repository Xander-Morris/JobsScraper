package jobs

import (
	"encoding/json"
	"fmt"
	"main/utils"
	"net/http"
	"time"
)

const arbeitnowEndpoint = "https://www.arbeitnow.com/api/job-board-api"

var _ JobSource = (*Arbeitnow)(nil)

type Arbeitnow struct {
	HTTPClient *http.Client
	UserAgent  string
	Endpoint   string
}

func NewArbeitnow(userAgent string) *Arbeitnow {
	return &Arbeitnow{
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		UserAgent:  userAgent,
		Endpoint:   arbeitnowEndpoint,
	}
}

type arbeitnowResponse struct {
	Data []arbeitnowJob `json:"data"`
}

type arbeitnowJob struct {
	Slug        string            `json:"slug"`
	CompanyName string            `json:"company_name"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Remote      bool              `json:"remote"`
	URL         string            `json:"url"`
	Tags        []string          `json:"tags"`
	JobTypes    arbeitnowJobTypes `json:"job_types"`
	Location    string            `json:"location"`
	CreatedAt   int64             `json:"created_at"`
}

type arbeitnowJobTypes []string

func (jt *arbeitnowJobTypes) UnmarshalJSON(data []byte) error {
	var asSlice []string
	if err := json.Unmarshal(data, &asSlice); err == nil {
		*jt = asSlice
		return nil
	}

	var asMap map[string]string
	if err := json.Unmarshal(data, &asMap); err != nil {
		return err
	}

	values := make([]string, 0, len(asMap))
	for _, v := range asMap {
		values = append(values, v)
	}

	*jt = values
	return nil
}

func (a *Arbeitnow) FetchJobs() ([]Job, error) {
	req, err := http.NewRequest("GET", a.Endpoint, nil)

	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("User-Agent", a.UserAgent)
	resp, err := a.HTTPClient.Do(req)

	if err != nil {
		return nil, fmt.Errorf("fetch arbeitnow feed: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("arbeitnow feed returned status %d", resp.StatusCode)
	}

	var parsed arbeitnowResponse

	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode arbeitnow feed: %w", err)
	}

	result := make([]Job, 0, len(parsed.Data))

	for _, raw := range parsed.Data {
		if raw.Slug == "" || raw.Title == "" {
			continue
		}

		result = append(result, raw.toJob())
	}

	return result, nil
}

func (raw arbeitnowJob) toJob() Job {
	workplaceType := Unknown

	if raw.Remote {
		workplaceType = Remote
	}

	tags := make([]string, 0, len(raw.Tags)+len(raw.JobTypes))
	tags = append(tags, raw.Tags...)
	tags = append(tags, raw.JobTypes...)

	job := Job{
		Title:         raw.Title,
		Company:       raw.CompanyName,
		Location:      raw.Location,
		WorkplaceType: workplaceType,
		Tags:          utils.CleanTags(tags),
		URL:           raw.URL,
		Description:   utils.StripHTML(raw.Description),
	}

	if raw.CreatedAt > 0 {
		job.PostedAt = time.Unix(raw.CreatedAt, 0)
	}

	return job
}
