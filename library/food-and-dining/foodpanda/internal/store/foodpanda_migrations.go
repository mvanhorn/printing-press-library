// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored foodpanda-specific schema. Kept in its own file so
// `generate --force` preserves it instead of re-emitting the templated store.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// EnsureMenuSnapshots creates the menu-snapshot table on first use. Menus are a
// detail-path resource that `sync` does not mirror, so the commands that need
// menu history (dish, menu-diff) own this table themselves.
func (s *Store) EnsureMenuSnapshots(ctx context.Context) error {
	const ddl = `
	CREATE TABLE IF NOT EXISTS fp_menu_snapshots (
		vendor_code TEXT NOT NULL,
		captured_at TEXT NOT NULL,
		vendor_name TEXT,
		country     TEXT,
		payload     TEXT NOT NULL,
		PRIMARY KEY (vendor_code, captured_at)
	)`
	if _, err := s.db.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("creating fp_menu_snapshots: %w", err)
	}
	if _, err := s.db.ExecContext(ctx,
		`CREATE INDEX IF NOT EXISTS idx_fp_menu_snapshots_code ON fp_menu_snapshots(vendor_code, captured_at DESC)`); err != nil {
		return fmt.Errorf("indexing fp_menu_snapshots: %w", err)
	}
	return nil
}

// SaveMenuSnapshot records one menu capture. Repeated captures within the same
// second collapse onto the same primary key, which is intentional.
func (s *Store) SaveMenuSnapshot(ctx context.Context, vendorCode, vendorName, country string, payload json.RawMessage) error {
	if err := s.EnsureMenuSnapshots(ctx); err != nil {
		return err
	}
	// Serialize with every other writer: SQLite WAL permits one writer.
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO fp_menu_snapshots (vendor_code, captured_at, vendor_name, country, payload)
		 VALUES (?, ?, ?, ?, ?)`,
		vendorCode, time.Now().UTC().Format(time.RFC3339), vendorName, country, string(payload))
	if err != nil {
		return fmt.Errorf("saving menu snapshot for %s: %w", vendorCode, err)
	}
	return nil
}

// MenuSnapshot is one stored capture.
type MenuSnapshot struct {
	VendorCode string
	VendorName string
	Country    string
	CapturedAt time.Time
	Payload    json.RawMessage
}

// ListMenuSnapshots returns snapshots for a vendor, newest first.
func (s *Store) ListMenuSnapshots(ctx context.Context, vendorCode string, limit int) ([]MenuSnapshot, error) {
	if err := s.EnsureMenuSnapshots(ctx); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT vendor_code, COALESCE(vendor_name,''), COALESCE(country,''), captured_at, payload
		 FROM fp_menu_snapshots WHERE vendor_code = ? ORDER BY captured_at DESC LIMIT ?`,
		vendorCode, limit)
	if err != nil {
		return nil, fmt.Errorf("reading menu snapshots: %w", err)
	}
	// Drain fully before any follow-up query: SQLite keeps a single connection
	// and a second query while rows are open can deadlock or error.
	type rawRow struct {
		code, name, country, captured, payload string
	}
	raws := make([]rawRow, 0, limit)
	for rows.Next() {
		var r rawRow
		var name, country sql.NullString
		if err := rows.Scan(&r.code, &name, &country, &r.captured, &r.payload); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning menu snapshot: %w", err)
		}
		r.name, r.country = name.String, country.String
		raws = append(raws, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating menu snapshots: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing menu snapshots: %w", err)
	}

	out := make([]MenuSnapshot, 0, len(raws))
	for _, r := range raws {
		ts, _ := time.Parse(time.RFC3339, r.captured)
		out = append(out, MenuSnapshot{
			VendorCode: r.code, VendorName: r.name, Country: r.country,
			CapturedAt: ts, Payload: json.RawMessage(r.payload),
		})
	}
	return out, nil
}
