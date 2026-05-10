// uk-train-goat hand-authored: human-readable duration parser for --in / --within.
package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// parseDurationMinutes parses a user-friendly duration string into minutes.
// Accepts:
//   - "30"           → 30 minutes (bare integer)
//   - "30m" / "1h"   → Go ParseDuration shape
//   - "1h30m" / "2h" → composed forms
//   - ""             → 0 (no constraint)
// Returns an error for malformed inputs so the CLI surfaces a clean
// usage error rather than silently defaulting.
func parseDurationMinutes(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	// Bare integer means minutes (preserves backwards-compatible expectation
	// from caminad/ldb-cli's --time-offset behaviour).
	if n, err := strconv.Atoi(s); err == nil {
		if n < 0 {
			return 0, fmt.Errorf("duration cannot be negative: %s", s)
		}
		return n, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q (use formats like '30', '30m', '1h', '1h30m'): %w", s, err)
	}
	if d < 0 {
		return 0, fmt.Errorf("duration cannot be negative: %s", s)
	}
	return int(d.Minutes()), nil
}
