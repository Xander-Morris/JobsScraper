package jobs

import (
	"encoding/xml"
	"fmt"
	"main/utils"
	"net/http"
	"strings"
	"time"
)

const weWorkRemotelyEndpoint = "https://weworkremotely.com/remote-jobs.rss"

var _ JobSource = (*WeWorkRemotely)(nil)

type WeWorkRemotely struct {
	HTTPClient *http.Client
	UserAgent  string
	Endpoint   string
}

func NewWeWorkRemotely(userAgent string) *WeWorkRemotely {
	return &WeWorkRemotely{
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		UserAgent:  userAgent,
		Endpoint:   weWorkRemotelyEndpoint,
	}
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
	req, err := http.NewRequest("GET", w.Endpoint, nil)

	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("User-Agent", w.UserAgent)
	resp, err := w.HTTPClient.Do(req)

	if err != nil {
		return nil, fmt.Errorf("fetch weworkremotely feed: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weworkremotely feed returned status %d", resp.StatusCode)
	}

	var parsed weWorkRemotelyFeed

	if err := xml.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode weworkremotely feed: %w", err)
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
