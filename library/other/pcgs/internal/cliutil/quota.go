package cliutil

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"time"
)

// QuotaLimit is the hard daily cap on PCGS API calls per token.
const QuotaLimit = 1000

// QuotaWarnAt is the soft-warn threshold (used >= this triggers a yellow line).
const QuotaWarnAt = 800

// QuotaUrgentAt is the urgent-warn threshold (used >= this triggers a red line).
const QuotaUrgentAt = 950

// QuotaSnapshot is the data point printed to stderr or rendered to JSON
// for the --quota / --quota-only root flags.
type QuotaSnapshot struct {
	Used      int       `json:"used"`
	Limit     int       `json:"limit"`
	Remaining int       `json:"remaining"`
	Reset     time.Time `json:"reset"`
}

// ResetForUTCNow returns the next 00:00 UTC boundary after the given time.
func ResetForUTCNow(now time.Time) time.Time {
	utc := now.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day()+1, 0, 0, 0, 0, time.UTC)
}

// ReadQuotaFromDB returns today's used count as a QuotaSnapshot, using
// a raw SQL count on the lookup_log table. Callers pass in *sql.DB so
// this helper does not depend on the store package (which would cause
// an import cycle).
func ReadQuotaFromDB(ctx context.Context, db *sql.DB) (QuotaSnapshot, error) {
	var used int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM lookup_log
		 WHERE cache_hit = 0
		   AND called_at >= datetime('now','start of day')`).Scan(&used)
	if err != nil {
		return QuotaSnapshot{}, fmt.Errorf("read quota: %w", err)
	}
	return QuotaSnapshot{
		Used:      used,
		Limit:     QuotaLimit,
		Remaining: QuotaLimit - used,
		Reset:     ResetForUTCNow(time.Now()),
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
		q.Used, q.Limit, q.Remaining, q.Reset.Format("2006-01-02 15:04:05 UTC"), warn)
}

// EmitQuotaLine writes the quota line to w. No-op when q is the zero value.
func EmitQuotaLine(w io.Writer, q QuotaSnapshot) {
	if q.Limit == 0 {
		return
	}
	fmt.Fprintln(w, FormatQuotaLine(q))
}
