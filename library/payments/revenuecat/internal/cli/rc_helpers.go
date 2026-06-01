// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

// Shared helpers for the hand-authored RevenueCat "novel" commands.
//
// RevenueCat's v2 objects are flat (no JSON:API attributes envelope): an
// object has top-level id/status/customer_id/... fields. Monetary values are
// nested MonetaryAmount objects ({currency, gross, proceeds, ...}). Timestamps
// are ms-since-epoch integers (created_at, current_period_ends_at, ...) but a
// few fields are ISO8601 strings (last_updated_at_iso8601). These helpers keep
// the per-command code thin and NULL-safe across that variation.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/revenuecat/internal/store"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// JSON coercion helpers.
// ---------------------------------------------------------------------------

// toFloatRC converts a JSON value (number, numeric string, or null) to a
// float64. Returns 0 on missing/unparseable so a single malformed row does not
// corrupt a rollup.
func toFloatRC(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	case string:
		var f float64
		_, _ = fmt.Sscanf(strings.TrimSpace(x), "%f", &f)
		return f
	}
	return 0
}

// toStringRC coerces a JSON value (string or number) to a string for ID
// comparisons.
func toStringRC(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return fmt.Sprintf("%.0f", x)
	case int:
		return fmt.Sprintf("%d", x)
	case int64:
		return fmt.Sprintf("%d", x)
	case json.Number:
		return x.String()
	}
	return ""
}

// monetaryGrossUSD extracts the gross amount from a RevenueCat MonetaryAmount
// object ({currency, gross, proceeds, ...}). RevenueCat reports
// total_revenue_in_usd already normalised to USD, so we read gross. Returns 0
// when the value is missing or not an object.
func monetaryGrossUSD(v any) float64 {
	m, ok := v.(map[string]any)
	if !ok {
		// Some endpoints may inline a bare number; fall back to coercion.
		return toFloatRC(v)
	}
	if g, ok := m["gross"]; ok {
		return toFloatRC(g)
	}
	return 0
}

// rcEpochMSToTime converts a ms-since-epoch value (number or numeric string)
// to a UTC time. Returns the zero time on missing/zero input.
func rcEpochMSToTime(v any) time.Time {
	ms := toFloatRC(v)
	if ms <= 0 {
		return time.Time{}
	}
	sec := int64(ms) / 1000
	nsec := (int64(ms) % 1000) * int64(time.Millisecond)
	return time.Unix(sec, nsec).UTC()
}

// ---------------------------------------------------------------------------
// Local-store loaders (lookup maps over synced resources).
//
// Each emits an os.Stderr warning when its LIMIT cap is hit so callers can
// distinguish "no data" from "scan saturated; lookups may be missing".
// ---------------------------------------------------------------------------

const (
	loadSubscriptionStatusCap = 500000
	loadEntitlementNamesCap   = 50000
	loadActiveEntsCap         = 1000000
)

// loadResourceRowsRC is the generic scan helper. It runs the local query and
// invokes apply for every row that decodes cleanly into a flat object map.
func loadResourceRowsRC(db *store.Store, resourceTypes []string, capRows int, helperName string, apply func(obj map[string]any)) {
	if len(resourceTypes) == 0 {
		return
	}
	placeholders := make([]string, len(resourceTypes))
	args := make([]any, 0, len(resourceTypes)+1)
	for i, rt := range resourceTypes {
		placeholders[i] = "?"
		args = append(args, rt)
	}
	args = append(args, capRows)
	query := fmt.Sprintf(
		`SELECT data FROM resources WHERE resource_type IN (%s) LIMIT ?`,
		strings.Join(placeholders, ","),
	)
	rows, err := db.Query(query, args...)
	if err != nil {
		return
	}
	defer rows.Close()
	loaded := 0
	for rows.Next() {
		loaded++
		var data sql.NullString
		if rows.Scan(&data) != nil || !data.Valid {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(data.String), &obj) != nil {
			continue
		}
		apply(obj)
	}
	if err := rows.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %s iteration failed (%v); results may be incomplete\n", helperName, err)
	}
	if loaded >= capRows {
		fmt.Fprintf(os.Stderr, "warning: %s hit %d-row cap; lookups may be missing for resources beyond the cap\n", helperName, capRows)
	}
}

// loadSubscriptionStatusRC returns subscription-id → status for every
// subscription in the local mirror (both top-level and customer-scoped).
func loadSubscriptionStatusRC(db *store.Store) map[string]string {
	out := map[string]string{}
	loadResourceRowsRC(db, []string{"subscriptions", "customers_subscriptions"}, loadSubscriptionStatusCap, "loadSubscriptionStatusRC", func(obj map[string]any) {
		id := toStringRC(obj["id"])
		if id == "" {
			return
		}
		if status, ok := obj["status"].(string); ok {
			out[id] = status
		}
	})
	return out
}

