package jobs

import (
	"encoding/json"
	"fmt"
	"main/utils"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const himalayasEndpoint = "https://himalayas.app/jobs/api"
var himalayasCompanySlug = regexp.MustCompile(`himalayas\.app/companies/([^/]+)/`)

func companyNameFromURL(url string) string {
	match := himalayasCompanySlug.FindStringSubmatch(url)

	if match == nil {
		return ""
	}

	words := strings.Split(match[1], "-")

	for i, word := range words {
		if word == "" {
			continue
		}

		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}

	return strings.Join(words, " ")
}

var _ JobSource = (*Himalayas)(nil)

type Himalayas struct {
	HTTPClient *http.Client
	UserAgent  string
	Endpoint   string
}

func NewHimalayas(userAgent string) *Himalayas {
	return &Himalayas{
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		UserAgent:  userAgent,
		Endpoint:   himalayasEndpoint,
	}
}

type himalayasResponse struct {
	Jobs []himalayasJob `json:"jobs"`
}

type himalayasJob struct {
	Title                string   `json:"title"`
	Guid                 string   `json:"guid"`
	CompanyName          string   `json:"companyName"`
	MinSalary            float64  `json:"minSalary"`
	MaxSalary            float64  `json:"maxSalary"`
	Categories           []string `json:"categories"`
	LocationRestrictions []string `json:"locationRestrictions"`
	Description          string   `json:"description"`
	PubDate              int64    `json:"pubDate"`
	ApplicationLink      string   `json:"applicationLink"`
}

func (h *Himalayas) FetchJobs() ([]Job, error) {
	req, err := http.NewRequest("GET", h.Endpoint, nil)

	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("User-Agent", h.UserAgent)
	resp, err := h.HTTPClient.Do(req)

	if err != nil {
		return nil, fmt.Errorf("fetch himalayas feed: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("himalayas feed returned status %d", resp.StatusCode)
	}

	var parsed himalayasResponse

	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode himalayas feed: %w", err)
	}

	result := make([]Job, 0, len(parsed.Jobs))

	for _, raw := range parsed.Jobs {
		if raw.Guid == "" || raw.Title == "" {
			continue
		}

		result = append(result, raw.toJob())
	}

	return result, nil
}

func (raw himalayasJob) toJob() Job {
	location := "Worldwide"

	if len(raw.LocationRestrictions) > 0 {
		location = strings.Join(raw.LocationRestrictions, ", ")
	}

	url := raw.ApplicationLink

	if url == "" {
		url = raw.Guid
	}

	companyName := raw.CompanyName

	if companyName == "" || strings.EqualFold(companyName, "name") {
		if fallback := companyNameFromURL(url); fallback != "" {
			companyName = fallback
		}
	}

	job := Job{
		Title:         raw.Title,
		Company:       companyName,
		Location:      location,
		WorkplaceType: Remote,
		Tags:          utils.CleanTags(raw.Categories),
		URL:           url,
		Description:   utils.StripHTML(raw.Description),
	}

	if raw.MinSalary > 0 {
		minSalary := int(raw.MinSalary)
		job.SalaryMin = &minSalary
	}

	if raw.MaxSalary > 0 {
		maxSalary := int(raw.MaxSalary)
		job.SalaryMax = &maxSalary
	}

	if raw.PubDate > 0 {
		job.PostedAt = time.Unix(raw.PubDate, 0)
	}

	return job
}
