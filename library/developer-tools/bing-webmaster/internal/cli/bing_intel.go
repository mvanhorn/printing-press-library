// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored SEO-intelligence helpers shared by the transcendence commands
// (review, drift, publish, triage, quota, gap, feed-health, watch). The logic
// here is kept pure and defensively parsed so it can be unit-tested without a
// live API: every helper tolerates missing fields, mixed PascalCase/camelCase
// keys, and numbers encoded as JSON numbers or strings.
package cli

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/bing-webmaster/internal/cliutil"
)

// ---------------------------------------------------------------- JSON helpers

// bArray decodes a JSON array into its raw elements; returns nil for non-arrays.
func bArray(data json.RawMessage) []json.RawMessage {
	var arr []json.RawMessage
	_ = json.Unmarshal(data, &arr)
	return arr
}

// bCIMap decodes a JSON object into a map keyed by lower-cased field names so
// callers can look up fields case-insensitively. Returns nil for non-objects.
func bCIMap(raw json.RawMessage) map[string]json.RawMessage {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	out := make(map[string]json.RawMessage, len(m))
	for k, v := range m {
		out[strings.ToLower(k)] = v
	}
	return out
}

// bNum extracts a numeric field case-insensitively, tolerating string-encoded
// numbers via cliutil.ExtractNumber.
func bNum(m map[string]json.RawMessage, key string) (float64, bool) {
	return cliutil.ExtractNumber(m, strings.ToLower(key))
}

// bStr extracts a string field case-insensitively. Non-string scalars are
// returned with surrounding quotes stripped.
func bStr(m map[string]json.RawMessage, key string) string {
	raw, ok := m[strings.ToLower(key)]
	if !ok || len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return strings.Trim(string(raw), "\"")
}

// ---------------------------------------------------------------- query stats

// bQueryRow is a normalized query-performance row.
type bQueryRow struct {
	Query       string
	Impressions float64
	Clicks      float64
	Position    float64
}

// bParseQueryRows normalizes a GetQueryStats/GetPageStats array. Rows without a
// query/page identifier are skipped. Position prefers AvgImpressionPosition and
// falls back to AvgClickPosition.
func bParseQueryRows(data json.RawMessage) []bQueryRow {
	rows := []bQueryRow{}
	for _, it := range bArray(data) {
		m := bCIMap(it)
		if m == nil {
			continue
		}
		q := bStr(m, "Query")
		if q == "" {
			q = bStr(m, "Page")
		}
		if q == "" {
			continue
		}
		imp, _ := bNum(m, "Impressions")
		clk, _ := bNum(m, "Clicks")
		pos, ok := bNum(m, "AvgImpressionPosition")
		if !ok {
			pos, _ = bNum(m, "AvgClickPosition")
		}
		rows = append(rows, bQueryRow{Query: q, Impressions: imp, Clicks: clk, Position: pos})
	}
	return rows
}

type bQueryMoved struct {
	Query           string  `json:"query"`
	ImpressionDelta float64 `json:"impressions_delta"`
	ClickDelta      float64 `json:"clicks_delta"`
	PositionDelta   float64 `json:"position_delta"` // new - old; negative = improved (moved up)
}

type bReviewSummary struct {
	GainedCount   int `json:"gained_count"`
	LostCount     int `json:"lost_count"`
	ImprovedCount int `json:"improved_count"`
	DeclinedCount int `json:"declined_count"`
}

type bReviewResult struct {
	PeriodDays int            `json:"period_days"`
	Gained     []string       `json:"gained"`
	Lost       []string       `json:"lost"`
	Moved      []bQueryMoved  `json:"moved"`
	Summary    bReviewSummary `json:"summary"`
}

