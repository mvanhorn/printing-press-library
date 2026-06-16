// Copyright 2026 Dickie and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored transcendence support: shared helpers for the local-mirror
// analytics commands (ladder, concentration, book). Not generated.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/ts/internal/store"
)

// activeHoldingWhere filters to live positions: money currently invested.
// Redeemed holdings have already paid back; cancelled never issued.
const activeHoldingWhere = `status NOT IN ('Redeemed', 'Cancelled')`

// openMirror opens the local SQLite mirror. If the mirror file does not exist
// yet it prints a sync hint and returns (nil, false, nil) so the caller can
// emit an empty result without erroring — a missing mirror is an empty-cache
// state, not a failure. Real open errors return (nil, false, err).
func openMirror(ctx context.Context, cmd *cobra.Command, dbPath string) (*store.Store, bool, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("ts-pp-cli")
	}
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: ts-pp-cli sync --db %s\n", dbPath, dbPath)
		return nil, false, nil
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, false, fmt.Errorf("opening database: %w", err)
	}
	return db, true, nil
}

// jsonMode reports whether output should be machine JSON rather than a human
// table: explicit --json/--agent, or output that is not a terminal (piped).
func jsonMode(cmd *cobra.Command, flags *rootFlags) bool {
	return flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout())
}

// emitEmpty writes a valid empty JSON result for agents and nothing for humans.
func emitEmpty(cmd *cobra.Command, flags *rootFlags) error {
	if flags.asJSON || flags.agent {
		fmt.Fprintln(cmd.OutOrStdout(), "[]")
	}
	return nil
}

// loadHolidaySet reads mirrored holiday dates into a set keyed by YYYY-MM-DD.
// Holidays are optional; a query error yields an empty set, not a failure.
func loadHolidaySet(ctx context.Context, db *store.Store) map[string]bool {
	set := map[string]bool{}
	rows, err := db.DB().QueryContext(ctx, `SELECT date FROM holidays WHERE date IS NOT NULL`)
	if err != nil {
		return set
	}
	defer rows.Close()
	for rows.Next() {
		var d sql.NullString
		if err := rows.Scan(&d); err != nil {
			continue
		}
		if d.Valid && len(d.String) >= 10 {
			set[d.String[:10]] = true
		}
	}
	return set
}

// adjustSettlement rolls a date forward past weekends and mirrored holidays to
// the next business day. Bounded to avoid an infinite loop on bad data.
func adjustSettlement(t time.Time, holidays map[string]bool) time.Time {
	for i := 0; i < 14; i++ {
		wd := t.Weekday()
		if wd == time.Saturday || wd == time.Sunday || holidays[t.Format("2006-01-02")] {
			t = t.AddDate(0, 0, 1)
			continue
		}
		break
	}
	return t
}

// parseStoredDate parses the DATETIME text the mirror stores, tolerating
// date-only, space-separated, and RFC3339 forms.
func parseStoredDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, l := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	if len(s) >= 10 {
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// parseShareLimit converts a threshold flag ("10%", "10", "0.1") to a fraction
// of 1.0. Empty input returns (0, false) meaning "no limit set".
func parseShareLimit(s string) (float64, bool) {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "%"))
	if s == "" {
		return 0, false
	}
	var v float64
	if _, err := fmt.Sscanf(s, "%g", &v); err != nil {
		return 0, false
	}
	if v > 1 { // entered as a percentage like 10 or 10%
		v = v / 100.0
	}
	return v, true
}
