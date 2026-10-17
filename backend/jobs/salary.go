package jobs

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// periodsPerYear converts a pay rate to a yearly salary.
var periodsPerYear = map[string]float64{
	"YEAR":  1,
	"MONTH": 12,
	"WEEK":  52,
	"DAY":   260,
	"HOUR":  2080,
}

// normalizePeriod reads pay periods like "hourly", "1 YEAR", or "per-hour-wage"; empty means yearly.
func normalizePeriod(period string) (string, bool) {
	p := strings.ToLower(strings.TrimSpace(period))

	switch {
	case p == "":
		return "YEAR", true
	case strings.Contains(p, "hour") || p == "hr":
		return "HOUR", true
	case strings.Contains(p, "day") || p == "daily":
		return "DAY", true
	case strings.Contains(p, "week") || p == "wk":
		return "WEEK", true
	case strings.Contains(p, "month") || p == "mo":
		return "MONTH", true
	case strings.Contains(p, "year") || strings.Contains(p, "annu") || p == "yr":
		return "YEAR", true
	}

	return "", false
}

// yearly rounds a yearly amount, nil when too small to be a salary or too big for the column.
func yearly(amount float64) *int {
	if amount < MinSalary || amount > math.MaxInt32 {
		return nil
	}

	n := int(math.Round(amount))

	return &n
}

// YearlySalary converts a pay range at period to yearly amounts. Non-USD currencies and unknown periods give nil.
func YearlySalary(low, high float64, period, currency string) (*int, *int) {
	if c := strings.ToUpper(strings.TrimSpace(currency)); c != "" && c != "USD" {
		return nil, nil
	}

	unit, ok := normalizePeriod(period)
	if !ok {
		return nil, nil
	}

	return yearly(low * periodsPerYear[unit]), yearly(high * periodsPerYear[unit])
}

// moneyPattern matches "$95,000", "US$34.32", "USD 120k"; pre holds a prefix like "CA" in "CA$".
func moneyPattern(n string, markerOptional bool) string {
	marker := `(?P<cur` + n + `>\b(?P<pre` + n + `>[a-z]{1,2})\$|\$|\busd\s?)`
	if markerOptional {
		marker += "?"
	}

	return marker + `\s?(?P<num` + n + `>\d[\d,]*(?:\.\d+)?)(?:\s?(?P<mul` + n + `>k|mm|m|b|thousand|million|billion)\b)?`
}

var (
	payRange = regexp.MustCompile(`(?i)` + moneyPattern("1", false) +
		`(?:\s*(?P<sep>-|–|—|to|and)\s*` + moneyPattern("2", true) + `)?`)
	foreignCurrency = regexp.MustCompile(`(?i)[€£¥₹]|\b(?:eur|gbp|cad|aud|nzd|sgd|hkd|inr|jpy|chf|sek|nok|dkk|pln|brl|mxn)\b`)
	foreignCodeNext = regexp.MustCompile(`(?i)^\s*\(?(?:eur|gbp|cad|aud|nzd|sgd|hkd|inr|jpy|chf|sek|nok|dkk|pln|brl|mxn)\b`)
	periodAfter     = regexp.MustCompile(`(?i)^[\s,]*(?:usd\b\s*)?(?:(?:/|per\b|an?\b|each\b)\s*(hour|hr|year|yr|annum|month|mo|week|wk|day)\b|(hourly|annually|yearly|monthly|weekly|daily)\b)`)
	periodBefore    = regexp.MustCompile(`(?i)\b(hourly|monthly|weekly|daily|annual|yearly)\s+(?:base\s+)?(?:pay|rate|wage|salary|compensation|range)`)
	payWord         = regexp.MustCompile(`(?i)\b(?:salary|salaries|pay|compensation|base|wages?|ote|earnings|rate|range)\b`)
)

const (
	// payWordWindow is how far before a range a pay word may sit in strict mode.
	payWordWindow = 150
	// periodWindow is how far around a range its pay period is looked for.
	periodWindow = 40
	// Plausible yearly salaries from text; outside this is a misread number.
	minTextSalary = 10_000
	maxTextSalary = 2_000_000
)

