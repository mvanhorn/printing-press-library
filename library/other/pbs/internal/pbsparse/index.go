// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package pbsparse

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Kind distinguishes the two release series PBS indexes on one page.
type Kind string

const (
	// KindWeekly is the Sensitive Price Indicator, published most weeks.
	KindWeekly Kind = "spi-weekly"
	// KindMonthly is the CPI monthly price annex.
	KindMonthly Kind = "cpi-monthly"
)

// FileRole classifies a release file by FILENAME rather than by the index key
// that pointed at it.
//
// This matters because the index is a hand-edited JavaScript array whose
// annexure/report keys are swapped on several rows, and at least one row
// carries the executive summary in both fields. Trusting the key silently
// files a summary as a price panel. The filename is the reliable signal.
type FileRole string

const (
	// RoleAnnexure is the city x item price appendix.
	RoleAnnexure FileRole = "annexure"
	// RoleReport is the executive summary: quintile indices, the item weight
	// vector, section counts and the rolling trend. Note that the weight
	// vector lives HERE, not in the annexure — "TOTAL" occurs zero times in
	// annexure text — so a release needs both files parsed.
	RoleReport FileRole = "report"
	// RoleConstruction is the Urban/Rural construction-input workbook, present
	// on only some CPI months.
	RoleConstruction FileRole = "construction"
	// RoleUnknown is a filename that matches no known role. Recorded rather
	// than dropped so a new upstream naming shape surfaces as a count.
	RoleUnknown FileRole = "unknown"
)

// ReleaseFile is one downloadable artifact belonging to a release.
type ReleaseFile struct {
	Role     FileRole `json:"role"`
	URL      string   `json:"url"`
	Filename string   `json:"filename"`
	Ext      string   `json:"ext"`
	// IndexKey is the raw JavaScript key that pointed at this file. Kept for
	// provenance so a key/filename disagreement is auditable rather than
	// silently resolved.
	IndexKey string `json:"index_key"`
}

// Release is one PBS publication event.
type Release struct {
	Kind Kind `json:"kind"`
	// AsOf is the period the release describes, taken from the index `date` or
	// `month` field. The index field is AUTHORITATIVE: two weekly rows carry a
	// filename whose embedded date disagrees with it, one by eight days, and
	// one row's file is named "Annex.pdf" with no date at all.
	AsOf     time.Time     `json:"as_of"`
	AsOfRaw  string        `json:"as_of_raw"`
	Files    []ReleaseFile `json:"files"`
	IndexPos int           `json:"index_pos"`
	// FilenameDateMismatch is set when a file's embedded date disagrees with
	// AsOf. Flagged, never corrected.
	FilenameDateMismatch bool `json:"filename_date_mismatch,omitempty"`
}

// AsOfKey returns the canonical primary key for a release.
func (r Release) AsOfKey() string { return r.AsOf.Format("2006-01-02") }

// File returns the first file with the given role.
func (r Release) File(role FileRole, preferExt string) (ReleaseFile, bool) {
	var fallback ReleaseFile
	var found bool
	for _, f := range r.Files {
		if f.Role != role {
			continue
		}
		if preferExt != "" && strings.EqualFold(f.Ext, preferExt) {
			return f, true
		}
		if !found {
			fallback, found = f, true
		}
	}
	return fallback, found
}

// Index is the parsed release index plus the anomalies found while parsing it.
type Index struct {
	Weekly  []Release `json:"weekly"`
	Monthly []Release `json:"monthly"`
	// DuplicateURLs maps a file URL to the release keys that both claim it.
	// This is how upstream data loss is detected: one CPI annex URL is listed
	// under both "March 2024" and "February 2024", and the file itself
	// contains only March, so February 2024 is already gone.
	DuplicateURLs map[string][]string `json:"duplicate_urls,omitempty"`
	// UnknownRoles counts filenames that matched no role pattern.
	UnknownRoles []string `json:"unknown_roles,omitempty"`
}

var (
	// Objects in the arrays use UNQUOTED JavaScript keys, so the payload is not
	// valid JSON and json.Unmarshal cannot be used.
	reObject = regexp.MustCompile(`\{[^{}]*\}`)
	// Keys may carry trailing whitespace before the colon — the CPI array
	// literally contains `Urban :` with a trailing space beside a plain
	// `Rural:`. Without trimming, thirteen months of construction data are
	// silently dropped.
	reKV = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s*:\s*"([^"]*)"`)

	reWeeklyDate = regexp.MustCompile(`^(\d{2})-(\d{2})-(\d{4})$`)

	reReportName = regexp.MustCompile(`(?i)executive|sumary|summary`)
	reAnnexName  = regexp.MustCompile(`(?i)annex`)
	reReportWord = regexp.MustCompile(`(?i)report`)
	reConstruct  = regexp.MustCompile(`(?i)construction`)

	// Filename date shapes seen across the 35 distinct naming patterns.
	reFnDate = []*regexp.Regexp{
		regexp.MustCompile(`(\d{2})[.\-](\d{2})[.\-](\d{4})`),
		regexp.MustCompile(`\b(\d{2})(\d{2})(\d{4})\b`),
	}
)

