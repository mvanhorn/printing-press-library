// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: parking quote-snapshot table. Not generated.

package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// QuoteRecord is one persisted parking-quote snapshot. Every quote/sweep/
// calendar/watch poll appends a row, giving the price/availability time series
// that history and drift read back.
type QuoteRecord struct {
	ID            int64   `json:"id"`
	Entry         string  `json:"entry"` // 2006-01-02T15:04 local
	Exit          string  `json:"exit"`
	Nights        int     `json:"nights"`
	ProductCode   string  `json:"product_code,omitempty"`
	ProductName   string  `json:"product_name,omitempty"`
	TotalPrice    float64 `json:"total_price"`
	OriginalPrice float64 `json:"original_price,omitempty"`
	Available     bool    `json:"available"`
	SoldOut       bool    `json:"sold_out"`
	Promo         string  `json:"promo,omitempty"`
	Reason        string  `json:"reason,omitempty"`
	NightlyJSON   string  `json:"nightly_json,omitempty"`
	CapturedAt    string  `json:"captured_at"` // RFC3339 UTC
}

// ensureQuotesTable lazily creates the quotes table. CREATE TABLE IF NOT EXISTS
// is idempotent, so this is safe to call on every access without a migration
// slice entry (keeps the hand-authored schema out of the generated store.go).
func (s *Store) ensureQuotesTable(ctx context.Context) error {
	_, err := s.DB().ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS parking_quotes (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			entry          TEXT NOT NULL,
			exit           TEXT NOT NULL,
			nights         INTEGER NOT NULL,
			product_code   TEXT,
			product_name   TEXT,
			total_price    REAL NOT NULL,
			original_price REAL,
			available      INTEGER NOT NULL,
			sold_out       INTEGER NOT NULL,
			promo          TEXT,
			reason         TEXT,
			nightly_json   TEXT,
			captured_at    TEXT NOT NULL
		)`)
	if err != nil {
		return err
	}
	_, _ = s.DB().ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_parking_quotes_range ON parking_quotes(entry, exit, captured_at)`)
	return nil
}

// InsertQuote appends a snapshot and returns its row id.
func (s *Store) InsertQuote(ctx context.Context, rec QuoteRecord) (int64, error) {
	if err := s.ensureQuotesTable(ctx); err != nil {
		return 0, err
	}
	res, err := s.DB().ExecContext(ctx, `
		INSERT INTO parking_quotes
			(entry, exit, nights, product_code, product_name, total_price, original_price, available, sold_out, promo, reason, nightly_json, captured_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		rec.Entry, rec.Exit, rec.Nights, rec.ProductCode, rec.ProductName,
		rec.TotalPrice, rec.OriginalPrice, boolToInt(rec.Available), boolToInt(rec.SoldOut),
		rec.Promo, rec.Reason, rec.NightlyJSON, rec.CapturedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListQuotes returns snapshots for an exact entry/exit range, newest first.
// If since is non-zero, only snapshots captured at or after it are returned.
// It does NOT create the table (so it is safe on a read-only connection); if the
// table does not exist yet — no quote has ever been persisted — it returns an
// empty result rather than an error.
func (s *Store) ListQuotes(ctx context.Context, entry, exit string, since time.Time) ([]QuoteRecord, error) {
	query := `
		SELECT id, entry, exit, nights, product_code, product_name, total_price,
		       original_price, available, sold_out, promo, reason, nightly_json, captured_at
		FROM parking_quotes
		WHERE entry = ? AND exit = ?`
	args := []any{entry, exit}
	if !since.IsZero() {
		query += ` AND captured_at >= ?`
		args = append(args, since.UTC().Format(time.RFC3339))
	}
	query += ` ORDER BY captured_at DESC, id DESC`
	return s.scanQuotes(ctx, query, args...)
}

// scanQuotes runs query and scans all rows (drain-first: no follow-up queries
// are issued while rows is open). A missing parking_quotes table (nothing has
// been persisted yet) is treated as an empty result, not an error, so read-only
// callers work before the first quote.
func (s *Store) scanQuotes(ctx context.Context, query string, args ...any) ([]QuoteRecord, error) {
	rows, err := s.DB().QueryContext(ctx, query, args...)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return []QuoteRecord{}, nil
		}
		return nil, err
	}
	defer rows.Close()
	out := make([]QuoteRecord, 0)
	for rows.Next() {
		var (
			r                    QuoteRecord
			available, soldOut   int
			code, name           sql.NullString
			orig                 sql.NullFloat64
			promo, reason, night sql.NullString
		)
		if err := rows.Scan(&r.ID, &r.Entry, &r.Exit, &r.Nights, &code, &name,
			&r.TotalPrice, &orig, &available, &soldOut, &promo, &reason, &night, &r.CapturedAt); err != nil {
			return nil, err
		}
		r.ProductCode = code.String
		r.ProductName = name.String
		r.OriginalPrice = orig.Float64
		r.Available = available != 0
		r.SoldOut = soldOut != 0
		r.Promo = promo.String
		r.Reason = reason.String
		r.NightlyJSON = night.String
		out = append(out, r)
	}
	return out, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
