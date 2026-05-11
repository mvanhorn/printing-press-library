package yclocal

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Snapshot describes one captured row in companies_history.
type Snapshot struct {
	SnapshotID string `json:"snapshot_id"`
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	TeamSize   int    `json:"team_size"`
	IsHiring   bool   `json:"is_hiring"`
	TopCompany bool   `json:"top_company"`
	Batch      string `json:"batch"`
	CapturedAt string `json:"captured_at"`
}

// CaptureSnapshot writes a new snapshot of the current companies table.
// The snapshot_id is the current ISO-8601 UTC timestamp.
// Returns the snapshot_id and the number of rows captured.
func CaptureSnapshot(ctx context.Context, db *sql.DB) (string, int64, error) {
	if err := EnsureSchema(db); err != nil {
		return "", 0, err
	}
	id := time.Now().UTC().Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `
INSERT OR REPLACE INTO companies_history
  (snapshot_id, slug, name, status, team_size, is_hiring, top_company, batch, captured_at)
SELECT ?, slug, name, status,
       COALESCE(team_size, 0),
       COALESCE(is_hiring, 0),
       COALESCE(top_company, 0),
       batch,
       datetime('now')
FROM companies WHERE slug IS NOT NULL AND slug <> ''`, id)
	if err != nil {
		return "", 0, fmt.Errorf("capture snapshot: %w", err)
	}
	rows, _ := res.RowsAffected()
	return id, rows, nil
}

// LatestSnapshotID returns the most recent snapshot_id, or empty string if none.
func LatestSnapshotID(ctx context.Context, db *sql.DB) (string, error) {
	if err := EnsureSchema(db); err != nil {
		return "", err
	}
	var id sql.NullString
	err := db.QueryRowContext(ctx, `SELECT snapshot_id FROM companies_history ORDER BY snapshot_id DESC LIMIT 1`).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return id.String, nil
}

// SnapshotAtOrBefore returns the most recent snapshot_id at or before t.
// Returns empty string if no snapshot exists at or before t.
func SnapshotAtOrBefore(ctx context.Context, db *sql.DB, t time.Time) (string, error) {
	if err := EnsureSchema(db); err != nil {
		return "", err
	}
	cutoff := t.UTC().Format(time.RFC3339)
	var id sql.NullString
	err := db.QueryRowContext(ctx, `SELECT snapshot_id FROM companies_history WHERE snapshot_id <= ? ORDER BY snapshot_id DESC LIMIT 1`, cutoff).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return id.String, nil
}

// ListSnapshots returns all snapshot IDs ordered newest first.
func ListSnapshots(ctx context.Context, db *sql.DB) ([]string, error) {
	if err := EnsureSchema(db); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `SELECT DISTINCT snapshot_id FROM companies_history ORDER BY snapshot_id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// EnsureRecentSnapshot captures a new snapshot if no snapshot exists or the
// latest snapshot is older than the requested staleness window. Returns the
// snapshot id that is now considered the latest.
func EnsureRecentSnapshot(ctx context.Context, db *sql.DB, maxAge time.Duration) (string, error) {
	latest, err := LatestSnapshotID(ctx, db)
	if err != nil {
		return "", err
	}
	if latest != "" {
		if ts, err := time.Parse(time.RFC3339, latest); err == nil {
			if time.Since(ts) < maxAge {
				return latest, nil
			}
		}
	}
	id, _, err := CaptureSnapshot(ctx, db)
	if err != nil {
		return "", err
	}
	return id, nil
}