// ParseSalaryText reads USD pay like "$50k - $70k" or "$34.32 – $42.13 per hour" as a yearly range.
// strict is for descriptions: only ranges after a pay word count, so a "$5,000 stipend" doesn't.
func ParseSalaryText(text string, strict bool) (*int, *int) {
	// Dedicated salary fields often leave off "$".
	if !strict && !strings.Contains(text, "$") && !foreignCurrency.MatchString(text) {
		text = "$" + text
	}

	var low, high float64

	for _, m := range payRange.FindAllStringSubmatchIndex(text, -1) {
		lo, hi, ok := readPayRange(text, m, strict)
		if !ok {
			continue
		}

		if low == 0 || lo < low {
			low = lo
		}
		high = max(high, hi)
	}

	if high == 0 {
		return nil, nil
	}

	return yearly(low), yearly(high)
}

// readPayRange turns one payRange match into a yearly range, combining geo tiers is left to the caller.
func readPayRange(text string, m []int, strict bool) (float64, float64, bool) {
	group := func(name string) string {
		i := payRange.SubexpIndex(name)
		if m[2*i] < 0 {
			return ""
		}
		return text[m[2*i]:m[2*i+1]]
	}

	before := text[max(0, m[0]-payWordWindow):m[0]]
	after := text[m[1]:min(len(text), m[1]+periodWindow)]

	if isForeignPrefix(group("pre1")) || foreignCodeNext.MatchString(after) {
		return 0, 0, false
	}

	first, ok := amount(group("num1"), group("mul1"))
	if !ok {
		return 0, 0, false
	}

	// "$80,000 and 401k" isn't a range; "between $80,000 and $90,000" is.
	hasSecond := group("num2") != "" && !(strings.EqualFold(group("sep"), "and") && group("cur2") == "")

	var second float64

	if hasSecond {
		if isForeignPrefix(group("pre2")) {
			return 0, 0, false
		}

		if second, ok = amount(group("num2"), group("mul2")); !ok {
			return 0, 0, false
		}

		// "$100-150k" means $100k-$150k.
		if group("mul1") == "" && group("mul2") != "" && first < 1000 {
			first *= 1000
		}
	}

	perYear := periodsPerYear[payPeriod(before, after)]
	low, high := first*perYear, first*perYear

	if !plausible(low) {
		return 0, 0, false
	}

	if s := second * perYear; hasSecond && plausible(s) {
		low, high = min(low, s), max(high, s)
	} else {
		hasSecond = false
	}

	if strict && (!hasSecond || !payWord.MatchString(before)) {
		return 0, 0, false
	}

	return low, high, true
}

func isForeignPrefix(prefix string) bool {
	return prefix != "" && !strings.EqualFold(prefix, "us")
}

// amount parses "120,000" with an optional "k"; millions and billions aren't salaries.
func amount(num, mul string) (float64, bool) {
	n, err := strconv.ParseFloat(strings.ReplaceAll(num, ",", ""), 64)
	if err != nil {
		return 0, false
	}

	switch strings.ToLower(mul) {
	case "":
		return n, true
	case "k", "thousand":
		return n * 1000, true
	}

	return 0, false
}

// payPeriod reads "per hour" after a range, or "hourly pay" just before it; yearly otherwise.
func payPeriod(before, after string) string {
	if match := periodAfter.FindStringSubmatch(after); match != nil {
		if unit, ok := normalizePeriod(match[1] + match[2]); ok {
			return unit
		}
	}

	near := before[max(0, len(before)-periodWindow):]
	if matches := periodBefore.FindAllStringSubmatch(near, -1); len(matches) > 0 {
		if unit, ok := normalizePeriod(matches[len(matches)-1][1]); ok {
			return unit
		}
	}

	return "YEAR"
}

func plausible(yearly float64) bool {
	return yearly >= minTextSalary && yearly <= maxTextSalary
}
