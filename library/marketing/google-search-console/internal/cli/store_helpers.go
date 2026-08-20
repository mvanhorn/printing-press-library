// Copyright 2026 Matt and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: shared helpers for the transcendence commands.

package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-search-console/internal/store"
)

// storeDBPathOverride is set via PRINTING_PRESS_STORE_DB env var (mainly for
// tests, so a smoke-test script can point every transcendence command at a
// fixture DB without collateral damage to the user's real cache).
const storeDBPathEnv = "PRINTING_PRESS_STORE_DB"

// openStore opens the canonical SQLite store, honoring an env override for
// fixture-based tests.
func openStore(ctx context.Context) (*store.Store, error) {
	path := os.Getenv(storeDBPathEnv)
	return store.Open(ctx, path)
}

// emptyStoreErr is the standard error message when a transcendence command
// runs against an empty corpus. Tells the caller exactly what to do.
func emptyStoreErr(siteURL string) error {
	if siteURL == "" {
		return fmt.Errorf("local store has no synced data; run `google-search-console-pp-cli sync --site <site-url> --last 90d` first")
	}
	return fmt.Errorf("local store has no rows for %q; run `google-search-console-pp-cli sync --site %s --last 90d` first", siteURL, siteURL)
}

// parseLast parses durations like "90d", "12w", "3m", "1y" into a time.Duration.
// Used by --last, --since, --window, --days flags.
//
//	"7d"  -> 7  days
//	"4w"  -> 28 days
//	"3m"  -> 90 days  (1m == 30d)
//	"1y"  -> 365 days
//	"24h" -> 24 hours (Go-native suffix)
//
// Bare integers ("90") are treated as days for forgiving CLI parsing.
func parseLast(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	// Try Go's native parser first ("24h", "30m", "5s")
	if d, err := time.ParseDuration(s); err == nil && !strings.HasSuffix(s, "m") {
		// time.ParseDuration treats "5m" as 5 minutes — we want 5 months.
		// So we only accept the native parser when the suffix isn't ambiguous.
		return d, nil
	}
	last := s[len(s)-1]
	num := s[:len(s)-1]
	switch last {
	case 'd', 'D':
		n, err := strconv.Atoi(num)
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	case 'w', 'W':
		n, err := strconv.Atoi(num)
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	case 'm', 'M':
		n, err := strconv.Atoi(num)
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 30 * 24 * time.Hour, nil
	case 'y', 'Y':
		n, err := strconv.Atoi(num)
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 365 * 24 * time.Hour, nil
	default:
		// Bare integer = days.
		if n, err := strconv.Atoi(s); err == nil {
			return time.Duration(n) * 24 * time.Hour, nil
		}
		return 0, fmt.Errorf("unrecognized duration %q (try '7d', '4w', '3m', '1y', or a Go duration like '24h')", s)
	}
}

// dateRange returns (startDate, endDate) inclusive, both as YYYY-MM-DD strings,
// for a window of length d ending today.
func dateRange(d time.Duration) (string, string) {
	end := time.Now().UTC()
	start := end.Add(-d)
	return start.Format("2006-01-02"), end.Format("2006-01-02")
}

// daysAgo formats today minus n days as YYYY-MM-DD.
func daysAgo(n int) string {
	return time.Now().UTC().AddDate(0, 0, -n).Format("2006-01-02")
}
