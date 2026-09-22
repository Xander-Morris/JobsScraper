package jobs

import (
	"fmt"
	"main/utils"
	"strings"
	"time"
)

const leverEndpoint = "https://api.lever.co/v0/postings"

// leverBoards are the companies whose public Lever boards get scraped.
var leverBoards = []Board{
	{"outreach", "Outreach"}, {"palantir", "Palantir"}, {"spotify", "Spotify"}, {"zoox", "Zoox"},
}

var _ JobSource = (*Lever)(nil)

type Lever struct {
	feed
	Boards []Board
}

func NewLever(userAgent string) *Lever {
	return &Lever{
		feed:   newBoardFeed(userAgent, leverEndpoint),
		Boards: leverBoards,
	}
}

type leverJob struct {
	Text             string          `json:"text"`
	HostedURL        string          `json:"hostedUrl"`
	CreatedAt        int64           `json:"createdAt"`
	WorkplaceType    string          `json:"workplaceType"`
	DescriptionPlain string          `json:"descriptionPlain"`
	AdditionalPlain  string          `json:"additionalPlain"`
	Lists            []leverList     `json:"lists"`
	Categories       leverCategories `json:"categories"`
}

type leverList struct {
	Text    string `json:"text"`
	Content string `json:"content"`
}

type leverCategories struct {
	Commitment string `json:"commitment"`
	Location   string `json:"location"`
	Team       string `json:"team"`
}

func (l *Lever) FetchJobs() ([]Job, error) {
	return fetchBoards("lever", l.Boards, func(board Board) ([]Job, error) {
		var parsed []leverJob

		if err := l.getJSON(l.Endpoint+"/"+board.Slug+"?mode=json", &parsed); err != nil {
			return nil, err
		}

		result := make([]Job, 0, len(parsed))

		for _, raw := range parsed {
			if raw.HostedURL == "" || raw.Text == "" {
				continue
			}

			result = append(result, raw.toJob(board.Company))
		}

		return result, nil
	})
}

func (raw leverJob) toJob(company string) Job {
	workplaceType, _ := ParseWorkplaceType(raw.WorkplaceType)

	if workplaceType == Unknown {
		workplaceType = workplaceFromLocation(raw.Categories.Location)
	}

	// Lever splits a posting into an intro, titled lists (requirements, benefits), and a closing section.
	var description strings.Builder
	description.WriteString(raw.DescriptionPlain)

	for _, list := range raw.Lists {
		fmt.Fprintf(&description, "\n\n%s\n%s", list.Text, utils.StripHTML(list.Content))
	}

	if raw.AdditionalPlain != "" {
		description.WriteString("\n\n" + raw.AdditionalPlain)
	}

	job := Job{
		Title:         strings.TrimSpace(raw.Text),
		Company:       company,
		Location:      raw.Categories.Location,
		WorkplaceType: workplaceType,
		Tags:          utils.CleanTags([]string{raw.Categories.Team, raw.Categories.Commitment}),
		URL:           raw.HostedURL,
		Description:   strings.TrimSpace(description.String()),
	}

	if raw.CreatedAt > 0 {
		job.PostedAt = time.UnixMilli(raw.CreatedAt)
	}

	return job
}
