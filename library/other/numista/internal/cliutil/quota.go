// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cliutil

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"time"
)

// PATCH: client-side monthly quota tracking for Numista's free-plan ceiling.
// QuotaLimit is the hard monthly cap on Numista API calls for the free plan.
const QuotaLimit = 2000

// QuotaWarnAt is the soft-warn threshold (used >= this triggers a warning line).
const QuotaWarnAt = 1600

// QuotaUrgentAt is the urgent-warn threshold.
const QuotaUrgentAt = 1900

// QuotaSnapshot is the data point printed to stderr or rendered to JSON
// for the --quota / --quota-only root flags.
type QuotaSnapshot struct {
	Used      int       `json:"used"`
	Limit     int       `json:"limit"`
	Remaining int       `json:"remaining"`
	Reset     time.Time `json:"reset"`
}

// ResetForNextMonth returns the first day of the next month at 00:00:00 UTC.
func ResetForNextMonth(now time.Time) time.Time {
	utc := now.UTC()
	return time.Date(utc.Year(), utc.Month()+1, 1, 0, 0, 0, 0, time.UTC)
}

// ReadQuotaFromDB returns this month's used count as a QuotaSnapshot, using
// a raw SQL count on the lookup_log table. Callers pass in *sql.DB so this
// helper does not depend on the store package (which would cause an import cycle).
//
// Timezone contract (load-bearing): `lookup_log.called_at` is stored as a bare
// SQLite DATETIME populated via `DEFAULT CURRENT_TIMESTAMP`, which is UTC.
// `strftime('%Y-%m', 'now')` is also UTC by default in SQLite. The comparison
// below is correct as long as nothing ever writes `called_at` with a local-time
// value. Anyone changing the schema or the insert path MUST preserve the
// implicit-UTC convention or this monthly window will mis-attribute calls
// near month boundaries for users not in UTC.
func ReadQuotaFromDB(ctx context.Context, db *sql.DB) (QuotaSnapshot, error) {
	var used int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM lookup_log
		 WHERE cache_hit = 0
		   AND strftime('%Y-%m', called_at) = strftime('%Y-%m', 'now')`).Scan(&used)
	if err != nil {
		return QuotaSnapshot{}, fmt.Errorf("read quota: %w", err)
	}
	return QuotaSnapshot{
		Used:      used,
		Limit:     QuotaLimit,
		Remaining: QuotaLimit - used,
		Reset:     ResetForNextMonth(time.Now()),
	}, nil
}

// FormatQuotaLine returns a single human-readable line summarizing the
// current quota, suitable for fprintf to stderr.
func FormatQuotaLine(q QuotaSnapshot) string {
	warn := ""
	switch {
	case q.Used >= q.Limit:
		warn = " (BUDGET EXCEEDED)"
	case q.Used >= QuotaUrgentAt:
		warn = " (URGENT)"
	case q.Used >= QuotaWarnAt:
		warn = " (warning)"
	}
	return fmt.Sprintf("[quota] %d/%d used, %d left, resets %s%s",
		q.Used, q.Limit, q.Remaining, q.Reset.Format("2006-01-02 UTC"), warn)
}

// EmitQuotaLine writes the quota line to w. No-op when q is the zero value.
func EmitQuotaLine(w io.Writer, q QuotaSnapshot) {
	if q.Limit == 0 {
		return
	}
	fmt.Fprintln(w, FormatQuotaLine(q))
}
