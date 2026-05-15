// Copyright 2026 yaooooooooooooooo. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel store — not generated.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// UpsertDemandEntry inserts or updates a demand entry. first_seen_at is
// preserved on update; last_seen_at is bumped to now on every call. The
// FTS5 mirror is kept in sync via a delete+insert pair against the rowid.
func (s *Store) UpsertDemandEntry(ctx context.Context, e DemandEntry) (isNew bool, err error) {
	if e.ID == "" {
		return false, errors.New("store: demand entry id required")
	}
	now := time.Now().UnixMilli()

	assetsJSON := "[]"
	if e.Assets != nil {
		b, mErr := json.Marshal(e.Assets)
		if mErr != nil {
			return false, fmt.Errorf("store: marshal assets: %w", mErr)
		}
		assetsJSON = string(b)
	}

	// Check existence so we can preserve first_seen_at and detect new rows.
	var existingRowID int64
	var existingFirst int64
	row := s.db.QueryRowContext(ctx, `SELECT rowid, first_seen_at FROM demand_entries WHERE id = ?`, e.ID)
	switch err := row.Scan(&existingRowID, &existingFirst); err {
	case nil:
		isNew = false
	case sql.ErrNoRows:
		isNew = true
		existingFirst = now
	default:
		return false, fmt.Errorf("store: probe demand entry: %w", err)
	}

	if isNew {
		res, err := s.db.ExecContext(ctx, `
			INSERT INTO demand_entries
			  (id, service, service_type, category, description, domain, assets_json, type, first_seen_at, last_seen_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.ID, e.Service, e.ServiceType, e.Category, e.Description, e.Domain, assetsJSON, e.Type, existingFirst, now)
		if err != nil {
			return false, fmt.Errorf("store: insert demand entry: %w", err)
		}
		rid, _ := res.LastInsertId()
		// Mirror to FTS — content-table mode lets us drive the FTS by rowid.
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO demand_entries_fts(rowid, service, description, domain) VALUES (?, ?, ?, ?)`,
			rid, e.Service, e.Description, e.Domain); err != nil {
			return false, fmt.Errorf("store: fts insert: %w", err)
		}
		return true, nil
	}

	// Update path
	if _, err := s.db.ExecContext(ctx, `
		UPDATE demand_entries
		   SET service=?, service_type=?, category=?, description=?, domain=?,
		       assets_json=?, type=?, last_seen_at=?
		 WHERE id=?`,
		e.Service, e.ServiceType, e.Category, e.Description, e.Domain,
		assetsJSON, e.Type, now, e.ID); err != nil {
		return false, fmt.Errorf("store: update demand entry: %w", err)
	}
	// Refresh the FTS row via delete+insert (FTS5 update-on-rowid is
	// available only with INSERT...ON CONFLICT, which content-table FTS
	// doesn't support; delete+insert is the documented pattern).
	if _, err := s.db.ExecContext(ctx, `DELETE FROM demand_entries_fts WHERE rowid=?`, existingRowID); err != nil {
		return false, fmt.Errorf("store: fts delete: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO demand_entries_fts(rowid, service, description, domain) VALUES (?, ?, ?, ?)`,
		existingRowID, e.Service, e.Description, e.Domain); err != nil {
		return false, fmt.Errorf("store: fts reinsert: %w", err)
	}
	return false, nil
}

// ListDemandEntries returns every entry, ordered by last_seen_at desc.
func (s *Store) ListDemandEntries(ctx context.Context) ([]DemandEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, service, service_type, category, description, domain,
		       assets_json, type, first_seen_at, last_seen_at
		  FROM demand_entries
		 ORDER BY last_seen_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("store: list demand: %w", err)
	}
	defer rows.Close()
	return scanDemandRows(rows)
}

// GetDemandEntry returns a single entry by id or sql.ErrNoRows.
func (s *Store) GetDemandEntry(ctx context.Context, id string) (*DemandEntry, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, service, service_type, category, description, domain,
		       assets_json, type, first_seen_at, last_seen_at
		  FROM demand_entries WHERE id=?`, id)
	var (
		e      DemandEntry
		assets string
	)
	if err := row.Scan(&e.ID, &e.Service, &e.ServiceType, &e.Category, &e.Description,
		&e.Domain, &assets, &e.Type, &e.FirstSeenAt, &e.LastSeenAt); err != nil {
		return nil, err
	}
	if assets != "" {
		_ = json.Unmarshal([]byte(assets), &e.Assets)
	}
	return &e, nil
}

// SearchDemandFTS runs an FTS5 MATCH query against service/description/domain.
func (s *Store) SearchDemandFTS(ctx context.Context, query string) ([]DemandEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT d.id, d.service, d.service_type, d.category, d.description, d.domain,
		       d.assets_json, d.type, d.first_seen_at, d.last_seen_at
		  FROM demand_entries_fts f
		  JOIN demand_entries d ON d.rowid = f.rowid
		 WHERE demand_entries_fts MATCH ?
		 ORDER BY d.last_seen_at DESC`, query)
	if err != nil {
		return nil, fmt.Errorf("store: fts search: %w", err)
	}
	defer rows.Close()
	return scanDemandRows(rows)
}

// ListStaleDemand returns entries whose last_seen_at < sinceMs.
func (s *Store) ListStaleDemand(ctx context.Context, sinceMs int64) ([]DemandEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, service, service_type, category, description, domain,
		       assets_json, type, first_seen_at, last_seen_at
		  FROM demand_entries WHERE last_seen_at < ? ORDER BY last_seen_at ASC`, sinceMs)
	if err != nil {
		return nil, fmt.Errorf("store: stale demand: %w", err)
	}
	defer rows.Close()
	return scanDemandRows(rows)
}

func scanDemandRows(rows *sql.Rows) ([]DemandEntry, error) {
	var out []DemandEntry
	for rows.Next() {
		var (
			e      DemandEntry
			assets string
		)
		if err := rows.Scan(&e.ID, &e.Service, &e.ServiceType, &e.Category, &e.Description,
			&e.Domain, &assets, &e.Type, &e.FirstSeenAt, &e.LastSeenAt); err != nil {
			return nil, err
		}
		if assets != "" {
			_ = json.Unmarshal([]byte(assets), &e.Assets)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
