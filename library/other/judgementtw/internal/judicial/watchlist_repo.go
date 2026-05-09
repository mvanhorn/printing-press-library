// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

package judicial

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// WatchKind classifies a watchlist entry. "case" means watch a single matter
// by JID-pattern (court+案號 root). "query" means watch a saved search.
type WatchKind string

const (
	WatchCase  WatchKind = "case"
	WatchQuery WatchKind = "query"
)

// WatchEntry is one named watch.
type WatchEntry struct {
	Name      string          `json:"name"`
	Kind      WatchKind       `json:"kind"`
	Query     json.RawMessage `json:"query"`
	LastSeen  string          `json:"last_seen"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`
}

// SaveWatch upserts a watch entry. createdAt is preserved on update.
func SaveWatch(ctx context.Context, db *sql.DB, name string, kind WatchKind, query any) error {
	q, err := json.Marshal(query)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.ExecContext(ctx, `
		INSERT INTO watchlist (name, kind, query_json, last_seen, created_at, updated_at)
		VALUES (?, ?, ?, '', ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			kind = excluded.kind,
			query_json = excluded.query_json,
			updated_at = excluded.updated_at`,
		name, string(kind), string(q), now, now)
	return err
}

// GetWatch returns a single watch by name.
func GetWatch(ctx context.Context, db *sql.DB, name string) (*WatchEntry, error) {
	row := db.QueryRowContext(ctx,
		`SELECT name, kind, query_json, last_seen, created_at, updated_at
		 FROM watchlist WHERE name = ?`, name)
	var e WatchEntry
	var q string
	if err := row.Scan(&e.Name, &e.Kind, &q, &e.LastSeen, &e.CreatedAt, &e.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	e.Query = json.RawMessage(q)
	return &e, nil
}

// UpdateWatchCursor advances the last_seen pointer for a watch entry.
func UpdateWatchCursor(ctx context.Context, db *sql.DB, name, lastSeen string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.ExecContext(ctx,
		`UPDATE watchlist SET last_seen = ?, updated_at = ? WHERE name = ?`,
		lastSeen, now, name)
	return err
}

// ListWatches returns all watches.
func ListWatches(ctx context.Context, db *sql.DB) ([]WatchEntry, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT name, kind, query_json, last_seen, created_at, updated_at
		 FROM watchlist ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WatchEntry
	for rows.Next() {
		var e WatchEntry
		var q string
		if err := rows.Scan(&e.Name, &e.Kind, &q, &e.LastSeen, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		e.Query = json.RawMessage(q)
		out = append(out, e)
	}
	return out, rows.Err()
}

// DeleteWatch removes a watch by name. Returns nil even when no row matched
// so callers can use `delete-or-noop` semantics.
func DeleteWatch(ctx context.Context, db *sql.DB, name string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM watchlist WHERE name = ?`, name)
	return err
}

// MaxJID returns the lexicographically-largest JID in the local store. Used
// as a coarse "what's new since last run" cursor for case-pattern watches
// (JIDs are roughly chronological by date suffix, so lex order works).
func MaxJID(ctx context.Context, db *sql.DB, courtPrefix string) (string, error) {
	if courtPrefix == "" {
		row := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(id), '') FROM judgments`)
		var s string
		if err := row.Scan(&s); err != nil {
			return "", err
		}
		return s, nil
	}
	row := db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(id), '') FROM judgments WHERE id LIKE ?`,
		fmt.Sprintf("%s%%", courtPrefix))
	var s string
	if err := row.Scan(&s); err != nil {
		return "", err
	}
	return s, nil
}
