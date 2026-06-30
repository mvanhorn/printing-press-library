// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored shared helpers for the Zoho Desk "transcendence" commands
// (sla-radar, agent-load, triage, since, contact-360, morning, rebalance,
// breach-history). Keeps store reads, time parsing, and name resolution in
// one place so the command bodies stay lean.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/zoho-desk/internal/store"
)

// parseZohoTime parses a Zoho ISO-8601 timestamp (e.g.
// "2024-05-01T10:00:00.000Z"). It tries RFC3339Nano first, then RFC3339,
// and reports ok=false for empty or unparseable input.
func parseZohoTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// priorityWeight maps a Zoho priority string to a numeric weight. Unknown or
// empty priorities default to 1 (treated as Low).
func priorityWeight(p string) int {
	switch strings.TrimSpace(p) {
	case "High":
		return 3
	case "Medium":
		return 2
	case "Low":
		return 1
	default:
		return 1
	}
}

// isClosedStatus reports whether a ticket status counts as resolved/closed.
func isClosedStatus(status string) bool {
	// Match common terminal statuses, not just the literal "Closed". Many
	// portals add "Resolved" or other closed-equivalent statuses; treating
	// them as open silently corrupts every analytics command.
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "closed", "resolved":
		return true
	}
	return false
}

// str coerces a JSON map value to a string: "" when absent or nil, otherwise
// fmt.Sprintf("%v").
func str(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// loadByType reads every row of the given resource_type from the resources
// table into []map[string]any. The id column backfills a missing JSON "id".
func loadByType(ctx context.Context, db *store.Store, rt string) ([]map[string]any, error) {
	rows, err := db.DB().QueryContext(ctx, `SELECT id, data FROM resources WHERE resource_type = ?`, rt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]map[string]any, 0)
	for rows.Next() {
		var id string
		var data []byte
		if err := rows.Scan(&id, &data); err != nil {
			continue
		}
		var m map[string]any
		if json.Unmarshal(data, &m) != nil || m == nil {
			continue
		}
		if str(m, "id") == "" {
			m["id"] = id
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// loadTickets reads all synced tickets.
func loadTickets(ctx context.Context, db *store.Store) ([]map[string]any, error) {
	return loadByType(ctx, db, "tickets")
}

// agentNames returns an agentId -> "First Last" lookup from synced agents.
func agentNames(ctx context.Context, db *store.Store) map[string]string {
	out := map[string]string{}
	agents, err := loadByType(ctx, db, "agents")
	if err != nil {
		return out
	}
	for _, a := range agents {
		id := str(a, "id")
		if id == "" {
			continue
		}
		out[id] = strings.TrimSpace(str(a, "firstName") + " " + str(a, "lastName"))
	}
	return out
}

// departmentNames returns a departmentId -> name lookup from synced departments.
func departmentNames(ctx context.Context, db *store.Store) map[string]string {
	out := map[string]string{}
	depts, err := loadByType(ctx, db, "departments")
	if err != nil {
		return out
	}
	for _, d := range depts {
		id := str(d, "id")
		if id == "" {
			continue
		}
		out[id] = str(d, "name")
	}
	return out
}

// medianFloat returns the median of vals (0 for empty input). vals is sorted
// in place by the caller's copy semantics — callers pass a throwaway slice.
func medianFloat(vals []float64) float64 {
	n := len(vals)
	if n == 0 {
		return 0
	}
	// simple insertion sort to avoid pulling sort into every caller's intent;
	// inputs here are small (one entry per agent).
	for i := 1; i < n; i++ {
		for j := i; j > 0 && vals[j-1] > vals[j]; j-- {
			vals[j-1], vals[j] = vals[j], vals[j-1]
		}
	}
	if n%2 == 1 {
		return vals[n/2]
	}
	return (vals[n/2-1] + vals[n/2]) / 2
}

// round1 rounds to 1 decimal place without importing math at every call site.
func round1(f float64) float64 {
	return float64(int64(f*10+sign(f)*0.5)) / 10
}

// round2 rounds to 2 decimal places.
func round2(f float64) float64 {
	return float64(int64(f*100+sign(f)*0.5)) / 100
}

func sign(f float64) float64 {
	if f < 0 {
		return -1
	}
	return 1
}
