// Copyright 2026 kothari-nikunj and contributors. Licensed under Apache-2.0. See LICENSE.

// This file is HAND-WRITTEN and intentionally lives outside the generator's
// output set so it survives across regenerations. It declares the
// hotel-goat-specific tables that have no API resource backing them:
//   - price_snapshots: append-only log of every (property, dates) -> price
//     observation, used by the `drift` and `watch` commands.
//   - brand_aliases:   loyalty-program -> sub-brand expansion, seeded from
//     the absorb manifest, used by the `brand-loyal` command.
//   - wishlist:        local-only saved properties.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// EnsureHotelGoatTables creates the three local-only tables on first call.
// Idempotent: safe to call before every CLI command. The store package's
// migrate() handles spec-derived tables; this initializer covers
// hand-built ones the generator doesn't know about.
func (s *Store) EnsureHotelGoatTables(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS price_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			property_token TEXT NOT NULL,
			property_name TEXT,
			checkin TEXT NOT NULL,
			checkout TEXT NOT NULL,
			price_per_night REAL,
			currency TEXT,
			snapshotted_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_price_snapshots_lookup
			ON price_snapshots(property_token, checkin, checkout, snapshotted_at)`,
		`CREATE TABLE IF NOT EXISTS brand_aliases (
			program TEXT NOT NULL,
			sub_brand TEXT NOT NULL,
			PRIMARY KEY (program, sub_brand)
		)`,
		`CREATE TABLE IF NOT EXISTS wishlist (
			property_token TEXT PRIMARY KEY,
			name TEXT,
			data TEXT,
			added_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensure hotel-goat table: %w (sql=%s)", err, firstLine(stmt))
		}
	}
	return s.seedBrandAliases(ctx)
}

// seedBrandAliases populates brand_aliases from the hard-coded loyalty
// program map. Re-inserts use OR IGNORE so users who hand-add aliases
// keep theirs; we never delete user rows.
func (s *Store) seedBrandAliases(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for program, subs := range loyaltyProgramSeed {
		for _, sub := range subs {
			if _, err := tx.ExecContext(ctx,
				`INSERT OR IGNORE INTO brand_aliases (program, sub_brand) VALUES (?, ?)`,
				program, sub,
			); err != nil {
				return fmt.Errorf("seed brand_aliases: %w", err)
			}
		}
	}
	return tx.Commit()
}

// loyaltyProgramSeed is the canonical seed list straight from the
// absorb manifest. Add new sub-brands here when programs spawn new
// flag brands.
var loyaltyProgramSeed = map[string][]string{
	"hyatt": {
		"Park Hyatt", "Andaz", "Thompson", "Hyatt Place", "Hyatt House",
		"Hyatt Centric", "Grand Hyatt", "Hyatt Regency", "Alila", "Miraval",
		"Destination by Hyatt", "Hyatt",
	},
	"marriott": {
		"Marriott", "Courtyard", "Residence Inn", "Westin", "Sheraton",
		"Renaissance", "Le Méridien", "JW Marriott", "Ritz-Carlton", "St. Regis",
		"W", "Tribute Portfolio", "Autograph Collection", "Aloft", "Element",
		"Moxy", "AC Hotels", "Delta Hotels", "Four Points", "Fairfield",
		"SpringHill Suites", "TownePlace Suites", "Bonvoy",
	},
	"hilton": {
		"Hilton", "DoubleTree", "Hampton", "Embassy Suites", "Hilton Garden Inn",
		"Homewood Suites", "Home2", "Tru", "Curio", "Tapestry", "Canopy",
		"Conrad", "Waldorf Astoria", "LXR", "Signia", "Motto",
	},
	"ihg": {
		"Holiday Inn", "Holiday Inn Express", "Crowne Plaza", "InterContinental",
		"Kimpton", "Hotel Indigo", "Voco", "EVEN", "Avid", "Staybridge",
		"Candlewood", "Six Senses", "Regent", "Vignette",
	},
	"accor": {
		"Sofitel", "Pullman", "Novotel", "Mercure", "ibis", "Raffles",
		"Fairmont", "MGallery", "SO/", "25hours", "Mama Shelter",
	},
}

// RecordPriceSnapshot appends one observation. property_token may be empty
// when only a name is known (e.g. resolved-by-name later).
func (s *Store) RecordPriceSnapshot(ctx context.Context, token, name, checkin, checkout, currency string, price float64) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO price_snapshots (property_token, property_name, checkin, checkout, price_per_night, currency)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		token, name, checkin, checkout, price, currency,
	)
	return err
}

// PriceSnapshot is a single price observation read back from the table.
type PriceSnapshot struct {
	ID            int64   `json:"id"`
	PropertyToken string  `json:"property_token,omitempty"`
	PropertyName  string  `json:"property_name,omitempty"`
	Checkin       string  `json:"checkin"`
	Checkout      string  `json:"checkout"`
	PricePerNight float64 `json:"price_per_night"`
	Currency      string  `json:"currency,omitempty"`
	SnapshottedAt string  `json:"snapshotted_at"`
}

