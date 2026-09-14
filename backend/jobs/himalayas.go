package jobs

import (
	"fmt"
	"log/slog"
	"main/utils"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
)

const himalayasEndpoint = "https://himalayas.app/jobs/api"

// himalayasMaxPages caps each run; the API returns at most 20 jobs per page.
const himalayasMaxPages = 25

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
	Jobs       []himalayasJob `json:"jobs"`
	NextCursor string         `json:"nextCursor"`
}

type himalayasJob struct {
	Title                string   `json:"title"`
	Guid                 string   `json:"guid"`
	CompanyName          string   `json:"companyName"`
	MinSalary            float64  `json:"minSalary"`
	MaxSalary            float64  `json:"maxSalary"`
	Categories           []string `json:"categories"`
	EmploymentType       string   `json:"employmentType"`
	LocationRestrictions []string `json:"locationRestrictions"`
	Description          string   `json:"description"`
	PubDate              int64    `json:"pubDate"`
	ApplicationLink      string   `json:"applicationLink"`
}

// FetchJobs follows the feed's cursor pagination. A failed later page keeps the pages already fetched.
func (h *Himalayas) FetchJobs() ([]Job, error) {
	var result []Job
	cursor := ""

	for page := 1; page <= himalayasMaxPages; page++ {
		pageURL := h.Endpoint

		if cursor != "" {
			pageURL += "?cursor=" + url.QueryEscape(cursor)
		}

		var parsed himalayasResponse

		if err := getJSON(h.HTTPClient, h.UserAgent, pageURL, &parsed); err != nil {
			if len(result) > 0 {
				slog.Warn("jobs: himalayas page failed, keeping earlier pages", "page", page, "error", err)
				break
			}

			return nil, fmt.Errorf("fetch himalayas feed: %w", err)
		}

		for _, raw := range parsed.Jobs {
			if raw.Guid == "" || raw.Title == "" {
				continue
			}

			result = append(result, raw.toJob())
		}

		if parsed.NextCursor == "" || len(parsed.Jobs) == 0 {
			break
		}

		cursor = parsed.NextCursor
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
		Tags:          utils.CleanTags(append(slices.Clone(raw.Categories), raw.EmploymentType)),
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
