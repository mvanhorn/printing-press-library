// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.

package cli

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/cliutil"
)

// EventRow is one regulatory determination as published.
//
// It is deliberately an EVENT, not a number: the amounts live inside the linked
// PDFs, and inventing them from the row text would be fabrication. What this
// carries is the date, who it concerns, what it says, and the document itself —
// which is what makes the feed event-study-grade against a ticker.
type EventRow struct {
	Surface string `json:"surface"`
	// Group is the accordion heading the row sat under: a company name on the
	// IPP pages, a YEAR on the K-Electric distribution page.
	Group string `json:"group,omitempty"`
	// Date is the published date string, verbatim.
	Date string `json:"date"`
	// DateISO is Date normalised to YYYY-MM-DD, or "" when the published
	// string is not a valid date. It is never guessed: one published row
	// carries the impossible 13-10-2026 as a month, and a row NEPRA typo'd
	// must stay visible rather than being silently repaired.
	DateISO     string `json:"date_iso,omitempty"`
	Description string `json:"description"`
	// DocumentURL is the absolute URL of the linked PDF, with NEPRA's own
	// encoding preserved.
	DocumentURL string `json:"document_url,omitempty"`
	// Docket is a TRF-nnn tariff docket when the row names one. These are the
	// stable join keys: TRF-71 is Nishat Power, TRF-70 Nishat Chunian,
	// TRF-600 Kot Addu, TRF-314 Lucky, TRF-100 the ex-WAPDA DISCOs.
	Docket string `json:"docket,omitempty"`
	// SRO is an S.R.O. notification number when the row names one.
	SRO string `json:"sro,omitempty"`
	// Undated marks a determination NEPRA published with no date. It is true
	// for a row that carries a document and text but no parseable date, and
	// it exists so the absence is a visible fact rather than a dropped row or
	// a guessed date. A date window filter excludes these, because they
	// cannot be placed in time.
	Undated bool `json:"undated,omitempty"`
}

