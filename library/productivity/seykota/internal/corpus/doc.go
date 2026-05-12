// Copyright 2026 kjuju600. Licensed under Apache-2.0. See LICENSE.

// Package corpus defines the document model shared by the crawler, the
// SQLite store, and the read commands. A Doc is one page from seykota.com:
// a FAQ month-page, a Trading System Project section, or the risk essay
// (optionally split into sections).
package corpus

// Sources.
const (
	SourceFAQ  = "faq"
	SourceTSP  = "tsp"
	SourceRisk = "risk"
)

// Doc is one indexed page (or page section) from seykota.com.
type Doc struct {
	ID           string   `json:"id"`             // url path, e.g. "tt/2023/JAN/01-31/default.html"
	Source       string   `json:"source"`         // faq | tsp | risk
	URL          string   `json:"url"`            // full https URL
	Title        string   `json:"title"`          // <title> or derived heading
	Year         string   `json:"year,omitempty"` // FAQ
	Month        string   `json:"month,omitempty"`// FAQ folder, e.g. "JAN" or "Jan"
	MonthN       int      `json:"month_n,omitempty"` // 1-12, sort key
	Range        string   `json:"range,omitempty"`// FAQ day-range folder, e.g. "01-31"
	Slug         string   `json:"slug,omitempty"` // TSP section slug
	Updated      string   `json:"updated,omitempty"` // TSP last-updated string
	Section      string   `json:"section,omitempty"` // risk essay heading
	Ord          int      `json:"ord,omitempty"`  // ordering within source
	Contributors []string `json:"contributors,omitempty"` // FAQ contributor names (best effort)
	Body         string   `json:"body"`           // cleaned plain text
	FetchedAt    string   `json:"fetched_at"`     // RFC3339
}

// Label is a human-friendly identifier for the doc, suitable for display
// and citations.
func (d Doc) Label() string {
	switch d.Source {
	case SourceFAQ:
		if d.Year != "" && d.Month != "" {
			return "Ed's FAQ " + d.Month + " " + d.Year
		}
		return "Ed's FAQ"
	case SourceTSP:
		if d.Title != "" {
			return "TSP — " + d.Title
		}
		return "TSP — " + d.Slug
	case SourceRisk:
		if d.Section != "" {
			return "Risk essay — " + d.Section
		}
		return "Risk essay"
	}
	return d.Title
}

// DateKey returns a sortable date string for ordering (FAQ: YYYY-MM; TSP:
// the updated date if parseable, else empty; risk: empty).
func (d Doc) DateKey() string {
	if d.Source == SourceFAQ && d.Year != "" {
		mn := d.MonthN
		if mn < 1 {
			mn = 1
		}
		return d.Year + "-" + twoDigit(mn)
	}
	return ""
}

func twoDigit(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}
