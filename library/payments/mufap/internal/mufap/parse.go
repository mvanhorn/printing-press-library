// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// Package mufap parses MUFAP's server-rendered HTML tables and derives the
// series the site itself never publishes.
//
// MUFAP renders one table per page with a real <thead>, and every data row
// carries exactly len(headers) <td> cells. That regularity holds across all
// five daily tabs and the monthly page, so one header-driven parser serves
// every surface instead of six positional ones that would drift apart.
package mufap

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/cliutil"
)

var (
	reTheadBlock = regexp.MustCompile(`(?is)<thead[^>]*>(.*?)</thead>`)
	reTh         = regexp.MustCompile(`(?is)<th[^>]*>(.*?)</th>`)
	reTr         = regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
	reTd         = regexp.MustCompile(`(?is)<td[^>]*>(.*?)</td>`)
	reValidity   = regexp.MustCompile(`^[A-Z][a-z]{2} \d{2}, \d{4}$`)
	reNumber     = regexp.MustCompile(`^-?[\d,]*\.?\d+$`)
)

// Table is one parsed HTML table: its header labels and its data rows.
type Table struct {
	Headers []string            `json:"headers"`
	Rows    []map[string]string `json:"rows"`
}

// ParseTable extracts the first <thead>-bearing table from a MUFAP page.
//
// Rows are matched on cell count rather than on position within the document
// because MUFAP concatenates hidden sibling tables (search panes, export
// scaffolding) into the same markup; those rows carry a different cell count
// and are skipped rather than silently mis-keyed.
func ParseTable(doc string) (*Table, error) {
	heads := reTheadBlock.FindStringSubmatch(doc)
	if heads == nil {
		return nil, fmt.Errorf("no <thead> found: page shape changed, or the response was a challenge page")
	}
	var headers []string
	for _, m := range reTh.FindAllStringSubmatch(heads[1], -1) {
		headers = append(headers, foldSpaces(cliutil.CleanText(stripTags(m[1]))))
	}
	if len(headers) == 0 {
		return nil, fmt.Errorf("<thead> present but held no <th> cells")
	}
	t := &Table{Headers: headers, Rows: make([]map[string]string, 0)}
	for _, tr := range reTr.FindAllStringSubmatch(doc, -1) {
		cells := reTd.FindAllStringSubmatch(tr[1], -1)
		if len(cells) != len(headers) {
			continue
		}
		row := make(map[string]string, len(headers))
		for i, c := range cells {
			row[headers[i]] = foldSpaces(cliutil.CleanText(stripTags(c[1])))
		}
		t.Rows = append(t.Rows, row)
	}
	return t, nil
}

var reTag = regexp.MustCompile(`(?s)<[^>]+>`)

func stripTags(s string) string {
	return strings.TrimSpace(reTag.ReplaceAllString(s, " "))
}

// foldSpaces collapses runs of whitespace, including the U+00A0 that
// cliutil.CleanText leaves behind after decoding &nbsp;, into single spaces.
// Fund names are join keys, so a non-breaking space inside one silently
// splits a fund into two distinct series.
var reSpaces = regexp.MustCompile(`[\s\x{00a0}]+`)

func foldSpaces(s string) string {
	return strings.TrimSpace(reSpaces.ReplaceAllString(s, " "))
}

// ParseNumber converts a MUFAP display number to a float.
//
// MUFAP writes negative returns in ACCOUNTING NOTATION -- "(4.97)" means
// -4.97 -- and never with a leading minus. Measured on 2026-09-04, tab=returns:
// 96 of 388 YTD cells were parenthesized and 0 carried a minus sign. Treating
// a parenthesized cell as unparseable silently drops every losing fund from
// the cross-section, which biases each aggregate upward by exactly the funds
// that fell: it cut the equity universe from 91 to 17 before this was fixed.
//
// Returns ok=false for "-", "N/A", "Not Published" and the empty string, all
// of which MUFAP uses for "this fund did not report". Callers must not coerce
// those to zero: a non-reporting fund is not a fund holding zero, and a fund
// that lost 5% is not a fund that did not report.
func ParseNumber(s string) (float64, bool) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	s = strings.TrimSuffix(s, "%")
	s = strings.TrimSpace(s)
	negative := false
	if len(s) > 1 && strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		negative = true
		s = strings.TrimSpace(s[1 : len(s)-1])
		s = strings.TrimSuffix(s, "%")
	}
	if s == "" || !reNumber.MatchString(s) {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	if negative {
		v = -v
	}
	return v, true
}

// IsValidityDate reports whether a cell holds MUFAP's "Mon DD, YYYY" date form.
func IsValidityDate(s string) bool { return reValidity.MatchString(strings.TrimSpace(s)) }

var monthNames = map[string]string{
	"Jan": "01", "Feb": "02", "Mar": "03", "Apr": "04", "May": "05", "Jun": "06",
	"Jul": "07", "Aug": "08", "Sep": "09", "Oct": "10", "Nov": "11", "Dec": "12",
}

// NormalizeValidityDate converts "Sep 04, 2026" to "2026-09-04".
//
// Every stored date is this ISO form so the panel joins directly against other
// dated research tables; MUFAP's display form sorts lexically wrong.
func NormalizeValidityDate(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if !reValidity.MatchString(s) {
		return "", false
	}
	mm, ok := monthNames[s[:3]]
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%s-%s-%s", s[8:12], mm, s[4:6]), true
}

// MonthKey converts an ISO date or YYYY-MM to MUFAP's "M-YYYY" allocation form.
//
// The allocation endpoint rejects both ISO dates and zero-padded months, and
// signals the rejection as HTTP 500 or as HTTP 200 with an empty table rather
// than as a parameter error, so the conversion happens here once.
func MonthKey(s string) (string, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, "-")
	if len(parts) < 2 {
		return "", fmt.Errorf("month %q: want YYYY-MM or YYYY-MM-DD", s)
	}
	y, err := strconv.Atoi(parts[0])
	if err != nil || y < 1990 || y > 2200 {
		return "", fmt.Errorf("month %q: year out of range", s)
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 1 || m > 12 {
		return "", fmt.Errorf("month %q: month out of range", s)
	}
	return fmt.Sprintf("%d-%d", m, y), nil
}