// eventsExtract pulls every determination row out of one accordion page.
//
// The markup is `<li class="accordion">` → `<div class="accordion-header">`
// carrying an `<h6>` heading → `<div class="accordion-content">` holding a
// header-less three-column table: date | description | "view" → PDF href.
// Rather than depend on that nesting exactly, this tracks the most recent
// accordion heading and attributes each subsequent row to it, which survives
// the markup drift between pages.
//
// A row is accepted only when its first cell is a dd-mm-yyyy date. The tables
// are header-less, so anything else is furniture; the count of rejected rows
// is returned so a silent drop is impossible.
func eventsExtract(surfaceID string, doc string, baseURL string) (rows []EventRow, skipped int, err error) {
	node, err := xhtml.Parse(strings.NewReader(doc))
	if err != nil {
		return nil, 0, fmt.Errorf("parsing events page: %w", err)
	}

	var group string
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode {
			if hasClass(n, "accordion-header") {
				if h := cliutil.CleanText(nodeTextOf(n)); h != "" {
					group = collapseSpace(h)
				}
			}
			if n.DataAtom == atom.Tr {
				if row, ok := eventRowFromTR(n, surfaceID, group, baseURL); ok {
					rows = append(rows, row)
				} else if trHasCells(n) {
					skipped++
				}
				// A <tr> holds no nested <tr>, so stop descending.
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(node)
	return rows, skipped, nil
}

// eventDateRE tolerates the stray whitespace NEPRA's markup puts inside a
// date. MEASURED: GEPCO publishes "09-12- 2009" and "11-09- 2009", which a
// strict ^dd-mm-yyyy$ pattern rejects — silently dropping two real
// determinations per page across most of the DISCO surfaces.
var eventDateRE = regexp.MustCompile(`^(\d{1,2})\s*-\s*(\d{1,2})\s*-\s*(\d{4})$`)

// eventLeadingDateRE matches a date at the START of a longer cell. Some rows
// fuse the date into the description cell instead of giving it its own column,
// so the date must be split off rather than buried in prose.
var eventLeadingDateRE = regexp.MustCompile(`^(\d{1,2}\s*-\s*\d{1,2}\s*-\s*\d{4})\s+(\S.*)$`)
var docketRE = regexp.MustCompile(`\bTRF[-\s]?(\d{1,4})\b`)
var sroRE = regexp.MustCompile(`(?i)\bS\.?\s?R\.?\s?O\.?\s*\.?\s*(\d{1,5}\s*\(\s*I\s*\)\s*[-/]?\s*\d{0,4})`)

// eventRowFromTR reads one table row, returning false when it is not a
// determination row.
func eventRowFromTR(tr *xhtml.Node, surfaceID, group, baseURL string) (EventRow, bool) {
	var cells []string
	var href string
	for td := tr.FirstChild; td != nil; td = td.NextSibling {
		if td.Type != xhtml.ElementNode || (td.DataAtom != atom.Td && td.DataAtom != atom.Th) {
			continue
		}
		cells = append(cells, collapseSpace(cliutil.CleanText(nodeTextOf(td))))
		if href == "" {
			href = firstHref(td)
		}
	}
	if len(cells) < 2 {
		return EventRow{}, false
	}
	first := strings.TrimSpace(cells[0])
	var date, desc string
	switch {
	case eventDateRE.MatchString(first):
		// The ordinary three-column shape: date | description | view.
		date, desc = first, strings.TrimSpace(cells[1])
	case eventLeadingDateRE.MatchString(first):
		// The date was fused into the first cell ahead of the text.
		m := eventLeadingDateRE.FindStringSubmatch(first)
		date, desc = m[1], m[2]
	default:
		// No date column at all: the row is description | view, so the
		// description is the FIRST cell. Reading cells[1] here would report
		// the link's own label ("View") as the determination text.
		desc = first
	}

	row := EventRow{
		Surface:     surfaceID,
		Group:       group,
		Date:        date,
		DateISO:     eventDateISO(date),
		Description: desc,
	}
	if href != "" {
		// SCHEME ALLOWLIST. normalizeHTMLURL returns any ABSOLUTE url
		// unchanged, so whatever scheme the page happens to carry travels
		// into this row and out through --json into whatever an agent does
		// with a "document URL". Only http and https name a document that
		// can be fetched; a javascript:, data: or file: href is not a
		// determination PDF and is dropped with the row kept, because the
		// determination itself is still real even when its link is not.
		if u := normalizeHTMLURL(href, baseURL); eventsFetchableURL(u) {
			row.DocumentURL = u
		}
	}
	if date == "" {
		// A determination NEPRA published without a date. It is kept, flagged,
		// and given NO guessed date: dropping it would lose a real decision,
		// and inferring one from a neighbouring row would invent a fact.
		// MEASURED: two such rows on the GEPCO page alone.
		if row.DocumentURL == "" || row.Description == "" {
			// No date, no document and no text: table furniture, not a row.
			return EventRow{}, false
		}
		row.Undated = true
	}

	// The docket and SRO are read from the description AND the document path,
	// because the filename often carries the docket where the prose does not.
	hay := row.Description + " " + href
	if m := docketRE.FindStringSubmatch(hay); m != nil {
		row.Docket = "TRF-" + m[1]
	}
	if m := sroRE.FindStringSubmatch(hay); m != nil {
		row.SRO = collapseSpace(strings.ReplaceAll(m[1], " ", ""))
	}
	return row, true
}

// eventDateISO normalises a dd-mm-yyyy string, returning "" when it is not a
// real calendar date. NEPRA publishes at least one impossible month, and a
// bad date must stay visible in Date rather than being repaired or dropped.
func eventDateISO(s string) string {
	m := eventDateRE.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	t, err := time.Parse("2-1-2006", strings.TrimSpace(m[1])+"-"+strings.TrimSpace(m[2])+"-"+strings.TrimSpace(m[3]))
	if err != nil {
		return ""
	}
	return t.Format("2006-01-02")
}

func trHasCells(tr *xhtml.Node) bool {
	for td := tr.FirstChild; td != nil; td = td.NextSibling {
		if td.Type == xhtml.ElementNode && (td.DataAtom == atom.Td || td.DataAtom == atom.Th) {
			return true
		}
	}
	return false
}

func hasClass(n *xhtml.Node, want string) bool {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, "class") {
			for _, c := range strings.Fields(a.Val) {
				if c == want {
					return true
				}
			}
		}
	}
	return false
}

func firstHref(n *xhtml.Node) string {
	if n.Type == xhtml.ElementNode && n.DataAtom == atom.A {
		for _, a := range n.Attr {
			if strings.EqualFold(a.Key, "href") && strings.TrimSpace(a.Val) != "" {
				return strings.TrimSpace(a.Val)
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if h := firstHref(c); h != "" {
			return h
		}
	}
	return ""
}

func nodeTextOf(n *xhtml.Node) string {
	var b strings.Builder
	var walk func(*xhtml.Node)
	walk = func(x *xhtml.Node) {
		if x.Type == xhtml.TextNode {
			b.WriteString(x.Data)
			b.WriteString(" ")
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// eventsFetchableURL reports whether a scraped href names something a caller
// could actually fetch. Relative and root-relative forms have already been
// resolved against the page's own base by the time this runs, so anything
// still carrying a scheme must carry an allowed one.
func eventsFetchableURL(u string) bool {
	if u == "" {
		return false
	}
	lower := strings.ToLower(u)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return true
	}
	// No scheme at all is fine: it is a path on this host.
	return !strings.Contains(lower[:min(len(lower), 24)], ":")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