// ListPriceSnapshots returns all snapshots for a property (matched by
// token OR name fragment) since `since`. Newest first.
func (s *Store) ListPriceSnapshots(ctx context.Context, tokenOrName string, since time.Time, limit int) ([]PriceSnapshot, error) {
	if limit <= 0 {
		limit = 200
	}
	q := `SELECT id, property_token, property_name, checkin, checkout, price_per_night, currency, snapshotted_at
		FROM price_snapshots
		WHERE (property_token = ? OR property_name LIKE ?)
			AND snapshotted_at >= ?
		ORDER BY snapshotted_at DESC
		LIMIT ?`
	// Match the format SQLite's CURRENT_TIMESTAMP writes — "YYYY-MM-DD
	// HH:MM:SS" with a space separator. Comparing against an RFC3339
	// string ("YYYY-MM-DDTHH:MM:SSZ" with a 'T' separator) is
	// lexicographic and silently drops every row from the boundary day
	// because 'T' sorts after every literal space-prefixed value.
	rows, err := s.db.QueryContext(ctx, q, tokenOrName, "%"+tokenOrName+"%", since.UTC().Format("2006-01-02 15:04:05"), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PriceSnapshot
	for rows.Next() {
		var (
			rec  PriceSnapshot
			tok  sql.NullString
			name sql.NullString
			cur  sql.NullString
		)
		if err := rows.Scan(&rec.ID, &tok, &name, &rec.Checkin, &rec.Checkout, &rec.PricePerNight, &cur, &rec.SnapshottedAt); err != nil {
			return nil, err
		}
		rec.PropertyToken = tok.String
		rec.PropertyName = name.String
		rec.Currency = cur.String
		out = append(out, rec)
	}
	return out, rows.Err()
}

// LatestSnapshotFor returns the most recent snapshot for the given
// (token, checkin, checkout) tuple, or nil when none.
func (s *Store) LatestSnapshotFor(ctx context.Context, tokenOrName, checkin, checkout string) (*PriceSnapshot, error) {
	q := `SELECT id, property_token, property_name, checkin, checkout, price_per_night, currency, snapshotted_at
		FROM price_snapshots
		WHERE (property_token = ? OR property_name LIKE ?)
			AND checkin = ? AND checkout = ?
		ORDER BY snapshotted_at DESC
		LIMIT 1`
	row := s.db.QueryRowContext(ctx, q, tokenOrName, "%"+tokenOrName+"%", checkin, checkout)
	var (
		rec  PriceSnapshot
		tok  sql.NullString
		name sql.NullString
		cur  sql.NullString
	)
	err := row.Scan(&rec.ID, &tok, &name, &rec.Checkin, &rec.Checkout, &rec.PricePerNight, &cur, &rec.SnapshottedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rec.PropertyToken = tok.String
	rec.PropertyName = name.String
	rec.Currency = cur.String
	return &rec, nil
}

// BrandsForProgram returns the seeded sub-brand list for a loyalty program.
func (s *Store) BrandsForProgram(ctx context.Context, program string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT sub_brand FROM brand_aliases WHERE program = ? ORDER BY sub_brand`,
		strings.ToLower(program),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// WishlistAdd inserts (or replaces) a wishlist entry. `data` is a
// caller-marshalled JSON of the full hotel record for cheap re-display.
func (s *Store) WishlistAdd(ctx context.Context, token, name string, data json.RawMessage) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO wishlist (property_token, name, data) VALUES (?, ?, ?)
		 ON CONFLICT(property_token) DO UPDATE SET name=excluded.name, data=excluded.data`,
		token, name, string(data),
	)
	return err
}

// WishlistRemove deletes a wishlist entry. Returns rows affected.
func (s *Store) WishlistRemove(ctx context.Context, token string) (int64, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.ExecContext(ctx, `DELETE FROM wishlist WHERE property_token = ?`, token)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// WishlistEntry is one saved property.
type WishlistEntry struct {
	PropertyToken string          `json:"property_token"`
	Name          string          `json:"name,omitempty"`
	Data          json.RawMessage `json:"data,omitempty"`
	AddedAt       string          `json:"added_at"`
}

func (s *Store) WishlistList(ctx context.Context) ([]WishlistEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT property_token, name, data, added_at FROM wishlist ORDER BY added_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WishlistEntry
	for rows.Next() {
		var (
			e    WishlistEntry
			name sql.NullString
			data sql.NullString
		)
		if err := rows.Scan(&e.PropertyToken, &name, &data, &e.AddedAt); err != nil {
			return nil, err
		}
		e.Name = name.String
		if data.Valid && data.String != "" {
			e.Data = json.RawMessage(data.String)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