// bComputeReview diffs two query snapshots. prior is the older baseline,
// current the fresh capture.
func bComputeReview(prior, current []bQueryRow, days int) bReviewResult {
	priorByQ := indexQueries(prior)
	currByQ := indexQueries(current)

	res := bReviewResult{PeriodDays: days, Gained: []string{}, Lost: []string{}, Moved: []bQueryMoved{}}
	for q := range currByQ {
		if _, ok := priorByQ[q]; !ok {
			res.Gained = append(res.Gained, q)
		}
	}
	for q := range priorByQ {
		if _, ok := currByQ[q]; !ok {
			res.Lost = append(res.Lost, q)
		}
	}
	for q, cur := range currByQ {
		old, ok := priorByQ[q]
		if !ok {
			continue
		}
		moved := bQueryMoved{
			Query:           q,
			ImpressionDelta: cur.Impressions - old.Impressions,
			ClickDelta:      cur.Clicks - old.Clicks,
			PositionDelta:   cur.Position - old.Position,
		}
		res.Moved = append(res.Moved, moved)
		if moved.PositionDelta < 0 {
			res.Summary.ImprovedCount++
		} else if moved.PositionDelta > 0 {
			res.Summary.DeclinedCount++
		}
	}
	sort.Strings(res.Gained)
	sort.Strings(res.Lost)
	sort.Slice(res.Moved, func(i, j int) bool { return res.Moved[i].PositionDelta < res.Moved[j].PositionDelta })
	res.Summary.GainedCount = len(res.Gained)
	res.Summary.LostCount = len(res.Lost)
	return res
}

func indexQueries(rows []bQueryRow) map[string]bQueryRow {
	m := make(map[string]bQueryRow, len(rows))
	for _, r := range rows {
		m[r.Query] = r
	}
	return m
}

// ---------------------------------------------------------------- drift

type bDriftRow struct {
	Query         string  `json:"query"`
	OldPosition   float64 `json:"old_position"`
	NewPosition   float64 `json:"new_position"`
	PositionDelta float64 `json:"position_delta"` // new - old; negative = climbed
}

type bDriftResult struct {
	PeriodDays int         `json:"period_days"`
	Climbers   []bDriftRow `json:"climbers"`
	Droppers   []bDriftRow `json:"droppers"`
}

// bComputeDrift ranks position movement for queries present in both snapshots.
func bComputeDrift(prior, current []bQueryRow, top, days int) bDriftResult {
	priorByQ := indexQueries(prior)
	var moved []bDriftRow
	for _, cur := range current {
		old, ok := priorByQ[cur.Query]
		if !ok {
			continue
		}
		delta := cur.Position - old.Position
		if delta == 0 {
			continue
		}
		moved = append(moved, bDriftRow{
			Query:         cur.Query,
			OldPosition:   old.Position,
			NewPosition:   cur.Position,
			PositionDelta: delta,
		})
	}
	// Climbers: most negative delta first. Droppers: most positive first.
	sort.Slice(moved, func(i, j int) bool { return moved[i].PositionDelta < moved[j].PositionDelta })
	res := bDriftResult{PeriodDays: days, Climbers: []bDriftRow{}, Droppers: []bDriftRow{}}
	for _, m := range moved {
		if m.PositionDelta < 0 && len(res.Climbers) < top {
			res.Climbers = append(res.Climbers, m)
		}
	}
	for i := len(moved) - 1; i >= 0; i-- {
		if moved[i].PositionDelta > 0 && len(res.Droppers) < top {
			res.Droppers = append(res.Droppers, moved[i])
		}
	}
	return res
}

// ---------------------------------------------------------------- sitemap

var locRe = regexp.MustCompile(`(?is)<loc>\s*(.*?)\s*</loc>`)

// bExtractLocs pulls <loc> URLs out of sitemap XML (namespace-agnostic).
func bExtractLocs(xml []byte) []string {
	matches := locRe.FindAllSubmatch(xml, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		u := strings.TrimSpace(string(m[1]))
		// Strip CDATA wrappers if present.
		u = strings.TrimPrefix(u, "<![CDATA[")
		u = strings.TrimSuffix(u, "]]>")
		u = strings.TrimSpace(u)
		if u != "" {
			out = append(out, u)
		}
	}
	return out
}

// bIsSitemapIndex reports whether the XML is a sitemap index (its <loc>s point
// to child sitemaps rather than pages).
func bIsSitemapIndex(xml []byte) bool {
	return regexp.MustCompile(`(?i)<sitemapindex`).Match(xml)
}

// bDedupeURLs removes blank lines and duplicates, preserving order.
func bDedupeURLs(urls []string) []string {
	seen := make(map[string]bool, len(urls))
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	return out
}

// bChunk splits urls into groups of at most size.
func bChunk(urls []string, size int) [][]string {
	if size <= 0 {
		size = 500
	}
	var out [][]string
	for i := 0; i < len(urls); i += size {
		end := i + size
		if end > len(urls) {
			end = len(urls)
		}
		out = append(out, urls[i:end])
	}
	return out
}

