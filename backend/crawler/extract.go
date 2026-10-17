package crawler

import (
	"encoding/json"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"main/jobs"
	"main/utils"

	"golang.org/x/net/html"
)

// page is what one fetched HTML page yields.
type page struct {
	postings []jobPosting
	links    []string
}

// parsePage pulls JSON-LD JobPostings and same-document links out of an HTML page at base.
func parsePage(r io.Reader, base *url.URL) (page, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return page{}, err
	}

	var result page

	for node := range doc.Descendants() {
		if node.Type != html.ElementNode {
			continue
		}

		switch node.Data {
		case "script":
			if strings.Contains(strings.ToLower(attr(node, "type")), "ld+json") && node.FirstChild != nil {
				result.postings = append(result.postings, findPostings(node.FirstChild.Data)...)
			}
		case "a":
			if link := resolveLink(base, attr(node, "href")); link != "" {
				result.links = append(result.links, link)
			}
		}
	}

	return result, nil
}

func attr(node *html.Node, name string) string {
	for _, a := range node.Attr {
		if a.Key == name {
			return a.Val
		}
	}

	return ""
}

// resolveLink makes href absolute and drops its fragment; empty for non-http links.
func resolveLink(base *url.URL, href string) string {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") {
		return ""
	}

	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}

	resolved := base.ResolveReference(ref)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return ""
	}

	resolved.Fragment = ""

	return resolved.String()
}

// jobPosting is a schema.org JobPosting. Most fields come in several shapes, so they stay raw.
type jobPosting struct {
	Title                         json.RawMessage `json:"title"`
	Name                          json.RawMessage `json:"name"`
	Description                   json.RawMessage `json:"description"`
	DatePosted                    json.RawMessage `json:"datePosted"`
	ValidThrough                  json.RawMessage `json:"validThrough"`
	HiringOrganization            json.RawMessage `json:"hiringOrganization"`
	JobLocation                   json.RawMessage `json:"jobLocation"`
	JobLocationType               json.RawMessage `json:"jobLocationType"`
	ApplicantLocationRequirements json.RawMessage `json:"applicantLocationRequirements"`
	EmploymentType                json.RawMessage `json:"employmentType"`
	BaseSalary                    json.RawMessage `json:"baseSalary"`
	Industry                      json.RawMessage `json:"industry"`
	OccupationalCategory          json.RawMessage `json:"occupationalCategory"`
}

// findPostings decodes a JSON-LD block and returns every JobPosting in it, including inside @graph.
func findPostings(script string) []jobPosting {
	data := []byte(strings.TrimSpace(script))

	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		// Some sites emit raw newlines inside strings, which JSON forbids.
		cleaned := strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(string(data))
		if err := json.Unmarshal([]byte(cleaned), &root); err != nil {
			return nil
		}
	}

	var postings []jobPosting

	var walk func(node any)
	walk = func(node any) {
		switch v := node.(type) {
		case []any:
			for _, item := range v {
				walk(item)
			}
		case map[string]any:
			if isJobPosting(v["@type"]) {
				if posting, ok := toPosting(v); ok {
					postings = append(postings, posting)
				}
				return
			}
			walk(v["@graph"])
		}
	}
	walk(root)

	return postings
}

func isJobPosting(t any) bool {
	switch v := t.(type) {
	case string:
		return v == "JobPosting" || strings.HasSuffix(v, "/JobPosting")
	case []any:
		for _, item := range v {
			if isJobPosting(item) {
				return true
			}
		}
	}

	return false
}

func toPosting(v map[string]any) (jobPosting, bool) {
	data, err := json.Marshal(v)
	if err != nil {
		return jobPosting{}, false
	}

	var posting jobPosting

	return posting, json.Unmarshal(data, &posting) == nil
}

var titleWorkplace = regexp.MustCompile(`(?i)\b(hybrid|on-?site|in[- ]office)\b`)

// maxDescriptionRunes keeps one huge posting from eating the free-tier database.
const maxDescriptionRunes = 20000

// toJob maps a posting found at pageURL; company overrides hiringOrganization when set.
// ok is false when the posting has no title or is past its validThrough date.
func (p jobPosting) toJob(pageURL, company string, now time.Time) (jobs.Job, bool) {
	title := text(p.Title)
	if title == "" {
		title = text(p.Name)
	}

	if title == "" {
		return jobs.Job{}, false
	}

	if validThrough := parseDate(text(p.ValidThrough)); !validThrough.IsZero() && validThrough.Before(now) {
		return jobs.Job{}, false
	}

	if company == "" {
		company = text(p.HiringOrganization)
	}

	location := placesText(p.JobLocation)
	workplace := jobs.WorkplaceFromLocation(location)

	if strings.EqualFold(text(p.JobLocationType), "TELECOMMUTE") {
		workplace = jobs.Remote

		if location == "" {
			location = "Remote"
			if regions := strings.Join(texts(p.ApplicantLocationRequirements), ", "); regions != "" {
				location += " - " + regions
			}
		}
	}

	// Some sites mark every posting TELECOMMUTE, so a title saying otherwise wins.
	if match := titleWorkplace.FindStringSubmatch(title); match != nil {
		workplace = jobs.InPerson
		if strings.EqualFold(match[1], "hybrid") {
			workplace = jobs.Hybrid
		}
	}

	tags := employmentTags(p.EmploymentType)
	tags = append(tags, texts(p.OccupationalCategory)...)
	tags = append(tags, texts(p.Industry)...)

	job := jobs.Job{
		Title:         strings.TrimSpace(utils.StripHTML(title)),
		Company:       strings.TrimSpace(company),
		Location:      location,
		WorkplaceType: workplace,
		Tags:          utils.CleanTags(tags),
		PostedAt:      parseDate(text(p.DatePosted)),
		URL:           pageURL,
		Description:   truncateRunes(utils.StripHTML(text(p.Description)), maxDescriptionRunes),
	}

	job.SalaryMin, job.SalaryMax = yearlySalary(p.BaseSalary)

	return job, true
}

