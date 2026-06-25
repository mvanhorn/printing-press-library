// Copyright 2026 adam-birddog and contributors. Licensed under Apache-2.0. See LICENSE.

// Shared helpers for the Sutra studio-analytics commands (scorecard, no-shows,
// utilization, expiring, churn, revenue, referral-funnel, ltv). Every analytic
// reads the local SQLite mirror and joins across the typed tables the sync
// command populates — the Sutra API exposes no reporting endpoints, so these
// reports exist only locally.
//
// pp:data-source local
package cli

import (
	"context"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/sutra-fitness/internal/store"
)

// openAnalyticsStore resolves the DB path and opens the local store read-side
// for an analytics command. ready=false means the mirror file does not exist
// yet; the caller emits its empty result and returns nil. This is the
// missing-mirror guard from the build checklist: a missing local cache is an
// empty state, not an error.
func openAnalyticsStore(ctx context.Context, cmd *cobra.Command, dbPath string) (db *store.Store, ready bool, err error) {
	if dbPath == "" {
		dbPath = defaultDBPath("sutra-fitness-pp-cli")
	}
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"no local mirror at %s\nrun: sutra-fitness-pp-cli sync\n", dbPath)
		return nil, false, nil
	}
	store, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, false, fmt.Errorf("opening local database: %w", err)
	}
	return store, true, nil
}

// emitAnalytics writes a Go value through the shared output pipeline so
// --json, --agent, --select, --compact, --csv, and --quiet all behave the same
// as on generated commands. In a terminal, array results render as a table and
// object results pretty-print.
func emitAnalytics(cmd *cobra.Command, flags *rootFlags, v any) error {
	return printJSONFiltered(cmd.OutOrStdout(), v, flags)
}

// round2 rounds a float to 2 decimals for stable report output. Uses
// math.Round so negative deltas round correctly (a naive int64(f*100+0.5)
// truncates toward zero and turns -25.0 into -24.99).
func round2(f float64) float64 {
	return math.Round(f*100) / 100
}

// pct returns 100*part/whole rounded to 2 decimals, or 0 when whole is 0.
func pct(part, whole int) float64 {
	if whole == 0 {
		return 0
	}
	return round2(float64(part) / float64(whole) * 100)
}

// parseLocalTime parses the ISO-8601 / RFC3339 datetimes Sutra returns,
// tolerating the date-only and space-separated SQLite shapes. Returns ok=false
// for empty or unparseable values so callers treat them as missing.
func parseLocalTime(s string) (time.Time, bool) {
	s = trimQuotes(s)
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func trimQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
