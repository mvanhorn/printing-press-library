// Copyright 2026 erash11 and contributors. Licensed under Apache-2.0. See LICENSE.
// Shared helpers for the hand-written novel commands (acwr, pb, benchmark,
// rtp, diff, heatmap, report). Live-API strategy: these commands fetch stats
// through the generated client and compute locally; the stats engine cannot
// express rolling windows, ranks, or cross-session joins.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/catapult-openfield-connect/internal/client"
	"github.com/mvanhorn/printing-press-library/library/health/catapult-openfield-connect/internal/cliutil"
)

type statsRow map[string]json.RawMessage

func rowString(r statsRow, key string) string {
	raw, ok := r[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		return strconv.FormatFloat(n, 'f', -1, 64)
	}
	return strings.Trim(string(raw), `"`)
}

func rowNum(r statsRow, key string) (float64, bool) {
	return cliutil.ExtractNumber(map[string]json.RawMessage(r), key)
}

func statsFilter(name, comparison string, values ...string) map[string]any {
	return map[string]any{"name": name, "comparison": comparison, "values": values}
}

// fetchStatsRows POSTs /stats with the given parameter slugs, grouping, and
// filters, returning tolerant map rows (metric values may arrive as numbers
// or JSON-encoded strings).
func fetchStatsRows(ctx context.Context, c *client.Client, parameters, groupBy []string, filters []map[string]any) ([]statsRow, error) {
	body := map[string]any{
		"parameters": parameters,
		"group_by":   groupBy,
	}
	if len(filters) > 0 {
		body["filters"] = filters
	}
	data, _, err := c.PostQueryWithParams(ctx, "/stats", map[string]string{"requested_only": "false"}, body)
	if err != nil {
		return nil, err
	}
	rows := make([]statsRow, 0)
	if err := json.Unmarshal(data, &rows); err != nil {
		// Some tenants wrap the array in an envelope; try common shapes.
		var envelope map[string]json.RawMessage
		if envErr := json.Unmarshal(data, &envelope); envErr == nil {
			for _, key := range []string{"data", "results", "stats"} {
				if inner, ok := envelope[key]; ok {
					if err2 := json.Unmarshal(inner, &rows); err2 == nil {
						return rows, nil
					}
				}
			}
		}
		return nil, fmt.Errorf("parsing stats response: %w", err)
	}
	return rows, nil
}

type activityMeta struct {
	ID    string
	Name  string
	Start time.Time
}

// fetchActivityIndex lists activities in [from, to] and returns them keyed by
// ID, plus a slice sorted by start time ascending.
func fetchActivityIndex(ctx context.Context, c *client.Client, from, to time.Time) (map[string]activityMeta, []activityMeta, error) {
	params := map[string]string{}
	if !from.IsZero() {
		params["startTime"] = strconv.FormatInt(from.Unix(), 10)
	}
	if !to.IsZero() {
		params["endTime"] = strconv.FormatInt(to.Unix(), 10)
	}
	data, err := c.Get(ctx, "/activities", params)
	if err != nil {
		return nil, nil, err
	}
	var raw []statsRow
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil, fmt.Errorf("parsing activities response: %w", err)
	}
	index := make(map[string]activityMeta, len(raw))
	ordered := make([]activityMeta, 0, len(raw))
	for _, r := range raw {
		id := rowString(r, "id")
		if id == "" {
			continue
		}
		meta := activityMeta{ID: id, Name: rowString(r, "name")}
		if ts, ok := cliutil.ExtractNumber(map[string]json.RawMessage(r), "start_time"); ok && ts > 0 {
			meta.Start = time.Unix(int64(ts), 0)
		}
		index[id] = meta
		ordered = append(ordered, meta)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Start.Before(ordered[j].Start) })
	return index, ordered, nil
}

type athleteMeta struct {
	ID       string
	Name     string
	Position string
}

func fetchAthleteIndex(ctx context.Context, c *client.Client) (map[string]athleteMeta, error) {
	data, err := c.Get(ctx, "/athletes", nil)
	if err != nil {
		return nil, err
	}
	var raw []statsRow
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing athletes response: %w", err)
	}
	out := make(map[string]athleteMeta, len(raw))
	for _, r := range raw {
		id := rowString(r, "id")
		if id == "" {
			continue
		}
		name := strings.TrimSpace(rowString(r, "first_name") + " " + rowString(r, "last_name"))
		pos := rowString(r, "position_name")
		if pos == "" {
			pos = rowString(r, "position")
		}
		out[id] = athleteMeta{ID: id, Name: name, Position: pos}
	}
	return out, nil
}

