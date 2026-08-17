// Hand-authored store extension for the `watch` novel feature: saved searches
// and per-run listing snapshots used to diff new listings and price drops
// across runs. Separate file so `generate --force` regen-merge preserves it.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// UpsertListing stores one Zameen listing keyed by its external id. A
// domain-named wrapper over the generic Upsert so callers express intent and
// the listings resource key stays in one place.
func (s *Store) UpsertListing(externalID string, data json.RawMessage) error {
	if externalID == "" {
		return fmt.Errorf("listing external id is required")
	}
	return s.Upsert("listings", externalID, data)
}

// SearchListings runs a full-text search over stored listings via the
// resources FTS index, scoped to the listings resource type.
func (s *Store) SearchListings(query string, limit int) ([]json.RawMessage, error) {
	return s.Search(query, limit, "listings")
}

// WatchSearch is a saved, named search definition.
type WatchSearch struct {
	Name      string `json:"name"`
	Params    string `json:"params"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// EnsureWatchTables lazily creates the watch tables. Safe to call repeatedly.
func (s *Store) EnsureWatchTables(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS watch_searches (
			name TEXT PRIMARY KEY,
			params TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS watch_snapshots (
			name TEXT NOT NULL,
			external_id TEXT NOT NULL,
			price INTEGER NOT NULL,
			seen_at INTEGER NOT NULL,
			PRIMARY KEY (name, external_id)
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensuring watch tables: %w", err)
		}
	}
	return nil
}

// SaveWatchSearch inserts or updates a saved search definition.
func (s *Store) SaveWatchSearch(ctx context.Context, name, params string, ts int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO watch_searches (name, params, created_at, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET params = excluded.params, updated_at = excluded.updated_at`,
		name, params, ts, ts)
	if err != nil {
		return fmt.Errorf("saving watch search %q: %w", name, err)
	}
	return nil
}

// GetWatchSearch returns the saved params for a named search.
func (s *Store) GetWatchSearch(ctx context.Context, name string) (string, bool, error) {
	row := s.db.QueryRowContext(ctx, `SELECT params FROM watch_searches WHERE name = ?`, name)
	var params string
	err := row.Scan(&params)
	switch {
	case err == nil:
		return params, true, nil
	case errors.Is(err, sql.ErrNoRows):
		return "", false, nil
	default:
		return "", false, err
	}
}

// ListWatchSearches returns all saved searches, newest first.
func (s *Store) ListWatchSearches(ctx context.Context) ([]WatchSearch, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, params, created_at, updated_at FROM watch_searches ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WatchSearch
	for rows.Next() {
		var w WatchSearch
		if err := rows.Scan(&w.Name, &w.Params, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// LoadWatchSnapshot returns the last-seen price keyed by external_id for a name.
func (s *Store) LoadWatchSnapshot(ctx context.Context, name string) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT external_id, price FROM watch_snapshots WHERE name = ?`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var id string
		var price int
		if err := rows.Scan(&id, &price); err != nil {
			return nil, err
		}
		out[id] = price
	}
	return out, rows.Err()
}

// ReplaceWatchSnapshot atomically replaces the stored snapshot for a name with
// the current price-by-id map, inside a single write transaction.
func (s *Store) ReplaceWatchSnapshot(ctx context.Context, name string, priceByID map[string]int, ts int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM watch_snapshots WHERE name = ?`, name); err != nil {
		return fmt.Errorf("clearing snapshot %q: %w", name, err)
	}
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO watch_snapshots (name, external_id, price, seen_at) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for id, price := range priceByID {
		if _, err := stmt.ExecContext(ctx, name, id, price, ts); err != nil {
			return fmt.Errorf("writing snapshot row %q: %w", id, err)
		}
	}
	return tx.Commit()
}
