// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"regexp"
	"strconv"
	"strings"
)

// Only one pay-transparency shape is parsed (228 of 301 salary texts on
// 2026-10-05): "[For <place>-based roles: ]The base salary range for this
// role is USD $X per year - USD $Y per year". Anything else stays unparsed:
// salary_text keeps the words and the numeric fields stay null.
var reSalary = regexp.MustCompile(`(?i)(?:for ([^:]{2,120}?)-based roles:\s*)?the base salary range for this role is ([A-Z]{3}) \$?([0-9][0-9,]*(?:\.[0-9]+)?) per (year|hour|month|week)\s*[-–]\s*([A-Z]{3}) \$?([0-9][0-9,]*(?:\.[0-9]+)?) per (year|hour|month|week)`)

// ParseSalaryRanges extracts every range sentence from plain salary text.
func ParseSalaryRanges(text string) []SalaryRange {
	matches := reSalary.FindAllStringSubmatch(text, -1)
	out := make([]SalaryRange, 0, len(matches))
	for _, m := range matches {
		if !strings.EqualFold(m[2], m[5]) || !strings.EqualFold(m[4], m[7]) {
			continue
		}
		lo, err1 := strconv.ParseFloat(strings.ReplaceAll(m[3], ",", ""), 64)
		hi, err2 := strconv.ParseFloat(strings.ReplaceAll(m[6], ",", ""), 64)
		if err1 != nil || err2 != nil {
			continue
		}
		out = append(out, SalaryRange{
			Location: strPtr(rangeLocation(m[1])),
			Currency: strings.ToUpper(m[2]),
			Min:      lo,
			Max:      hi,
			Period:   strings.ToLower(m[4]),
		})
	}
	return out
}

// applySingleRange fills the scalar salary fields only when every parsed
// range agrees; multi-city postings with different ranges keep them null and
// carry the detail in salary_ranges.
func applySingleRange(p *Posting) {
	if len(p.SalaryRanges) == 0 {
		return
	}
	first := p.SalaryRanges[0]
	for _, r := range p.SalaryRanges[1:] {
		if r.Currency != first.Currency || r.Min != first.Min || r.Max != first.Max || r.Period != first.Period {
			return
		}
	}
	lo, hi, cur, per := first.Min, first.Max, first.Currency, first.Period
	p.SalaryMin, p.SalaryMax, p.SalaryCurrency, p.SalaryPeriod = &lo, &hi, &cur, &per
}

// rangeLocation trims a captured "<place>-based roles" location to the text
// after its last "for ": the regex starts the capture at the leftmost "for",
// which can sit in an earlier sentence ("...a bonus. For Seattle, WA").
func rangeLocation(loc string) string {
	if i := strings.LastIndex(strings.ToLower(loc), "for "); i >= 0 {
		loc = loc[i+len("for "):]
	}
	return loc
}
