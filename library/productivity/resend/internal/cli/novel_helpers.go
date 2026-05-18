// PATCH: shared helper for novel commands — parses Resend timestamps that
// mix RFC3339 ("2026-05-14T16:28:52Z") with Postgres-style space-separated
// ("2025-11-04 16:28:52.015491+00") representations across endpoints.
package cli

import (
	"strings"
	"time"
)

// parseTimestamp accepts the timestamp formats Resend returns across endpoints:
// canonical RFC3339, RFC3339 with nanoseconds, Postgres-style space separator
// with fractional seconds and "+00" / "+0000" timezone shorthand. Returns
// (zero, false) when none of the supported formats parse cleanly.
func parseTimestamp(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999-07",
		"2006-01-02 15:04:05.999999-0700",
		"2006-01-02 15:04:05-07",
		"2006-01-02 15:04:05-0700",
		"2006-01-02 15:04:05Z",
	}
	// Normalize "+00" suffix to "+0000" so it matches the -0700 layout token.
	normalized := s
	if len(normalized) >= 3 {
		tail := normalized[len(normalized)-3:]
		if tail == "+00" || tail == "-00" {
			normalized = normalized + "00"
		}
	}
	for _, layout := range formats {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
		if t, err := time.Parse(layout, normalized); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
