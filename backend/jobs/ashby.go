package jobs

import (
	"main/utils"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const ashbyEndpoint = "https://api.ashbyhq.com/posting-api/job-board"

// ashbyBoards are the companies whose public Ashby boards get scraped.
var ashbyBoards = []Board{
	{"1password", "1Password"}, {"docker", "Docker"}, {"linear", "Linear"}, {"notion", "Notion"},
	{"openai", "OpenAI"}, {"posthog", "PostHog"}, {"ramp", "Ramp"}, {"replit", "Replit"},
	{"supabase", "Supabase"}, {"zapier", "Zapier"},
}

// camelCaseBoundary splits employment types like "FullTime" into "Full Time".
var camelCaseBoundary = regexp.MustCompile(`([a-z])([A-Z])`)

var _ JobSource = (*Ashby)(nil)

type Ashby struct {
	HTTPClient *http.Client
	UserAgent  string
	Endpoint   string
	Boards     []Board
}

func NewAshby(userAgent string) *Ashby {
	return &Ashby{
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
		UserAgent:  userAgent,
		Endpoint:   ashbyEndpoint,
		Boards:     ashbyBoards,
	}
}

type ashbyResponse struct {
	Jobs []ashbyJob `json:"jobs"`
}

type ashbyJob struct {
	Title           string `json:"title"`
	Location        string `json:"location"`
	IsRemote        bool   `json:"isRemote"`
	IsListed        bool   `json:"isListed"`
	WorkplaceType   string `json:"workplaceType"`
	EmploymentType  string `json:"employmentType"`
	Department      string `json:"department"`
	Team            string `json:"team"`
	PublishedAt     string `json:"publishedAt"`
	JobURL          string `json:"jobUrl"`
	DescriptionHTML string `json:"descriptionHtml"`
}

func (a *Ashby) FetchJobs() ([]Job, error) {
	return fetchBoards("ashby", a.Boards, func(board Board) ([]Job, error) {
		var parsed ashbyResponse

		if err := getJSON(a.HTTPClient, a.UserAgent, a.Endpoint+"/"+board.Slug, &parsed); err != nil {
			return nil, err
		}

		result := make([]Job, 0, len(parsed.Jobs))

		for _, raw := range parsed.Jobs {
			if !raw.IsListed || raw.JobURL == "" || raw.Title == "" {
				continue
			}

			result = append(result, raw.toJob(board.Company))
		}

		return result, nil
	})
}

func (raw ashbyJob) toJob(company string) Job {
	workplaceType, _ := ParseWorkplaceType(raw.WorkplaceType)

	if workplaceType == Unknown && raw.IsRemote {
		workplaceType = Remote
	}

	employmentType := camelCaseBoundary.ReplaceAllString(raw.EmploymentType, "$1 $2")

	job := Job{
		Title:         strings.TrimSpace(raw.Title),
		Company:       company,
		Location:      raw.Location,
		WorkplaceType: workplaceType,
		Tags:          utils.CleanTags([]string{raw.Department, raw.Team, employmentType}),
		URL:           raw.JobURL,
		Description:   utils.StripHTML(raw.DescriptionHTML),
	}

	if postedAt, err := time.Parse(time.RFC3339, raw.PublishedAt); err == nil {
		job.PostedAt = postedAt
	}

	return job
}
