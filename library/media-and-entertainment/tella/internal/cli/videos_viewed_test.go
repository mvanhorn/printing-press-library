// Copyright 2026 gregce. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
	"time"
)

// TestParseSinceWindow pins the round-8 fix: unrecognized --since values
// must produce a parse error instead of silently expanding the rollup
// query to the entire event stream. Before this change, `--since yesterday`
// or `--since "2 weeks"` returned zero time, the caller treated zero as
// "no window," and the user saw an unbounded rollup with no warning.
func TestParseSinceWindow(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantErr   bool
		wantZero  bool
		wantDelta time.Duration
	}{
		// Valid: explicit empty string is the documented "no window" sentinel.
		{"empty string is no-window", "", false, true, 0},
		{"whitespace only is no-window", "   ", false, true, 0},
		// Valid: Go duration strings.
		{"go duration 1h", "1h", false, false, 1 * time.Hour},
		{"go duration 90m", "90m", false, false, 90 * time.Minute},
		{"go duration compound 1h30m", "1h30m", false, false, 90 * time.Minute},
		// Valid: shorthand <N><unit>.
		{"shorthand 7d", "7d", false, false, 7 * 24 * time.Hour},
		{"shorthand 2w", "2w", false, false, 14 * 24 * time.Hour},
		{"shorthand 15m", "15m", false, false, 15 * time.Minute},
		// Invalid: must error rather than silently widen the rollup.
		{"english word yesterday", "yesterday", true, false, 0},
		{"phrase with space", "2 weeks", true, false, 0},
		{"bad unit suffix", "5y", true, false, 0},
		{"trailing garbage", "7d_extra", true, false, 0},
		{"non-numeric prefix", "ab7d", true, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := time.Now()
			got, err := parseSinceWindow(tc.input)
			after := time.Now()

			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseSinceWindow(%q) = (%v, nil), want error", tc.input, got)
				}
				if !got.IsZero() {
					t.Fatalf("parseSinceWindow(%q) returned non-zero time %v on error path, want zero", tc.input, got)
				}
				// Error message must name the input so the user can see
				// what was rejected.
				if !strings.Contains(err.Error(), tc.input) && strings.TrimSpace(tc.input) != "" {
					t.Fatalf("parseSinceWindow(%q) error %q does not mention the input", tc.input, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSinceWindow(%q): unexpected error %v", tc.input, err)
			}
			if tc.wantZero {
				if !got.IsZero() {
					t.Fatalf("parseSinceWindow(%q) = %v, want zero time", tc.input, got)
				}
				return
			}
			// Bound check: parsed start must be within
			// [before - delta, after - delta]. before/after sandwich the
			// time.Now() inside parseSinceWindow, so the parsed start can
			// drift at most by (after-before) ~ a few microseconds.
			lowest := before.Add(-tc.wantDelta)
			highest := after.Add(-tc.wantDelta)
			if got.Before(lowest) || got.After(highest) {
				t.Fatalf("parseSinceWindow(%q) = %v, want within [%v, %v]", tc.input, got, lowest, highest)
			}
		})
	}
}