// matchAthlete resolves a case-insensitive full/partial name to one athlete.
func matchAthlete(index map[string]athleteMeta, query string) (athleteMeta, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	var hits []athleteMeta
	for _, a := range index {
		if strings.Contains(strings.ToLower(a.Name), q) {
			hits = append(hits, a)
		}
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		return athleteMeta{}, notFoundErr(fmt.Errorf("no athlete matching %q", query))
	default:
		names := make([]string, 0, len(hits))
		for _, h := range hits {
			names = append(names, h.Name)
		}
		sort.Strings(names)
		return athleteMeta{}, usageErr(fmt.Errorf("athlete %q is ambiguous: %s", query, strings.Join(names, ", ")))
	}
}

// riskZone buckets an acute:chronic ratio using the standard thresholds.
func riskZone(acwr float64) string {
	switch {
	case acwr <= 0:
		return "no-data"
	case acwr < 0.8:
		return "caution-low"
	case acwr <= 1.3:
		return "safe"
	case acwr <= 1.5:
		return "caution"
	default:
		return "danger"
	}
}

func dayKey(t time.Time) string { return t.Format("2006-01-02") }

// resolveActivityArg turns an activity reference (raw ID, YYYY-MM-DD or
// YYYYMMDD date, "last", "prev") into an activity from the ordered index.
func resolveActivityArg(arg string, index map[string]activityMeta, ordered []activityMeta) (activityMeta, error) {
	if m, ok := index[arg]; ok {
		return m, nil
	}
	if len(ordered) > 0 {
		switch strings.ToLower(arg) {
		case "last", "latest":
			return ordered[len(ordered)-1], nil
		case "prev", "previous":
			if len(ordered) > 1 {
				return ordered[len(ordered)-2], nil
			}
		}
	}
	norm := arg
	if len(arg) == 8 {
		if _, err := strconv.Atoi(arg); err == nil {
			norm = arg[0:4] + "-" + arg[4:6] + "-" + arg[6:8]
		}
	}
	if _, err := time.Parse("2006-01-02", norm); err == nil {
		var candidates []activityMeta
		for _, m := range ordered {
			if !m.Start.IsZero() && dayKey(m.Start) == norm {
				candidates = append(candidates, m)
			}
		}
		if len(candidates) > 0 {
			return candidates[len(candidates)-1], nil
		}
		return activityMeta{}, notFoundErr(fmt.Errorf("no activity on %s", norm))
	}
	return activityMeta{}, notFoundErr(fmt.Errorf("activity %q not found in the last year of activities (pass an ID from that window, a date like 2026-03-15, 'last', or 'prev')", arg))
}

func median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	s := append([]float64(nil), vals...)
	sort.Float64s(s)
	mid := len(s) / 2
	if len(s)%2 == 0 {
		return (s[mid-1] + s[mid]) / 2
	}
	return s[mid]
}

// ACWR window constants — these three move together.
const (
	acwrChronicDays           = 28
	acwrAcuteDays             = 7
	chronicWeeks              = acwrChronicDays / 7
	defaultPBWindowActivities = 60 // ~a season of activities
	verifyActivityBudget      = 8  // lastActivities cap under verify/dogfood envs
)

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// nameMatches reports whether name contains query case-insensitively; an
// empty query matches everything.
func nameMatches(name, query string) bool {
	return query == "" || strings.Contains(strings.ToLower(name), strings.ToLower(query))
}

// computeACWR turns a per-day load map into (acute 7d sum, chronic weekly
// average over the 28d window, ratio). asOf anchors the acute window.
func computeACWR(loadByDay map[string]float64, asOf time.Time) (acute, chronicWeekly, ratio float64) {
	acuteStart := dayKey(asOf.AddDate(0, 0, -(acwrAcuteDays - 1)))
	var total float64
	for day, v := range loadByDay {
		total += v
		if day >= acuteStart {
			acute += v
		}
	}
	chronicWeekly = total / chronicWeeks
	if chronicWeekly > 0 {
		ratio = acute / chronicWeekly
	}
	return acute, chronicWeekly, ratio
}

// sortAcwr orders rows danger-first, then by ratio descending.
func sortAcwr(rows []acwrRow) {
	zoneRank := map[string]int{"danger": 0, "caution": 1, "caution-low": 2, "safe": 3, "no-data": 4}
	sort.Slice(rows, func(i, j int) bool {
		if zoneRank[rows[i].RiskZone] != zoneRank[rows[j].RiskZone] {
			return zoneRank[rows[i].RiskZone] < zoneRank[rows[j].RiskZone]
		}
		return rows[i].ACWR > rows[j].ACWR
	})
}

