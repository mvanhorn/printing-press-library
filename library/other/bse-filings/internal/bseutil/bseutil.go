// Copyright 2026 rushyant-m. Licensed under Apache-2.0. See LICENSE.

// Package bseutil holds the pure, side-effect-free helpers the novel BSE
// commands depend on: transcript paragraph splitting, BSE date parsing,
// per-term frequency counting, PeerSmartSearch name extraction, and quarter
// derivation from a filing date. Kept in its own package so the logic is unit
// testable without a SQLite store or a live HTTP client.
package bseutil

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// paragraphTargetLen is the soft size a chunk grows to before SplitParagraphs
// looks for a sentence boundary to break on. Big enough that a chunk holds a
// full thought (good FTS phrase matches) but small enough that grep snippets
// stay readable.
const paragraphTargetLen = 600

// SplitParagraphs turns extracted PDF text into paragraph-sized chunks. BSE
// transcript PDFs extract as one continuous run of single-newline lines with
// no blank-line paragraph breaks, so a naive blank-line split yields one giant
// paragraph. Instead we honor blank-line breaks when present, but within each
// block we accumulate lines into a buffer and flush a chunk once it passes a
// target length at the next sentence boundary. Whitespace-only and
// sub-threshold fragments (page numbers, headers) are dropped.
func SplitParagraphs(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	collapse := regexp.MustCompile(`\n{2,}`)
	blocks := collapse.Split(text, -1)

	var out []string
	for _, b := range blocks {
		out = append(out, chunkBlock(b)...)
	}
	return out
}

// chunkBlock joins the lines of one block, then splits the joined text into
// chunks of roughly paragraphTargetLen that end on sentence boundaries.
func chunkBlock(block string) []string {
	var joined strings.Builder
	for _, ln := range strings.Split(block, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		if joined.Len() > 0 {
			joined.WriteByte(' ')
		}
		joined.WriteString(ln)
	}
	text := normalizeSpace(joined.String())
	if len(text) < 25 {
		return nil
	}

	var chunks []string
	var buf strings.Builder
	flush := func() {
		p := strings.TrimSpace(buf.String())
		if len(p) >= 25 {
			chunks = append(chunks, p)
		}
		buf.Reset()
	}
	// Split into sentences on ". " boundaries, re-appending the period, and
	// pack sentences into chunks until the target length is reached.
	for _, sentence := range splitSentences(text) {
		if buf.Len() > 0 {
			buf.WriteByte(' ')
		}
		buf.WriteString(sentence)
		if buf.Len() >= paragraphTargetLen {
			flush()
		}
	}
	flush()
	return chunks
}

// splitSentences breaks text on ". " boundaries while keeping the terminator
// attached to the preceding sentence. Falls back to the whole string when no
// boundary is present (single-sentence blocks).
func splitSentences(text string) []string {
	var out []string
	start := 0
	for i := 0; i+1 < len(text); i++ {
		if (text[i] == '.' || text[i] == '?' || text[i] == '!') && text[i+1] == ' ' {
			out = append(out, strings.TrimSpace(text[start:i+1]))
			start = i + 1
		}
	}
	if rest := strings.TrimSpace(text[start:]); rest != "" {
		out = append(out, rest)
	}
	return out
}

var spaceRE = regexp.MustCompile(`\s+`)

func normalizeSpace(s string) string {
	return strings.TrimSpace(spaceRE.ReplaceAllString(s, " "))
}

// bseDateLayouts are the surface forms BSE uses across endpoints. Corp-action
// and forthcoming-result feeds use "27 May 2026"; some carry a time suffix.
var bseDateLayouts = []string{
	"2 Jan 2006",
	"02 Jan 2006",
	"2 January 2006",
	"02 January 2006",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02",
	"20060102",
}

// ParseBSEDate parses the date strings BSE returns ("27 May 2026",
// "2026-05-26T20:33:59.6", "20260527") into a time.Time. The bool reports
// success; callers that cannot parse a date skip the row rather than guess.
func ParseBSEDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range bseDateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// CountTerms returns a case-insensitive whole-word-ish occurrence count of
// each term across the supplied texts. Matching is substring-on-word: a term
// "margin" matches "margins" and "margin." but the count is per-occurrence,
// so "margin margin" yields 2. Terms are lowercased for the lookup; the
// returned map is keyed by the original term spelling.
func CountTerms(texts []string, terms []string) map[string]int {
	counts := make(map[string]int, len(terms))
	lowered := make([]string, len(terms))
	for i, t := range terms {
		lowered[i] = strings.ToLower(strings.TrimSpace(t))
		counts[t] = 0
	}
	for _, txt := range texts {
		lt := strings.ToLower(txt)
		for i, lterm := range lowered {
			if lterm == "" {
				continue
			}
			counts[terms[i]] += strings.Count(lt, lterm)
		}
	}
	return counts
}

// liclickRE pulls the scrip code and company name out of a PeerSmartSearch
// HTML fragment: ng-click="liclick('500325','RELIANCE INDUSTRIES LTD')".
var liclickRE = regexp.MustCompile(`liclick\('(\d+)','([^']*)'\)`)

// PeerMatch is one suggestion parsed from a PeerSmartSearch response.
type PeerMatch struct {
	ScripCode string
	Name      string
}

// ParsePeerSearch extracts the (scrip code, name) pairs from a PeerSmartSearch
// HTML-string response. Returns them in document order; the first entry is the
// best match for resolving a name when the user omits --name.
func ParsePeerSearch(html string) []PeerMatch {
	var out []PeerMatch
	for _, m := range liclickRE.FindAllStringSubmatch(html, -1) {
		out = append(out, PeerMatch{ScripCode: m[1], Name: strings.TrimSpace(m[2])})
	}
	return out
}

// QuarterFromDate maps a filing date to an Indian-fiscal-year quarter label
// (e.g. "Q1 FY27"). India's fiscal year runs Apr–Mar, so Apr–Jun is Q1 and a
// result filed in that window reports the prior fiscal year. The label uses
// the two-digit fiscal-year-end suffix matching how analysts cite quarters.
func QuarterFromDate(t time.Time) string {
	month := int(t.Month())
	year := t.Year()
	var q, fyEnd int
	switch {
	case month >= 4 && month <= 6:
		q, fyEnd = 1, year+1
	case month >= 7 && month <= 9:
		q, fyEnd = 2, year+1
	case month >= 10 && month <= 12:
		q, fyEnd = 3, year+1
	default: // Jan–Mar
		q, fyEnd = 4, year
	}
	// Two-digit fiscal suffix, zero-padded (FY07, not FY7).
	fy := fyEnd % 100
	suffix := strconv.Itoa(fy)
	if fy < 10 {
		suffix = "0" + suffix
	}
	return "Q" + strconv.Itoa(q) + " FY" + suffix
}
