package jobs

import (
	"encoding/json"
	"strings"
	"time"
)

type WorkplaceType int

const (
	Unknown WorkplaceType = iota
	Remote
	Hybrid
	InPerson
)

func ParseWorkplaceType(s string) (WorkplaceType, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return Unknown, true
	case "remote":
		return Remote, true
	case "hybrid":
		return Hybrid, true
	case "in_person", "in-person", "inperson", "onsite", "on-site", "on_site":
		return InPerson, true
	default:
		return Unknown, false
	}
}

// workplaceFromLocation infers Remote from free-text locations like "Remote - US".
func workplaceFromLocation(location string) WorkplaceType {
	if strings.Contains(strings.ToLower(location), "remote") {
		return Remote
	}

	return Unknown
}

func (w WorkplaceType) String() string {
	switch w {
	case Remote:
		return "remote"
	case Hybrid:
		return "hybrid"
	case InPerson:
		return "in_person"
	default:
		return "unknown"
	}
}

func (w WorkplaceType) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.String())
}

func (w *WorkplaceType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	parsed, _ := ParseWorkplaceType(s)
	*w = parsed

	return nil
}

type Job struct {
	ID            int64         `json:"id"`
	Title         string        `json:"title"`
	Company       string        `json:"company"`
	Location      string        `json:"location"`
	WorkplaceType WorkplaceType `json:"workplace_type"`
	Tags          []string      `json:"tags"`
	SalaryMin     *int          `json:"salary_min"`
	SalaryMax     *int          `json:"salary_max"`
	PostedAt      time.Time     `json:"posted_at"`
	URL           string        `json:"url"`
	// Description is empty in search results; only job detail loads it.
	Description string `json:"description,omitempty"`
	// MatchScore is a text-relevance score (roughly 0-1) against the caller's active
	// resume, nil when there's no active resume to compare against.
	MatchScore *float64 `json:"match_score,omitempty"`
	// Applied is whether the caller has marked this job as applied to. Always false
	// for anonymous callers.
	Applied bool `json:"applied"`
}