// rowDate returns the row's session date (YYYY-MM-DD), preferring the stats
// engine's own date column, falling back to start_time.
func rowDate(r statsRow) string {
	if d := rowString(r, "date"); d != "" {
		if norm := normalizeDay(d); norm != "" {
			return norm
		}
	}
	if ts, ok := cliutil.ExtractNumber(map[string]json.RawMessage(r), "start_time"); ok && ts > 0 {
		return dayKey(time.Unix(int64(ts), 0))
	}
	return ""
}

// normalizeDay accepts the formats OpenField actually emits (MM/DD/YYYY on
// US tenants, ISO elsewhere) and returns YYYY-MM-DD, or "" when unparseable.
func normalizeDay(d string) string {
	if len(d) > 10 {
		d = d[:10]
	}
	for _, layout := range []string{"2006-01-02", "01/02/2006", "1/2/2006"} {
		if t, err := time.Parse(layout, d); err == nil {
			return dayKey(t)
		}
	}
	return ""
}

// indexFromRows derives an activity index from stats rows themselves
// (requested_only=false rows carry activity_id/activity_name/start_time).
func indexFromRows(rows []statsRow) map[string]activityMeta {
	out := map[string]activityMeta{}
	for _, r := range rows {
		id := rowString(r, "activity_id")
		if id == "" {
			continue
		}
		if _, ok := out[id]; ok {
			continue
		}
		meta := activityMeta{ID: id, Name: rowString(r, "activity_name")}
		if ts, ok := cliutil.ExtractNumber(map[string]json.RawMessage(r), "start_time"); ok && ts > 0 {
			meta.Start = time.Unix(int64(ts), 0)
		}
		out[id] = meta
	}
	return out
}

// fetchStatsWindow fetches per-athlete-per-activity stats rows for [from, to].
// The OpenField date filter hangs server-side (observed 2026-08-06), so the
// window is expressed as an activity_id set from the fast /activities listing.
// Under verify/dogfood envs it curtails to the last 8 activities to fit the
// probe time budget.
func fetchStatsWindow(ctx context.Context, c *client.Client, parameters []string, from, to time.Time) ([]statsRow, error) {
	if cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
		rows, err := fetchStatsRows(ctx, c, parameters, []string{"athlete", "activity"},
			[]map[string]any{statsFilter("lastActivities", "=", fmt.Sprint(verifyActivityBudget))})
		if err != nil {
			return nil, err
		}
		fromKey, toKey := dayKey(from), dayKey(to)
		kept := rows[:0]
		for _, r := range rows {
			if d := rowDate(r); d >= fromKey && d <= toKey {
				kept = append(kept, r)
			}
		}
		return kept, nil
	}
	_, ordered, err := fetchActivityIndex(ctx, c, from, to)
	if err != nil {
		return nil, err
	}
	if len(ordered) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len(ordered))
	for _, m := range ordered {
		ids = append(ids, m.ID)
	}
	return fetchStatsRows(ctx, c, parameters, []string{"athlete", "activity"},
		[]map[string]any{statsFilter("activity_id", "=", ids...)})
}

// fetchPB returns each athlete's aggregated value for a metric (max for
// velocity-style metrics) over the last N activities. A true all-time query
// (no filter) exceeds 90s server-side on real tenants; N=60 answers in ~6s
// and covers a season. Curtailed to 8 under verify/dogfood envs.
func fetchPB(ctx context.Context, c *client.Client, metric string, lastN int) (map[string]float64, error) {
	if cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
		lastN = verifyActivityBudget
	}
	rows, err := fetchStatsRows(ctx, c, []string{metric}, []string{"athlete"},
		[]map[string]any{statsFilter("lastActivities", "=", fmt.Sprint(lastN))})
	if err != nil {
		return nil, err
	}
	out := make(map[string]float64, len(rows))
	for _, r := range rows {
		if v, ok := rowNum(r, metric); ok {
			out[rowString(r, "athlete_id")] = v
		}
	}
	return out, nil
}

// markdownCell neutralizes table-breaking characters in user/tenant data.
func markdownCell(v string) string {
	v = strings.ReplaceAll(v, "|", "\\|")
	v = strings.ReplaceAll(v, "\n", " ")
	return v
}

// csvSafeCell prefixes formula-triggering characters so exported CSVs cannot
// execute in spreadsheet apps.
func csvSafeCell(v string) string {
	if v != "" && strings.ContainsRune("=+-@", rune(v[0])) {
		return "'" + v
	}
	return v
}
