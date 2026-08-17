// Hand-authored 1688-specific store extension. Not generator-emitted.
//
// Offers and suppliers live in the generic `resources` table (resource_type
// 'offer' / 'supplier') so the existing FTS index powers `find`. This file
// adds the drift snapshot table and the json_extract query helpers the
// transcendence commands read.
package store

import (
	"context"
	"encoding/json"
)

// OfferSnapshot is one point-in-time observation of an offer's volatile
// fields, written on every search/sync so `drift` can diff across runs.
type OfferSnapshot struct {
	OfferID       string  `json:"offer_id"`
	SyncedAt      string  `json:"synced_at"`
	Keyword       string  `json:"keyword"`
	PriceCNY      float64 `json:"price_cny"`
	RepurchasePct float64 `json:"repurchase_pct"`
	BookedCount   int     `json:"booked_count"`
}

// Ensure1688Schema lazily creates the snapshot table. Cheap to call on every
// command entry (CREATE TABLE IF NOT EXISTS is a no-op once present).
func (s *Store) Ensure1688Schema(ctx context.Context) error {
	// Serialize through the store's write mutex like every other writer in
	// this package (CREATE TABLE is a write).
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.DB().ExecContext(ctx, `CREATE TABLE IF NOT EXISTS offer_snapshots (
		offer_id TEXT NOT NULL,
		synced_at TEXT NOT NULL,
		keyword TEXT,
		price_cny REAL,
		repurchase_pct REAL,
		booked_count INTEGER,
		PRIMARY KEY (offer_id, synced_at)
	)`)
	return err
}

// InsertOfferSnapshot records one observation. INSERT OR IGNORE keeps the
// (offer_id, synced_at) pair unique without clobbering an existing row.
func (s *Store) InsertOfferSnapshot(ctx context.Context, snap OfferSnapshot) error {
	if err := s.Ensure1688Schema(ctx); err != nil {
		return err
	}
	// Hold the store write mutex for the insert, matching the package's
	// write-serialization contract (Ensure1688Schema locked/unlocked above).
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.DB().ExecContext(ctx,
		`INSERT OR IGNORE INTO offer_snapshots (offer_id, synced_at, keyword, price_cny, repurchase_pct, booked_count)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		snap.OfferID, snap.SyncedAt, snap.Keyword, snap.PriceCNY, snap.RepurchasePct, snap.BookedCount)
	return err
}

// OfferSnapshots returns every snapshot whose offer_id OR keyword matches the
// target, oldest first, so drift can compare first vs latest.
func (s *Store) OfferSnapshots(ctx context.Context, target string) ([]OfferSnapshot, error) {
	if err := s.Ensure1688Schema(ctx); err != nil {
		return nil, err
	}
	rows, err := s.DB().QueryContext(ctx,
		`SELECT offer_id, synced_at, COALESCE(keyword,''), COALESCE(price_cny,0), COALESCE(repurchase_pct,0), COALESCE(booked_count,0)
		 FROM offer_snapshots WHERE offer_id = ? OR keyword = ? ORDER BY synced_at ASC`, target, target)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OfferSnapshot
	for rows.Next() {
		var sn OfferSnapshot
		if err := rows.Scan(&sn.OfferID, &sn.SyncedAt, &sn.Keyword, &sn.PriceCNY, &sn.RepurchasePct, &sn.BookedCount); err != nil {
			continue
		}
		out = append(out, sn)
	}
	return out, rows.Err()
}

// OffersByKeyword returns stored offer JSON for a synced keyword. An empty
// keyword returns all stored offers (newest first). limit <= 0 means no cap.
func (s *Store) OffersByKeyword(ctx context.Context, keyword string, limit int) ([]json.RawMessage, error) {
	q := `SELECT data FROM resources WHERE resource_type = 'offer'`
	args := []any{}
	if keyword != "" {
		q += ` AND json_extract(data, '$.keyword') = ?`
		args = append(args, keyword)
	}
	q += ` ORDER BY updated_at DESC`
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	return s.queryOfferJSON(ctx, q, args...)
}

// FindOffers does an offline substring match over stored offer titles and
// supplier names. LIKE is used instead of FTS because SQLite's default
// tokenizer does not segment CJK text, so an FTS MATCH on a Chinese substring
// returns nothing.
func (s *Store) FindOffers(ctx context.Context, term string, limit int) ([]json.RawMessage, error) {
	if limit <= 0 {
		limit = 25
	}
	like := "%" + term + "%"
	return s.queryOfferJSON(ctx,
		`SELECT data FROM resources WHERE resource_type = 'offer'
		   AND (json_extract(data, '$.title') LIKE ? OR json_extract(data, '$.supplier_name') LIKE ?)
		 ORDER BY updated_at DESC LIMIT ?`,
		like, like, limit)
}

// OffersBySupplier returns stored offer JSON for one supplier member ID.
func (s *Store) OffersBySupplier(ctx context.Context, memberID string) ([]json.RawMessage, error) {
	return s.queryOfferJSON(ctx,
		`SELECT data FROM resources WHERE resource_type = 'offer' AND json_extract(data, '$.supplier_member_id') = ? ORDER BY updated_at DESC`,
		memberID)
}

func (s *Store) queryOfferJSON(ctx context.Context, q string, args ...any) ([]json.RawMessage, error) {
	rows, err := s.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []json.RawMessage
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			continue
		}
		out = append(out, json.RawMessage(data))
	}
	return out, rows.Err()
}
