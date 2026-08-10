package utils

import (
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

var unicodeEscape = regexp.MustCompile(`\\u[0-9a-fA-F]{4}`)

func decodeUnicodeEscapes(input string) string {
	return unicodeEscape.ReplaceAllStringFunc(input, func(m string) string {
		code, err := strconv.ParseUint(m[2:], 16, 32)

		if err != nil {
			return m
		}

		return string(rune(code))
	})
}

func StripHTML(input string) string {
	decoded := html.UnescapeString(decodeUnicodeEscapes(input))
	p := bluemonday.StrictPolicy()

	return p.Sanitize(decoded)
}

func CleanTags(tags []string) []string {
	cleaned := make([]string, 0, len(tags))

	for _, tag := range tags {
		if clean := strings.TrimSpace(html.UnescapeString(decodeUnicodeEscapes(tag))); clean != "" {
			cleaned = append(cleaned, clean)
		}
	}

	return cleaned
}
