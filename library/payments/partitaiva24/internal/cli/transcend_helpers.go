// Copyright 2026 giuseppe-bisemi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/partitaiva24/internal/store"
)

// openStore opens the SQLite store at the canonical path.
func openStore(ctx context.Context) (*store.Store, error) {
	dbPath := defaultDBPath("partitaiva24-pp-cli")
	return store.OpenWithContext(ctx, dbPath)
}

// quarterRange returns the YYYY-MM-DD start/end dates for a quarter spec like "2026-Q2".
func quarterRange(spec string) (start, end string, err error) {
	parts := strings.Split(strings.TrimSpace(spec), "-Q")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid quarter %q (expected YYYY-Q1..Q4)", spec)
	}
	y, errY := strconv.Atoi(parts[0])
	q, errQ := strconv.Atoi(parts[1])
	if errY != nil || errQ != nil || q < 1 || q > 4 {
		return "", "", fmt.Errorf("invalid quarter %q (expected YYYY-Q1..Q4)", spec)
	}
	startMonth := (q-1)*3 + 1
	endMonth := startMonth + 2
	endDay := time.Date(y, time.Month(endMonth+1), 0, 0, 0, 0, 0, time.UTC).Day()
	return fmt.Sprintf("%04d-%02d-01", y, startMonth), fmt.Sprintf("%04d-%02d-%02d", y, endMonth, endDay), nil
}

// yearRange returns the YYYY-MM-DD start/end dates for a year.
func yearRange(year int) (string, string) {
	return fmt.Sprintf("%04d-01-01", year), fmt.Sprintf("%04d-12-31", year)
}

// monthRange returns dates for a YYYY-MM spec.
func monthRange(spec string) (string, string, error) {
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid month %q (expected YYYY-MM)", spec)
	}
	y, errY := strconv.Atoi(parts[0])
	m, errM := strconv.Atoi(parts[1])
	if errY != nil || errM != nil || m < 1 || m > 12 {
		return "", "", fmt.Errorf("invalid month %q", spec)
	}
	endDay := time.Date(y, time.Month(m+1), 0, 0, 0, 0, 0, time.UTC).Day()
	return fmt.Sprintf("%04d-%02d-01", y, m), fmt.Sprintf("%04d-%02d-%02d", y, m, endDay), nil
}

// currentYear returns the current calendar year.
func currentYear() int { return time.Now().UTC().Year() }

// daysElapsed returns days between Jan 1 of year and today (capped at 365).
func daysElapsed(year int) int {
	now := time.Now().UTC()
	if year < now.Year() {
		return 365
	}
	if year > now.Year() {
		return 0
	}
	jan1 := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	d := int(now.Sub(jan1).Hours() / 24)
	if d < 1 {
		d = 1
	}
	if d > 365 {
		d = 365
	}
	return d
}

// scanFloatSum runs a single-column REAL aggregate query and returns the sum (0 on null).
func scanFloatSum(ctx context.Context, db *sql.DB, query string, args ...any) (float64, error) {
	row := db.QueryRowContext(ctx, query, args...)
	var v sql.NullFloat64
	if err := row.Scan(&v); err != nil {
		return 0, err
	}
	if !v.Valid {
		return 0, nil
	}
	return v.Float64, nil
}

// scanIntSum runs a single-column INTEGER aggregate query.
func scanIntSum(ctx context.Context, db *sql.DB, query string, args ...any) (int64, error) {
	row := db.QueryRowContext(ctx, query, args...)
	var v sql.NullInt64
	if err := row.Scan(&v); err != nil {
		return 0, err
	}
	if !v.Valid {
		return 0, nil
	}
	return v.Int64, nil
}

// parseDurationDays accepts "7d", "30d", "24h", "1h", returns a Duration.
// Falls back to time.ParseDuration for the standard suffixes.
func parseDurationDays(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// extractInvoiceNumberInt pulls the trailing or leading integer out of a
// human-shaped invoice number like "2026-001", "001/2026", or "1".
// Returns (n, true) on success, (0, false) on unparseable.
func extractInvoiceNumberInt(num string) (int, bool) {
	num = strings.TrimSpace(num)
	if num == "" {
		return 0, false
	}
	// Strip year prefix/suffix joined by - or /
	parts := strings.FieldsFunc(num, func(r rune) bool {
		return r == '-' || r == '/' || r == ' '
	})
	for i := len(parts) - 1; i >= 0; i-- {
		// Skip 4-digit year tokens
		if len(parts[i]) == 4 {
			if y, err := strconv.Atoi(parts[i]); err == nil && y >= 2000 && y <= 2100 {
				continue
			}
		}
		if n, err := strconv.Atoi(parts[i]); err == nil {
			return n, true
		}
	}
	if n, err := strconv.Atoi(num); err == nil {
		return n, true
	}
	return 0, false
}
