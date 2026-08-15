package utils

import (
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

var (
	unicodeEscape = regexp.MustCompile(`\\u[0-9a-fA-F]{4}`)

	lineBreakTag   = regexp.MustCompile(`(?i)<br\s*/?>`)
	listItemOpen   = regexp.MustCompile(`(?i)<li[^>]*>`)
	blockOpen      = regexp.MustCompile(`(?i)<(p|div|h[1-6]|tr)(\s[^>]*)?>`)
	blockClose     = regexp.MustCompile(`(?i)</(p|div|h[1-6]|tr|li|ul|ol)>`)
	excessNewlines = regexp.MustCompile(`\n{3,}`)
)

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

	withBreaks := lineBreakTag.ReplaceAllString(decoded, "\n")
	withBreaks = listItemOpen.ReplaceAllString(withBreaks, "\n• ")
	withBreaks = blockOpen.ReplaceAllString(withBreaks, "\n\n")
	withBreaks = blockClose.ReplaceAllString(withBreaks, "\n\n")

	stripped := bluemonday.StrictPolicy().Sanitize(withBreaks)

	plain := html.UnescapeString(stripped)

	return normalizeWhitespace(plain)
}

func normalizeWhitespace(input string) string {
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}

	collapsed := excessNewlines.ReplaceAllString(strings.Join(lines, "\n"), "\n\n")

	return strings.TrimSpace(collapsed)
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