// ---------------------------------------------------------------- GSC CSV

// bDetectQueryColumn returns the index of the query column in a GSC export
// header, or -1 if none is found. Matches "query", "queries", "top queries",
// "keyword", or "search query" case-insensitively.
func bDetectQueryColumn(header []string) int {
	for i, h := range header {
		hl := strings.ToLower(strings.TrimSpace(h))
		if strings.Contains(hl, "quer") || strings.Contains(hl, "keyword") {
			return i
		}
	}
	return -1
}

// bParseGSCQueries reads a GSC performance CSV export and returns the queries
// from the detected query column (lower-cased, deduped).
func bParseGSCQueries(r io.Reader) ([]string, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []string{}, nil
	}
	col := bDetectQueryColumn(rows[0])
	if col < 0 {
		col = 0 // fall back to first column
	}
	seen := map[string]bool{}
	out := []string{}
	for _, row := range rows[1:] {
		if col >= len(row) {
			continue
		}
		q := strings.TrimSpace(row[col])
		if q == "" {
			continue
		}
		key := strings.ToLower(q)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, q)
	}
	return out, nil
}

type bGapResult struct {
	Summary struct {
		BingOnlyCount   int `json:"bing_only_count"`
		GoogleOnlyCount int `json:"google_only_count"`
		BothCount       int `json:"both_count"`
	} `json:"summary"`
	BingOnly   []string `json:"bing_only"`
	GoogleOnly []string `json:"google_only"`
}

// bComputeGap reconciles Bing queries against GSC queries (case-insensitive).
func bComputeGap(bingQueries, gscQueries []string) bGapResult {
	bingSet := lowerSet(bingQueries)
	gscSet := lowerSet(gscQueries)
	res := bGapResult{BingOnly: []string{}, GoogleOnly: []string{}}
	for q := range bingSet {
		if !gscSet[q] {
			res.BingOnly = append(res.BingOnly, q)
		} else {
			res.Summary.BothCount++
		}
	}
	for q := range gscSet {
		if !bingSet[q] {
			res.GoogleOnly = append(res.GoogleOnly, q)
		}
	}
	sort.Strings(res.BingOnly)
	sort.Strings(res.GoogleOnly)
	res.Summary.BingOnlyCount = len(res.BingOnly)
	res.Summary.GoogleOnlyCount = len(res.GoogleOnly)
	return res
}

func lowerSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		s = strings.TrimSpace(strings.ToLower(s))
		if s != "" {
			m[s] = true
		}
	}
	return m
}

// ---------------------------------------------------------------- crawl issues

// bCrawlIssueLabels decodes the Bing CrawlIssues flags integer into human
// labels. The mapping is best-effort (the Bing docs expose a flags enum whose
// exact bit values are not all documented); unrecognized bits are reported as
// "other(bitN)" so nothing is silently dropped.
var bCrawlIssueFlags = []struct {
	Bit   int64
	Label string
}{
	{1, "http-300-redirect"},
	{2, "http-301-moved"},
	{4, "http-302-found"},
	{8, "http-400-bad-request"},
	{16, "http-403-forbidden"},
	{32, "http-404-not-found"},
	{64, "http-500-server-error"},
	{128, "blocked-by-robots-txt"},
	{256, "contains-malware"},
	{512, "missing-title"},
	{1024, "missing-meta-description"},
	{2048, "title-too-long"},
	{4096, "title-too-short"},
	{8192, "multiple-titles"},
	{16384, "multiple-meta-descriptions"},
}

func bDecodeCrawlIssues(flags int64) []string {
	if flags == 0 {
		return []string{"none"}
	}
	var out []string
	matched := int64(0)
	for _, f := range bCrawlIssueFlags {
		if flags&f.Bit != 0 {
			out = append(out, f.Label)
			matched |= f.Bit
		}
	}
	// Report any remaining set bits we don't have a label for.
	for bit := int64(1); bit <= flags; bit <<= 1 {
		if flags&bit != 0 && matched&bit == 0 {
			out = append(out, "other(bit"+itoa(bit)+")")
		}
	}
	if len(out) == 0 {
		out = append(out, "other("+itoa(flags)+")")
	}
	return out
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
