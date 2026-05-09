// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.
// PATCH(library-side): added by U3 of the digg search/roster plan to host
// the parser for Digg's `firstPostAge` strings ("2d", "26d", "5h") and the
// shared `--since` flag accepted by `search` and (future) other read
// commands. Lives here rather than next to the search command so other
// units (notably U2's `authors get`) can reuse it.

package cliutil

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseDiggAge converts a Digg-style age string into a time.Duration. It
// accepts the formats Digg's /api/search/stories `firstPostAge` field
// returns (`Nh`, `Nd`, `Nw`) plus `Nm` for months (treated as 30 days)
// so the same parser can back the user-facing `--since` flag without a
// second helper.
//
// Conventions:
//   - input is case-insensitive: `5H` and `5h` parse identically.
//   - the integer must be > 0; `0d`, `-3d` are rejected. A non-positive
//     duration is never a useful filter input and matches Digg's own
//     behavior (firstPostAge is never `"0h"`).
//   - the suffix is required; `"5"` (no unit) is rejected so a typo
//     can't silently be parsed as 5 nanoseconds via time.ParseDuration.
//   - whitespace is trimmed; `" 7d "` is valid.
//
// Caller policy: this helper returns an error on malformed input. The
// FILTER step decides whether to drop or keep on parse failure (the
// `search` command keeps; see digg_commands.go). That separation matters
// because Digg sometimes returns unexpected age formats from edge-case
// clusters and silently dropping them is a worse default than surfacing
// them with their original string.
func ParseDiggAge(s string) (time.Duration, error) {
	in := strings.TrimSpace(strings.ToLower(s))
	if in == "" {
		return 0, fmt.Errorf("empty duration")
	}
	// Last byte is the unit; everything before is the integer. Reject
	// inputs shorter than 2 chars (no unit) or with a non-suffix unit.
	if len(in) < 2 {
		return 0, fmt.Errorf("malformed duration %q: missing unit (h/d/w/m)", s)
	}
	unit := in[len(in)-1]
	num := in[:len(in)-1]
	n, err := strconv.Atoi(num)
	if err != nil {
		return 0, fmt.Errorf("malformed duration %q: %w", s, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("duration %q must be positive", s)
	}
	switch unit {
	case 'h':
		return time.Duration(n) * time.Hour, nil
	case 'd':
		return time.Duration(n) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	case 'm':
		// "Nm" means months in this context, NOT minutes. Digg's
		// `firstPostAge` never returns minute granularity (the API
		// rounds up to the nearest hour) and the `--since` use case is
		// last-N-days/weeks/months timing, never last-N-minutes. We
		// pick 30 days/month as the conventional approximation.
		return time.Duration(n) * 30 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("malformed duration %q: unknown unit %q (want h/d/w/m)", s, string(unit))
	}
}
