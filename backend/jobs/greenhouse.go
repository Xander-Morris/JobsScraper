package jobs

import (
	"main/utils"
	"net/http"
	"strings"
	"time"
)

const greenhouseEndpoint = "https://boards-api.greenhouse.io/v1/boards"

// greenhouseBoards are the companies whose public Greenhouse boards get scraped.
var greenhouseBoards = []Board{
	{"affirm", "Affirm"}, {"airbnb", "Airbnb"}, {"anthropic", "Anthropic"}, {"asana", "Asana"},
	{"brex", "Brex"}, {"cloudflare", "Cloudflare"}, {"coinbase", "Coinbase"}, {"databricks", "Databricks"},
	{"datadog", "Datadog"}, {"discord", "Discord"}, {"dropbox", "Dropbox"}, {"duolingo", "Duolingo"},
	{"elastic", "Elastic"}, {"figma", "Figma"}, {"gitlab", "GitLab"}, {"gusto", "Gusto"},
	{"instacart", "Instacart"}, {"lyft", "Lyft"}, {"mongodb", "MongoDB"}, {"okta", "Okta"},
	{"pinterest", "Pinterest"}, {"reddit", "Reddit"}, {"robinhood", "Robinhood"}, {"roblox", "Roblox"},
	{"samsara", "Samsara"}, {"squarespace", "Squarespace"}, {"stripe", "Stripe"}, {"toast", "Toast"},
	{"twilio", "Twilio"}, {"zscaler", "Zscaler"},
}

var _ JobSource = (*Greenhouse)(nil)

type Greenhouse struct {
	HTTPClient *http.Client
	UserAgent  string
	Endpoint   string
	Boards     []Board
}

func NewGreenhouse(userAgent string) *Greenhouse {
	return &Greenhouse{
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
		UserAgent:  userAgent,
		Endpoint:   greenhouseEndpoint,
		Boards:     greenhouseBoards,
	}
}

type greenhouseResponse struct {
	Jobs []greenhouseJob `json:"jobs"`
}

type greenhouseJob struct {
	Title          string                 `json:"title"`
	AbsoluteURL    string                 `json:"absolute_url"`
	FirstPublished string                 `json:"first_published"`
	Content        string                 `json:"content"`
	Location       greenhouseLocation     `json:"location"`
	Departments    []greenhouseDepartment `json:"departments"`
}

type greenhouseLocation struct {
	Name string `json:"name"`
}

type greenhouseDepartment struct {
	Name string `json:"name"`
}

func (g *Greenhouse) FetchJobs() ([]Job, error) {
	return fetchBoards("greenhouse", g.Boards, func(board Board) ([]Job, error) {
		var parsed greenhouseResponse

		if err := getJSON(g.HTTPClient, g.UserAgent, g.Endpoint+"/"+board.Slug+"/jobs?content=true", &parsed); err != nil {
			return nil, err
		}

		result := make([]Job, 0, len(parsed.Jobs))

		for _, raw := range parsed.Jobs {
			if raw.AbsoluteURL == "" || raw.Title == "" {
				continue
			}

			result = append(result, raw.toJob(board.Company))
		}

		return result, nil
	})
}

func (raw greenhouseJob) toJob(company string) Job {
	tags := make([]string, 0, len(raw.Departments))

	for _, department := range raw.Departments {
		tags = append(tags, department.Name)
	}

	job := Job{
		Title:         strings.TrimSpace(raw.Title),
		Company:       company,
		Location:      raw.Location.Name,
		WorkplaceType: workplaceFromLocation(raw.Location.Name),
		Tags:          utils.CleanTags(tags),
		URL:           raw.AbsoluteURL,
		Description:   utils.StripHTML(raw.Content),
	}

	// first_published is the original post date; updated_at moves on any edit, so it isn't used.
	if postedAt, err := time.Parse(time.RFC3339, raw.FirstPublished); err == nil {
		job.PostedAt = postedAt
	}

	return job
}