// ClassifyFileRole assigns a role from a filename.
//
// Report markers are tested before annexure markers because
// "SPI-Executive-SumarySPI-Report_*.pdf" contains both, and it is a report.
// The annexure test tolerates the upstream typo "Annexture".
func ClassifyFileRole(filename string) FileRole {
	base := filename
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	switch {
	case reConstruct.MatchString(base):
		return RoleConstruction
	case reReportName.MatchString(base):
		return RoleReport
	case reAnnexName.MatchString(base):
		return RoleAnnexure
	case reReportWord.MatchString(base):
		return RoleReport
	default:
		return RoleUnknown
	}
}

// resolveURL turns an index path into an absolute URL.
//
// The weekly array mixes conventions: most rows are relative WITHOUT a leading
// slash ("wp-content/uploads/..."), a minority are already absolute. The CPI
// array is entirely absolute. A naive join on the relative form produces a
// wrong path, so the shape is detected per value rather than assumed per array.
func resolveURL(base, v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
		return v
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(v, "/")
}

func filenameOf(u string) string {
	if i := strings.LastIndex(u, "/"); i >= 0 {
		return u[i+1:]
	}
	return u
}

func extOf(fn string) string {
	if i := strings.LastIndex(fn, "."); i >= 0 {
		return strings.ToLower(fn[i+1:])
	}
	return ""
}

// filenameDateAgrees reports whether a filename's embedded date matches asOf.
// A filename with no date at all agrees vacuously — "Annex.pdf" is a real row
// and must not be flagged as a mismatch.
func filenameDateAgrees(fn string, asOf time.Time) bool {
	for _, re := range reFnDate {
		if m := re.FindStringSubmatch(fn); m != nil {
			got := fmt.Sprintf("%s-%s-%s", m[1], m[2], m[3])
			if got == asOf.Format("02-01-2006") {
				return true
			}
			return false
		}
	}
	return true
}

// parseMonth accepts BOTH month spellings the CPI array uses.
//
// Measured: 47 of 50 rows carry a full month name ("August 2026") and 3 carry the
// abbreviated form ("Feb 2026", "Jan 2026", "Aug 2025"). Accepting only the full
// name silently drops those three months, and with them three months of
// Urban/Rural construction files.
func parseMonth(raw string) (time.Time, bool) {
	for _, layout := range []string{"January 2006", "Jan 2006"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// extractArray pulls one top-level JavaScript array literal out of a page by
// brace-matching from its assignment. Regex alone cannot do this safely because
// the array spans tens of kilobytes and contains nested braces.
func extractArray(page, name string) (string, bool) {
	re := regexp.MustCompile(`(?:const|var|let)\s+` + regexp.QuoteMeta(name) + `\s*=\s*\[`)
	loc := re.FindStringIndex(page)
	if loc == nil {
		return "", false
	}
	start := loc[1] - 1
	depth := 0
	inStr := false
	var quote byte
	esc := false
	for i := start; i < len(page); i++ {
		c := page[i]
		// STRING AWARENESS. Brackets inside a quoted value must not affect the
		// depth: a single filename containing ']' would otherwise terminate the
		// array early, dropping every later release while ParseIndex still
		// reported success because SOME releases were found.
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == quote:
				inStr = false
			}
			continue
		}
		switch c {
		case '"', '\'':
			inStr = true
			quote = c
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return page[start : i+1], true
			}
		}
	}
	// An unterminated array is a truncated page, not an empty one.
	return "", false
}

func parseObjects(lit string) []map[string]string {
	var out []map[string]string
	for _, om := range reObject.FindAllString(lit, -1) {
		rec := map[string]string{}
		for _, kv := range reKV.FindAllStringSubmatch(om, -1) {
			// Trim the key: `Urban ` and `Rural` are siblings in one object.
			rec[strings.TrimSpace(kv[1])] = kv[2]
		}
		if len(rec) > 0 {
			out = append(out, rec)
		}
	}
	return out
}

