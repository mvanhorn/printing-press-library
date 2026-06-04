// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

// Shared helpers for hand-authored Lemon Squeezy transcendence commands.
// Lemon Squeezy's JSON:API responses encode numeric attributes inconsistently
// across the resource set (raw numbers for store counters, strings for some
// amounts), so a small adapter layer keeps the per-feature code thin.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/lemonsqueezy/internal/store"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// JSON coercion helpers.
//
// Lemon Squeezy returns numeric attributes as either JSON numbers or strings
// across endpoints (e.g. store_id is numeric on /stores but stringly on
// /orders.attributes.store_id), so the analytics commands route every typed
// extraction through these helpers rather than declaring concrete types in
// each envelope struct.
// ---------------------------------------------------------------------------

// toFloatLS converts a JSON value (number, JSON-encoded numeric string, or
// null) to a float64. Returns 0 on missing/unparseable rather than erroring so
// a single malformed row does not corrupt a whole rollup.
func toFloatLS(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case string:
		var f float64
		_, _ = fmt.Sscanf(strings.TrimSpace(x), "%f", &f)
		return f
	}
	return 0
}

// toStringLS coerces a JSON value (string, number) to a string for ID
// comparisons. Lemon Squeezy returns numeric IDs as JSON numbers in attribute
// positions but as strings in 'data.id'; this normalises both.
func toStringLS(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return fmt.Sprintf("%.0f", x)
	case int:
		return fmt.Sprintf("%d", x)
	case int64:
		return fmt.Sprintf("%d", x)
	}
	return ""
}

// toBoolLS coerces a JSON value to a bool. Accepts native bools (read
// responses), 0/1 integers (some LS write responses echo back numeric
// booleans), and "true"/"false"/"1"/"0" strings. Anything else returns false.
func toBoolLS(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case int:
		return x != 0
	case int64:
		return x != 0
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case "true", "1", "yes":
			return true
		}
	}
	return false
}

// parseLSTime parses a Lemon Squeezy ISO8601 timestamp. Returns the zero time
// on parse failure.
func parseLSTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// ---------------------------------------------------------------------------
// Resource loaders.
//
// Every analytics command needs cheap lookup maps over locally-synced
// resources (customer-id → email, subscription-id → status, variant-id →
// name, etc). These helpers consolidate the boilerplate scan-and-build
// pattern in one place; each is shared across multiple commands.
//
// Every helper emits an `os.Stderr` warning when its `LIMIT` cap is hit so
// callers can distinguish "no data" from "scan saturated; lookups may be
// missing".
// ---------------------------------------------------------------------------

// Conservative defaults that exceed the volume any single Lemon Squeezy
// store routinely produces; bump if real users hit the saturation warning.
const (
	loadCustomerEmailsCap      = 500000
	loadSubscriptionStatesCap  = 500000
	loadLastInvoiceBySubCap    = 500000
	loadVariantNamesCap        = 10000
	loadInstanceCountsByKeyCap = 200000
	loadRedemptionVelocityCap  = 1000000
)

// loadResourceRows is the generic scan helper every other loader builds on.
// It runs the local-store query, scans each row's JSON envelope, and invokes
// `apply` for every row that decodes cleanly. Hitting the cap surfaces a
// stderr warning that names the saturated helper so callers can tell apart
// "no data" from "scan saturated".
func loadResourceRows(db *store.Store, resourceType string, capRows int, helperName string, apply func(env map[string]json.RawMessage)) {
	rows, err := db.Query(
		`SELECT data FROM resources WHERE resource_type = ? LIMIT ?`,
		resourceType, capRows,
	)
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
		var env map[string]json.RawMessage
		if json.Unmarshal([]byte(data.String), &env) != nil {
			continue
		}
		apply(env)
	}
	if loaded >= capRows {
		fmt.Fprintf(os.Stderr, "warning: %s hit %d-row cap; lookups may be missing for resources beyond the cap\n", helperName, capRows)
	}
}

// extractIDAndAttributes pulls the top-level `id` and the `attributes`
// sub-document out of a JSON:API envelope decoded by loadResourceRows.
// Returns empty strings / nil on missing fields.
func extractIDAndAttributes(env map[string]json.RawMessage) (id string, attrs map[string]any) {
	if raw, ok := env["id"]; ok {
		_ = json.Unmarshal(raw, &id)
	}
	if raw, ok := env["attributes"]; ok {
		_ = json.Unmarshal(raw, &attrs)
	}
	return id, attrs
}

// loadCustomerEmails returns customer-id → email for every customer in the
// local mirror.
func loadCustomerEmails(db *store.Store) map[string]string {
	out := map[string]string{}
	loadResourceRows(db, "customers", loadCustomerEmailsCap, "loadCustomerEmails", func(env map[string]json.RawMessage) {
		id, attrs := extractIDAndAttributes(env)
		if id == "" {
			return
		}
		if email, ok := attrs["email"].(string); ok {
			out[id] = email
		}
	})
	return out
}

