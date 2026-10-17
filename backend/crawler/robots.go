package crawler

import (
	"bufio"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// robots is the part of a robots.txt that applies to us.
type robots struct {
	rules      []robotsRule
	crawlDelay time.Duration
	sitemaps   []string
}

type robotsRule struct {
	allow   bool
	length  int
	pattern *regexp.Regexp
}

type robotsGroup struct {
	agents     []string
	rules      []robotsRule
	crawlDelay time.Duration
}

// parseRobots follows RFC 9309: the group naming agent (a product token like "jobsscraper") wins over "*",
// and the longest matching rule wins.
func parseRobots(r io.Reader, agent string) robots {
	agent = strings.ToLower(agent)

	var (
		groups   []*robotsGroup
		current  *robotsGroup
		sitemaps []string
		inAgents bool
	)

	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line, _, _ := strings.Cut(scanner.Text(), "#")
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}

		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)

		switch key {
		case "user-agent":
			if !inAgents {
				current = &robotsGroup{}
				groups = append(groups, current)
			}
			current.agents = append(current.agents, strings.ToLower(value))
			inAgents = true
		case "allow", "disallow":
			inAgents = false
			// An empty Disallow allows everything, so it adds no rule.
			if current != nil && value != "" {
				current.rules = append(current.rules, newRobotsRule(key == "allow", value))
			}
		case "crawl-delay":
			inAgents = false
			if seconds, err := strconv.ParseFloat(value, 64); current != nil && err == nil && seconds > 0 {
				current.crawlDelay = time.Duration(seconds * float64(time.Second))
			}
		case "sitemap":
			sitemaps = append(sitemaps, value)
		}
	}

	return robots{sitemaps: sitemaps}.withGroups(pickGroups(groups, agent))
}

// pickGroups returns the groups naming agent, or the "*" groups if none do.
func pickGroups(groups []*robotsGroup, agent string) []*robotsGroup {
	var named, wildcard []*robotsGroup

	for _, group := range groups {
		for _, name := range group.agents {
			if name == "*" {
				wildcard = append(wildcard, group)
				break
			}
			if name == agent {
				named = append(named, group)
				break
			}
		}
	}

	if len(named) > 0 {
		return named
	}

	return wildcard
}

func (r robots) withGroups(groups []*robotsGroup) robots {
	for _, group := range groups {
		r.rules = append(r.rules, group.rules...)
		r.crawlDelay = max(r.crawlDelay, group.crawlDelay)
	}

	return r
}

func newRobotsRule(allow bool, path string) robotsRule {
	anchored := strings.HasSuffix(path, "$")
	pattern := regexp.QuoteMeta(strings.TrimSuffix(path, "$"))
	pattern = "^" + strings.ReplaceAll(pattern, `\*`, ".*")

	if anchored {
		pattern += "$"
	}

	return robotsRule{allow: allow, length: len(path), pattern: regexp.MustCompile(pattern)}
}

// allowed reports whether path (with its query) may be fetched.
func (r robots) allowed(path string) bool {
	best := -1
	allow := true

	for _, rule := range r.rules {
		if !rule.pattern.MatchString(path) {
			continue
		}

		// Ties go to Allow.
		if rule.length > best || (rule.length == best && rule.allow) {
			best = rule.length
			allow = rule.allow
		}
	}

	return allow
}
