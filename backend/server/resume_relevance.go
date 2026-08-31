package server

import (
	"regexp"
	"strings"

	"main/database"
)

const maxResumeQueryTerms = 40

var termSplitPattern = regexp.MustCompile(`[,;:|/&]+|\band\b`)

func buildResumeSearchQuery(extraction database.ResumeExtraction) string {
	var terms []string

	for _, skill := range extraction.Skills {
		terms = append(terms, splitTerms(skill)...)
	}

	for _, edu := range extraction.Education {
		if edu.Major != "" {
			terms = append(terms, edu.Major)
		}
	}

	for _, we := range extraction.WorkExperience {
		if we.JobTitle != "" {
			terms = append(terms, we.JobTitle)
		}
	}

	for _, p := range extraction.Projects {
		terms = append(terms, splitTerms(strings.Join(p.Technologies, ","))...)
	}

	seen := make(map[string]bool, len(terms))
	unique := make([]string, 0, min(len(terms), maxResumeQueryTerms))

	for _, term := range terms {
		key := strings.ToLower(term)
		if key == "" || seen[key] {
			continue
		}

		seen[key] = true
		unique = append(unique, term)

		if len(unique) >= maxResumeQueryTerms {
			break
		}
	}

	return strings.Join(unique, " OR ")
}

func splitTerms(s string) []string {
	var out []string

	for _, part := range termSplitPattern.Split(s, -1) {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}

	return out
}