// text reads a string, a number, an object's name or @value, or an array's first such value.
func text(raw json.RawMessage) string {
	if values := texts(raw); len(values) > 0 {
		return values[0]
	}

	return ""
}

func texts(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}

	var s string
	if json.Unmarshal(raw, &s) == nil {
		if s = strings.TrimSpace(s); s != "" {
			return []string{s}
		}
		return nil
	}

	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return []string{n.String()}
	}

	var list []json.RawMessage
	if json.Unmarshal(raw, &list) == nil {
		var result []string
		for _, item := range list {
			result = append(result, texts(item)...)
		}
		return result
	}

	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) == nil {
		for _, key := range []string{"name", "@value"} {
			if value := text(object[key]); value != "" {
				return []string{value}
			}
		}
	}

	return nil
}

func objects(raw json.RawMessage) []map[string]json.RawMessage {
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) != nil {
		list = []json.RawMessage{raw}
	}

	var result []map[string]json.RawMessage

	for _, item := range list {
		var object map[string]json.RawMessage
		if json.Unmarshal(item, &object) == nil {
			result = append(result, object)
		}
	}

	return result
}

// maxPlaces bounds how many of a posting's locations are listed.
const maxPlaces = 3

// placesText joins jobLocation Places as "locality, region", falling back to country.
func placesText(raw json.RawMessage) string {
	var places []string
	seen := make(map[string]bool)

	for _, place := range objects(raw) {
		name := addressText(place["address"])
		if name == "" {
			name = text(place["name"])
		}

		if name != "" && !seen[name] && len(places) < maxPlaces {
			seen[name] = true
			places = append(places, name)
		}
	}

	return strings.Join(places, "; ")
}

func addressText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}

	addresses := objects(raw)
	if len(addresses) == 0 {
		return ""
	}

	address := addresses[0]
	locality, region := text(address["addressLocality"]), text(address["addressRegion"])

	var parts []string
	if locality != "" {
		parts = append(parts, locality)
	}

	if region != "" && !strings.Contains(locality, region) {
		parts = append(parts, region)
	}

	if len(parts) == 0 {
		return text(address["addressCountry"])
	}

	return strings.Join(parts, ", ")
}

// employmentNames spells schema.org employment types the way the job type filters match them.
var employmentNames = map[string]string{
	"FULL_TIME":  "Full-time",
	"PART_TIME":  "Part-time",
	"CONTRACTOR": "Contract",
	"TEMPORARY":  "Temporary",
	"INTERN":     "Internship",
	"VOLUNTEER":  "Volunteer",
	"PER_DIEM":   "Per diem",
}

func employmentTags(raw json.RawMessage) []string {
	var tags []string

	for _, value := range texts(raw) {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if name, ok := employmentNames[strings.ToUpper(part)]; ok {
				part = name
			}
			if part != "" && part != "OTHER" {
				tags = append(tags, part)
			}
		}
	}

	return tags
}

// yearlySalary reads a USD MonetaryAmount; other currencies are skipped since salary filters assume dollars.
func yearlySalary(raw json.RawMessage) (*int, *int) {
	amounts := objects(raw)
	if len(amounts) == 0 {
		return nil, nil
	}

	amount := amounts[0]
	value := amount["value"]
	unit := text(amount["unitText"])

	var low, high float64

	if n, ok := number(value); ok {
		low, high = n, n
	} else if quantities := objects(value); len(quantities) > 0 {
		quantity := quantities[0]
		if u := text(quantity["unitText"]); u != "" {
			unit = u
		}

		single, _ := number(quantity["value"])
		low, high = single, single

		if n, ok := number(quantity["minValue"]); ok {
			low = n
		}
		if n, ok := number(quantity["maxValue"]); ok {
			high = n
		}
	}

	return jobs.YearlySalary(low, high, unit, text(amount["currency"]))
}

func number(raw json.RawMessage) (float64, bool) {
	var n float64
	if json.Unmarshal(raw, &n) == nil {
		return n, true
	}

	var s string
	if json.Unmarshal(raw, &s) == nil {
		cleaned := strings.NewReplacer(",", "", "$", "", " ", "").Replace(s)
		if parsed, err := strconv.ParseFloat(cleaned, 64); err == nil {
			return parsed, true
		}
	}

	return 0, false
}

func truncateRunes(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}

	return strings.TrimSpace(string(runes[:limit])) + "…"
}