// ParseIndex parses the PBS price-statistics page into a sorted release index.
//
// baseURL is the site origin used to absolutise relative paths, e.g.
// "https://www.pbs.gov.pk".
func ParseIndex(page, baseURL string) (*Index, error) {
	idx := &Index{DuplicateURLs: map[string][]string{}}
	seen := map[string][]string{}

	record := func(rel *Release, key, raw string) {
		u := resolveURL(baseURL, raw)
		if u == "" {
			return
		}
		fn := filenameOf(u)
		role := ClassifyFileRole(fn)
		if role == RoleUnknown {
			idx.UnknownRoles = append(idx.UnknownRoles, fn)
		}
		if !filenameDateAgrees(fn, rel.AsOf) {
			rel.FilenameDateMismatch = true
		}
		rel.Files = append(rel.Files, ReleaseFile{
			Role: role, URL: u, Filename: fn, Ext: extOf(fn), IndexKey: key,
		})
		seen[u] = append(seen[u], string(rel.Kind)+":"+rel.AsOfKey())
	}

	if lit, ok := extractArray(page, "data"); ok {
		for i, rec := range parseObjects(lit) {
			raw := strings.TrimSpace(rec["date"])
			m := reWeeklyDate.FindStringSubmatch(raw)
			if m == nil {
				continue
			}
			t, err := time.Parse("02-01-2006", raw)
			if err != nil {
				continue
			}
			rel := Release{Kind: KindWeekly, AsOf: t, AsOfRaw: raw, IndexPos: i}
			for _, k := range []string{"annexure", "report", "annexureExcel", "reportExcel"} {
				if v := rec[k]; v != "" {
					record(&rel, k, v)
				}
			}
			idx.Weekly = append(idx.Weekly, rel)
		}
	}

	if lit, ok := extractArray(page, "cpidata1"); ok {
		for i, rec := range parseObjects(lit) {
			raw := strings.TrimSpace(rec["month"])
			t, ok := parseMonth(raw)
			if !ok {
				continue
			}
			rel := Release{Kind: KindMonthly, AsOf: t, AsOfRaw: raw, IndexPos: i}
			for _, k := range []string{"annex", "review", "Urban", "Rural"} {
				if v := rec[k]; v != "" {
					record(&rel, k, v)
				}
			}
			idx.Monthly = append(idx.Monthly, rel)
		}
	}

	if len(idx.Weekly) == 0 && len(idx.Monthly) == 0 {
		return nil, fmt.Errorf("no release arrays found in page (%d bytes): the index is a hand-edited JavaScript literal and its variable names may have changed", len(page))
	}

	// The upstream array is NOT sorted: its first element is the newest release
	// but its last element is not the oldest. Reading the tail as "oldest"
	// understates retrievable depth by more than two years, so sort explicitly.
	sortDesc := func(rs []Release) {
		sort.SliceStable(rs, func(i, j int) bool { return rs[i].AsOf.After(rs[j].AsOf) })
	}
	sortDesc(idx.Weekly)
	sortDesc(idx.Monthly)

	for u, keys := range seen {
		if len(keys) > 1 {
			uniq := map[string]bool{}
			var out []string
			for _, k := range keys {
				if !uniq[k] {
					uniq[k] = true
					out = append(out, k)
				}
			}
			if len(out) > 1 {
				sort.Strings(out)
				idx.DuplicateURLs[u] = out
			}
		}
	}
	return idx, nil
}

// All returns weekly and monthly releases together, newest first.
func (i *Index) All() []Release {
	out := append([]Release{}, i.Weekly...)
	out = append(out, i.Monthly...)
	sort.SliceStable(out, func(a, b int) bool { return out[a].AsOf.After(out[b].AsOf) })
	return out
}

// Gap is a detected hole in the weekly cadence.
type Gap struct {
	After        string `json:"after"`
	Before       string `json:"before"`
	DaysBetween  int    `json:"days_between"`
	MissingWeeks int    `json:"missing_weeks"`
}

// WeeklyGaps finds holes in the weekly series using a TOLERANCE-BASED cadence
// rather than a weekday rule.
//
// A weekday rule is wrong here: releases land on six different weekdays
// (predominantly Thursday, but also Wednesday, Saturday, Sunday, Tuesday and
// Friday), so an "expected Thursday" detector reports eighteen gaps that do not
// exist. Instead a gap is any interval long enough to have swallowed at least
// one whole publication cycle.
func (i *Index) WeeklyGaps() []Gap {
	rs := append([]Release{}, i.Weekly...)
	sort.SliceStable(rs, func(a, b int) bool { return rs[a].AsOf.Before(rs[b].AsOf) })
	var gaps []Gap
	for n := 0; n+1 < len(rs); n++ {
		days := int(rs[n+1].AsOf.Sub(rs[n].AsOf).Hours() / 24)
		// 7 is the nominal cadence. Allow slippage up to 11 days before
		// calling it a gap, so a release that merely shifted weekday is not
		// counted as missing.
		if days < 12 {
			continue
		}
		missing := days/7 - 1
		if missing < 1 {
			missing = 1
		}
		gaps = append(gaps, Gap{
			After: rs[n].AsOfKey(), Before: rs[n+1].AsOfKey(),
			DaysBetween: days, MissingWeeks: missing,
		})
	}
	return gaps
}