// loadEntitlementNamesRC returns entitlement-id → display_name (and lookup_key
// fallback) for every project entitlement in the local mirror.
func loadEntitlementNamesRC(db *store.Store) map[string]string {
	out := map[string]string{}
	loadResourceRowsRC(db, []string{"entitlements"}, loadEntitlementNamesCap, "loadEntitlementNamesRC", func(obj map[string]any) {
		id := toStringRC(obj["id"])
		if id == "" {
			return
		}
		if name, ok := obj["display_name"].(string); ok && name != "" {
			out[id] = name
			return
		}
		if lk, ok := obj["lookup_key"].(string); ok {
			out[id] = lk
		}
	})
	return out
}

// ---------------------------------------------------------------------------
// Chart-series parsing.
//
// /charts/{chart_name} returns a chart_data object whose `values` is an array
// of row objects shaped like:
//
//	{"cohort": 1746057600, "incomplete": false, "measure": 0, "value": 0.0}
//
// `cohort` is the period start as Unix SECONDS (NOT ms — unlike subscription
// timestamps elsewhere in this API). `measure` is the 0-based index into the
// chart's `measures` array (multi-measure charts like mrr_movement emit one row
// per measure per cohort). `value` is that measure's numeric value. We group
// rows by cohort and place each measure's value at its index, producing one
// chartPoint per period with Values ordered by measure index.
// ---------------------------------------------------------------------------

// chartData is the decoded /charts/{chart_name} response.
type chartData struct {
	Object        string            `json:"object"`
	Category      string            `json:"category"`
	DisplayName   string            `json:"display_name"`
	Resolution    string            `json:"resolution"`
	YAxisCurrency string            `json:"yaxis_currency"`
	Values        []json.RawMessage `json:"values"`
	Summary       map[string]any    `json:"summary"`
}

// chartPoint is one normalised time-series row: a period start plus the numeric
// series values for that period (positional, as the API returns them).
type chartPoint struct {
	When   time.Time
	Values []float64
}

// parseChartData decodes a /charts/{chart_name} body into chartData. Returns a
// zero chartData (not an error) on a body that does not match so a single
// degenerate chart does not abort a multi-chart join.
func parseChartData(raw json.RawMessage) chartData {
	var cd chartData
	_ = json.Unmarshal(raw, &cd)
	return cd
}

// chartRow is one decoded RC chart value row.
type chartRow struct {
	Cohort  *float64 `json:"cohort"`
	Measure int      `json:"measure"`
	Value   float64  `json:"value"`
}

// points groups the raw `values` rows by cohort (period start, Unix seconds)
// and returns one chartPoint per period in chronological order, with Values
// indexed by measure. firstSeriesValue() therefore returns measure 0 (the
// primary series, e.g. MRR for the mrr chart).
func (cd chartData) points() []chartPoint {
	type agg struct {
		when   time.Time
		byIdx  map[int]float64
		maxIdx int
	}
	groups := map[int64]*agg{}
	order := make([]int64, 0, len(cd.Values))
	for _, rawRow := range cd.Values {
		var row chartRow
		if json.Unmarshal(rawRow, &row) != nil || row.Cohort == nil {
			continue
		}
		cohort := int64(*row.Cohort)
		g, ok := groups[cohort]
		if !ok {
			// cohort is Unix SECONDS, not ms — decode directly.
			g = &agg{when: time.Unix(cohort, 0).UTC(), byIdx: map[int]float64{}}
			groups[cohort] = g
			order = append(order, cohort)
		}
		g.byIdx[row.Measure] = row.Value
		if row.Measure > g.maxIdx {
			g.maxIdx = row.Measure
		}
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	out := make([]chartPoint, 0, len(order))
	for _, cohort := range order {
		g := groups[cohort]
		vals := make([]float64, g.maxIdx+1)
		for idx, v := range g.byIdx {
			vals[idx] = v
		}
		out = append(out, chartPoint{When: g.when, Values: vals})
	}
	return out
}

// firstSeriesValue returns the first numeric series value of a chart point
// (the value after the leading timestamp), or 0 when the point has no series.
func (p chartPoint) firstSeriesValue() float64 {
	if len(p.Values) == 0 {
		return 0
	}
	return p.Values[0]
}

// ---------------------------------------------------------------------------
// Multi-resource sync hint helper (RC analog of LS hintIfMultiUnsynced).
// ---------------------------------------------------------------------------

// hintIfMultiUnsynced runs hintIfUnsynced for the primary then each secondary
// resource; if ANY is unsynced the unsynced hint fires and we stop. Otherwise
// hintIfStale runs against the primary so callers see freshness warnings on the
// most authoritative table.
func hintIfMultiUnsynced(cmd *cobra.Command, db *store.Store, primary string, secondaries []string, maxAge time.Duration) {
	if hintIfUnsynced(cmd, db, primary) {
		return
	}
	for _, r := range secondaries {
		if hintIfUnsynced(cmd, db, r) {
			return
		}
	}
	hintIfStale(cmd, db, primary, maxAge)
}
