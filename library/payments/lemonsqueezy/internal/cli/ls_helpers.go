// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

// Shared helpers for hand-authored Lemon Squeezy transcendence commands.
// Lemon Squeezy's JSON:API responses encode numeric attributes inconsistently
// across the resource set (raw numbers for store counters, strings for some
// amounts), so a small adapter layer keeps the per-feature code thin.

package cli

import (
	"fmt"
	"strings"
	"time"
)

// toFloatLS converts a JSON value (number, JSON-encoded numeric string, or null)
// to a float64. Returns 0 on missing/unparseable rather than erroring so a single
// malformed row does not corrupt a whole rollup.
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

// toBoolLS coerces a JSON value to a bool. Lemon Squeezy uses true/false
// booleans for refund flags; this preserves that and treats anything else as false.
func toBoolLS(v any) bool {
	if b, ok := v.(bool); ok {
		return b
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
