package jobs

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"main/utils"
	"net/http"
	"time"
)

const arbeitnowEndpoint = "https://www.arbeitnow.com/api/job-board-api"

// arbeitnowMaxPages caps each run; pages are ordered by created_at.
const arbeitnowMaxPages = 5

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
	Data  []arbeitnowJob `json:"data"`
	Links struct {
		Next *string `json:"next"`
	} `json:"links"`
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

// FetchJobs pages through the feed. A failed later page keeps the pages already fetched.
func (a *Arbeitnow) FetchJobs() ([]Job, error) {
	var result []Job

	for page := 1; page <= arbeitnowMaxPages; page++ {
		var parsed arbeitnowResponse

		if err := getJSON(a.HTTPClient, a.UserAgent, fmt.Sprintf("%s?page=%d", a.Endpoint, page), &parsed); err != nil {
			if len(result) > 0 {
				slog.Warn("jobs: arbeitnow page failed, keeping earlier pages", "page", page, "error", err)
				break
			}

			return nil, fmt.Errorf("fetch arbeitnow feed: %w", err)
		}

		for _, raw := range parsed.Data {
			if raw.Slug == "" || raw.Title == "" {
				continue
			}

			result = append(result, raw.toJob())
		}

		if parsed.Links.Next == nil || len(parsed.Data) == 0 {
			break
		}
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
