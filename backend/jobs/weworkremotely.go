package jobs

import (
	"main/utils"
	"strings"
	"time"
)

const weWorkRemotelyEndpoint = "https://weworkremotely.com/remote-jobs.rss"

var _ JobSource = (*WeWorkRemotely)(nil)

type WeWorkRemotely struct {
	feed
}

func NewWeWorkRemotely(userAgent string) *WeWorkRemotely {
	return &WeWorkRemotely{newFeed(userAgent, weWorkRemotelyEndpoint)}
}

type weWorkRemotelyFeed struct {
	Channel struct {
		Items []weWorkRemotelyItem `xml:"item"`
	} `xml:"channel"`
}

type weWorkRemotelyItem struct {
	Title       string `xml:"title"`
	Region      string `xml:"region"`
	Country     string `xml:"country"`
	State       string `xml:"state"`
	Skills      string `xml:"skills"`
	Category    string `xml:"category"`
	Type        string `xml:"type"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Link        string `xml:"link"`
}

func (w *WeWorkRemotely) FetchJobs() ([]Job, error) {
	var parsed weWorkRemotelyFeed

	if err := w.getXML(w.Endpoint, &parsed); err != nil {
		return nil, err
	}

	result := make([]Job, 0, len(parsed.Channel.Items))

	for _, raw := range parsed.Channel.Items {
		if raw.Link == "" || raw.Title == "" {
			continue
		}

		result = append(result, raw.toJob())
	}

	return result, nil
}

func (raw weWorkRemotelyItem) toJob() Job {
	company := ""
	title := raw.Title

	if idx := strings.Index(raw.Title, ": "); idx != -1 {
		company = raw.Title[:idx]
		title = raw.Title[idx+2:]
	}

	location := raw.State

	if location == "" {
		location = raw.Country
	}

	if location == "" {
		location = raw.Region
	}

	var tags []string

	if raw.Category != "" {
		tags = append(tags, raw.Category)
	}

	if raw.Type != "" {
		tags = append(tags, raw.Type)
	}

	for _, skill := range strings.Split(raw.Skills, ",") {
		if trimmed := strings.TrimSpace(skill); trimmed != "" {
			tags = append(tags, trimmed)
		}
	}

	job := Job{
		Title:         title,
		Company:       company,
		Location:      location,
		WorkplaceType: Remote,
		Tags:          utils.CleanTags(tags),
		URL:           raw.Link,
		Description:   utils.StripHTML(raw.Description),
	}

	if postedAt, err := time.Parse(time.RFC1123Z, raw.PubDate); err == nil {
		job.PostedAt = postedAt
	}

	return job
}