// loadSubscriptionStates returns subscription-id → status for every
// subscription in the local mirror.
func loadSubscriptionStates(db *store.Store) map[string]string {
	out := map[string]string{}
	loadResourceRows(db, "subscriptions", loadSubscriptionStatesCap, "loadSubscriptionStates", func(env map[string]json.RawMessage) {
		id, attrs := extractIDAndAttributes(env)
		if id == "" {
			return
		}
		if status, ok := attrs["status"].(string); ok {
			out[id] = status
		}
	})
	return out
}

// loadSubscriptionCreatedAt returns subscription-id → created_at time. Used
// by mrr-trend to classify the earliest in-window invoice as "new" only
// when the parent sub was created in the window — robust to invoice-scan
// truncation because the sub-level signal is independent of invoice volume.
func loadSubscriptionCreatedAt(db *store.Store) map[string]time.Time {
	out := map[string]time.Time{}
	loadResourceRows(db, "subscriptions", loadSubscriptionStatesCap, "loadSubscriptionCreatedAt", func(env map[string]json.RawMessage) {
		id, attrs := extractIDAndAttributes(env)
		if id == "" {
			return
		}
		when := parseLSTime(toStringLS(attrs["created_at"]))
		if !when.IsZero() {
			out[id] = when
		}
	})
	return out
}

// loadLastInvoiceBySub returns subscription-id → most-recent paid invoice USD
// amount. Used by churn-watch to estimate dollar exposure per churned sub.
func loadLastInvoiceBySub(db *store.Store) map[string]float64 {
	type stamp struct {
		when time.Time
		amt  float64
	}
	tmp := map[string]stamp{}
	loadResourceRows(db, "subscription-invoices", loadLastInvoiceBySubCap, "loadLastInvoiceBySub", func(env map[string]json.RawMessage) {
		_, attrs := extractIDAndAttributes(env)
		if attrs == nil {
			return
		}
		if status, ok := attrs["status"].(string); ok && status != "" && status != "paid" {
			return
		}
		subID := toStringLS(attrs["subscription_id"])
		if subID == "" {
			return
		}
		when := parseLSTime(toStringLS(attrs["created_at"]))
		amt := toFloatLS(attrs["total_usd"])
		if amt == 0 {
			amt = toFloatLS(attrs["total"])
		}
		if cur, ok := tmp[subID]; !ok || when.After(cur.when) {
			tmp[subID] = stamp{when: when, amt: amt / 100.0}
		}
	})
	out := make(map[string]float64, len(tmp))
	for k, v := range tmp {
		out[k] = v.amt
	}
	return out
}

// loadVariantNames returns variant-id → variant name.
func loadVariantNames(db *store.Store) map[string]string {
	out := map[string]string{}
	loadResourceRows(db, "variants", loadVariantNamesCap, "loadVariantNames", func(env map[string]json.RawMessage) {
		id, attrs := extractIDAndAttributes(env)
		if id == "" {
			return
		}
		if name, ok := attrs["name"].(string); ok {
			out[id] = name
		}
	})
	return out
}

// loadInstanceCountsByKey returns license-key-id → count of activation
// instances. Used by license-rollup to compute per-key activation totals
// from the locally-mirrored license-key-instances resource.
func loadInstanceCountsByKey(db *store.Store) map[string]int {
	out := map[string]int{}
	loadResourceRows(db, "license-key-instances", loadInstanceCountsByKeyCap, "loadInstanceCountsByKey", func(env map[string]json.RawMessage) {
		_, attrs := extractIDAndAttributes(env)
		if attrs == nil {
			return
		}
		keyID := toStringLS(attrs["license_key_id"])
		if keyID != "" {
			out[keyID]++
		}
	})
	return out
}

// velocityStat captures a discount code's redemption pace. Velocity24h is the
// number of redemptions in the last 24 hours; Total is the all-time count
// from the local mirror.
type velocityStat struct {
	Velocity24h float64
	Total       float64
}

// loadRedemptionVelocityByDiscount returns discount-id → velocityStat. Used
// by campaign-watch to project sellout time.
func loadRedemptionVelocityByDiscount(db *store.Store, now time.Time) map[string]velocityStat {
	out := map[string]velocityStat{}
	cutoff := now.Add(-24 * time.Hour)
	loadResourceRows(db, "discount-redemptions", loadRedemptionVelocityCap, "loadRedemptionVelocityByDiscount", func(env map[string]json.RawMessage) {
		_, attrs := extractIDAndAttributes(env)
		if attrs == nil {
			return
		}
		dID := toStringLS(attrs["discount_id"])
		if dID == "" {
			return
		}
		cur := out[dID]
		cur.Total++
		when := parseLSTime(toStringLS(attrs["created_at"]))
		if !when.IsZero() && when.After(cutoff) {
			cur.Velocity24h++
		}
		out[dID] = cur
	})
	return out
}

// ---------------------------------------------------------------------------
// Multi-resource sync hint helper.
//
// Analytics commands routinely join 2-4 local resources (e.g. churn-watch
// uses subscriptions + customers + subscription-invoices). hintIfMultiUnsynced
// runs hintIfUnsynced for every required resource — if ANY of them isn't
// synced yet, the unsynced hint fires for that resource; otherwise
// hintIfStale runs against the primary so users see "data is N hours old"
// warnings on the most authoritative table.
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
