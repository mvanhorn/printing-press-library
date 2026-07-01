// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// Shared helpers for the hand-written novel commands (site health,
// underperformance, changes, equipment faults, budget status). Kept in
// their own file per the regen-merge durability convention.

package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/monitoring/solaredge/internal/store"
)

// solarEdgeDailyRequestLimit is the vendor-documented daily quota: 300
// requests per (account-token, siteId) pair from the same source IP. The
// API exposes no header or endpoint for the remaining count, which is why
// 'budget status' has to track it locally instead of reading it from a
// response.
const solarEdgeDailyRequestLimit = 300

// solarEdgeEnergyHistoryCapDays is the request window used by 'site
// underperformance' for its single /energy?timeUnit=DAY call. The API caps
// that endpoint at a 1-year window (see the vendor docs' "Site Energy"
// usage limitation); 364 instead of 365 leaves a 1-day margin so an
// off-by-one in how the API counts inclusive date boundaries can't push
// the request over the limit and turn into an HTTP 403.
const solarEdgeEnergyHistoryCapDays = 364

// solarEdgeMinBaselineDays is the minimum number of non-null baseline days
// 'site underperformance' wants before it trusts the computed mean enough
// to flag days against it. Below this, a single missing-data day skews the
// average too much to be a meaningful comparison.
const solarEdgeMinBaselineDays = 7

// solarEdgeMaxUnderperformanceSinceDays caps the --since window 'site
// underperformance' will check, so a very large --since value (e.g. "999d")
// can't push recentDays past the size of the 364-day history call and
// leave no baseline at all.
const solarEdgeMaxUnderperformanceSinceDays = 300

// solarEdgeMaxChangesSinceDays caps the --since window 'site changes' will
// compare, keeping the underlying 2x-window /energy call (and the
// HTTP-403 risk if it ever exceeded the API's 1-year limit) bounded.
const solarEdgeMaxChangesSinceDays = 180

// extractStringField reads a string field from a decoded JSON object,
// returning ok=false if the key is absent or not a string.
func extractStringField(obj map[string]json.RawMessage, key string) (string, bool) {
	raw, ok := obj[key]
	if !ok {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}

// recordSolarEdgeCalls is a best-effort write to the local call-log table.
// Failures are swallowed: the budget tracker is a convenience feature, and
// a local-store write error must never fail the live command that already
// succeeded against the real API.
func recordSolarEdgeCalls(ctx context.Context, siteID string, n int) {
	dbPath := defaultDBPath("solaredge-pp-cli")
	if dbPath == "" {
		return
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return
	}
	defer db.Close()
	_ = store.RecordSolarEdgeAPICalls(db.DB(), siteID, n)
}

// countInventoryEquipment sums every equipment category in a decoded
// Inventory object. Shared by 'site health' (overall equipment count) and
// 'site changes' (current, non-delta equipment snapshot).
func countInventoryEquipment(invObj map[string]json.RawMessage) int {
	count := 0
	for _, key := range []string{"inverters", "batteries", "meters", "gateways", "sensors"} {
		raw, ok := invObj[key]
		if !ok {
			continue
		}
		var items []json.RawMessage
		if json.Unmarshal(raw, &items) == nil {
			count += len(items)
		}
	}
	return count
}

// energyDayPoint is one {date, value} entry from a /site/{siteId}/energy
// response with timeUnit=DAY. value is a pointer because the API returns
// null for days with no data.
type energyDayPoint struct {
	Date  string   `json:"date"`
	Value *float64 `json:"value"`
}

// parseEnergySeriesValues unwraps an "energy"-rooted response envelope and
// decodes its "values" array. Shared by 'site underperformance' and 'site
// changes', which both make a single /energy?timeUnit=DAY call and then
// slice the resulting day-by-day series differently.
func parseEnergySeriesValues(energyRaw json.RawMessage) ([]energyDayPoint, error) {
	var energyObj map[string]json.RawMessage
	if err := json.Unmarshal(applyResponsePath(energyRaw, "energy"), &energyObj); err != nil {
		return nil, fmt.Errorf("parsing energy response: %w", err)
	}
	valuesRaw, ok := energyObj["values"]
	if !ok {
		return nil, nil
	}
	var points []energyDayPoint
	if err := json.Unmarshal(valuesRaw, &points); err != nil {
		return nil, fmt.Errorf("parsing energy values: %w", err)
	}
	return points, nil
}

// sumEnergyPoints totals the non-null values in points and reports how many
// of them had data. Shared by 'site underperformance' (baseline mean) and
// 'site changes' (period totals).
func sumEnergyPoints(points []energyDayPoint) (sum float64, nonNullCount int) {
	for _, p := range points {
		if p.Value != nil {
			sum += *p.Value
			nonNullCount++
		}
	}
	return sum, nonNullCount
}
