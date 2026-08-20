// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: shared helpers for the parking quote/sweep/watch/calendar
// commands. Not generated.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/sea-airport-parking/internal/config"
	"github.com/mvanhorn/printing-press-library/library/travel/sea-airport-parking/internal/parking"
	"github.com/mvanhorn/printing-press-library/library/travel/sea-airport-parking/internal/store"
)

// dbName is the local SQLite database name for this CLI.
const dbName = "sea-airport-parking-pp-cli"

// defaultEntryHour / defaultExitHour are used when a date is given without a
// time, matching the site's 11:00 default selection.
const defaultEntryHour = 11

// newParkingClient builds a parking.Client from resolved config so the
// SEA_AIRPORT_PARKING_BASE_URL override (used by verify/dogfood) is honored.
func newParkingClient(flags *rootFlags) (*parking.Client, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, configErr(err)
	}
	return parking.New(cfg.BaseURL, flags.timeout)
}

// parseWhen parses a user-supplied entry/exit datetime. Accepted forms:
//
//	2006-01-02T15:04        2006-01-02 15:04       2006-01-02T15:04:05Z07:00
//	2006-01-02              01/02/2006             01/02/2006 15:04
//
// A date with no time defaults to 11:00 local (the site's default).
func parseWhen(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date/time")
	}
	layouts := []string{
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"01/02/2006 15:04",
	}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, s, time.Local); err == nil {
			return t, nil
		}
	}
	// Date-only forms default the time to 11:00 local.
	for _, l := range []string{"2006-01-02", "01/02/2006"} {
		if t, err := time.ParseInLocation(l, s, time.Local); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), defaultEntryHour, 0, 0, 0, time.Local), nil
		}
	}
	return time.Time{}, fmt.Errorf("could not parse date/time %q (use YYYY-MM-DDTHH:MM or YYYY-MM-DD)", s)
}

// quoteToRecord converts a live quote into a persistable snapshot row.
func quoteToRecord(q *parking.Quote) store.QuoteRecord {
	nightly := ""
	if len(q.NightlyPrices) > 0 {
		if b, err := json.Marshal(q.NightlyPrices); err == nil {
			nightly = string(b)
		}
	}
	return store.QuoteRecord{
		Entry:         q.EntryStr,
		Exit:          q.ExitStr,
		Nights:        q.Nights,
		ProductCode:   q.ProductCode,
		ProductName:   q.ProductName,
		TotalPrice:    q.TotalPrice,
		OriginalPrice: q.OriginalPrice,
		Available:     q.Available,
		SoldOut:       q.SoldOut,
		Promo:         q.Promo,
		Reason:        q.Reason,
		NightlyJSON:   nightly,
		CapturedAt:    q.CapturedAt.Format(time.RFC3339),
	}
}

// persistQuote appends a snapshot to the local store. Persistence failures are
// non-fatal for the live commands (the quote itself still returns), so callers
// log the error to stderr rather than failing the command.
func persistQuote(ctx context.Context, dbPath string, q *parking.Quote) error {
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return err
	}
	defer s.Close()
	_, err = s.InsertQuote(ctx, quoteToRecord(q))
	return err
}

// validateRange applies the client-side bounds the site enforces server-side,
// giving a fast, actionable error before any network call.
func validateRange(entry, exit time.Time) error {
	if !exit.After(entry) {
		return fmt.Errorf("exit must be after entry")
	}
	return nil
}
