// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// SnapshotRow is one resource's data as it existed on a given snapshot date.
type SnapshotRow struct {
	ResourceID string
	Data       json.RawMessage
}

// CaptureSnapshot copies the current contents of the resources table for the
// given resource types into resource_snapshots, stamped with today's date
// (UTC). Calling it more than once on the same day refreshes today's rows in
// place, so history only ever accumulates one entry per resource per day.
func (s *Store) CaptureSnapshot(ctx context.Context, resourceTypes ...string) (date string, captured int, err error) {
	date = time.Now().UTC().Format("2006-01-02")
	now := time.Now().UTC().Format(time.RFC3339)

	query := `SELECT resource_type, id, data FROM resources`
	args := []any{}
	if len(resourceTypes) > 0 {
		placeholders := ""
		for i, rt := range resourceTypes {
			if i > 0 {
				placeholders += ","
			}
			placeholders += "?"
			args = append(args, rt)
		}
		query += ` WHERE resource_type IN (` + placeholders + `)`
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return date, 0, err
	}
	type row struct {
		resourceType string
		id           string
		data         string
	}
	var buffered []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.resourceType, &r.id, &r.data); err != nil {
			rows.Close()
			return date, 0, err
		}
		buffered = append(buffered, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return date, 0, err
	}
	rows.Close()

	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.Begin()
	if err != nil {
		return date, 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO resource_snapshots (resource_type, resource_id, data, snapshot_date, captured_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(resource_type, resource_id, snapshot_date) DO UPDATE SET data = excluded.data, captured_at = excluded.captured_at
	`)
	if err != nil {
		return date, 0, err
	}
	defer stmt.Close()

	for _, r := range buffered {
		if _, err := stmt.Exec(r.resourceType, r.id, r.data, date, now); err != nil {
			return date, 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return date, 0, err
	}
	return date, len(buffered), nil
}

// SnapshotDates returns every distinct snapshot_date on record, oldest first.
func (s *Store) SnapshotDates(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT snapshot_date FROM resource_snapshots ORDER BY snapshot_date ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var dates []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		dates = append(dates, d)
	}
	return dates, rows.Err()
}

// NearestSnapshotDateOnOrBefore returns the latest snapshot_date that is <=
// the given date (YYYY-MM-DD). ok is false when no snapshot qualifies.
func (s *Store) NearestSnapshotDateOnOrBefore(ctx context.Context, date string) (string, bool, error) {
	var found sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT MAX(snapshot_date) FROM resource_snapshots WHERE snapshot_date <= ?`, date,
	).Scan(&found)
	if err != nil {
		return "", false, err
	}
	if !found.Valid || found.String == "" {
		return "", false, nil
	}
	return found.String, true, nil
}

// SnapshotRows returns every resource of the given type as it existed on the
// given snapshot date.
func (s *Store) SnapshotRows(ctx context.Context, date, resourceType string) ([]SnapshotRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT resource_id, data FROM resource_snapshots WHERE snapshot_date = ? AND resource_type = ?`,
		date, resourceType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SnapshotRow
	for rows.Next() {
		var r SnapshotRow
		var data string
		if err := rows.Scan(&r.ResourceID, &data); err != nil {
			return nil, err
		}
		r.Data = json.RawMessage(data)
		out = append(out, r)
	}
	return out, rows.Err()
}
