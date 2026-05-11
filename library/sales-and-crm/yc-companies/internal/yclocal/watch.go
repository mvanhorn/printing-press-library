package yclocal

import (
	"context"
	"database/sql"
	"fmt"
)

// WatchEntry describes a single tracked company slug.
type WatchEntry struct {
	Slug    string `json:"slug"`
	AddedAt string `json:"added_at"`
}

// WatchAdd inserts slugs into the watch table.
// Returns (added, skipped) — skipped are slugs that were already watched
// (or do not exist in the companies table).
func WatchAdd(ctx context.Context, db *sql.DB, slugs []string) (added []string, skipped []string, err error) {
	if err := EnsureSchema(db); err != nil {
		return nil, nil, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	for _, slug := range slugs {
		if slug == "" {
			continue
		}
		var exists int
		_ = tx.QueryRowContext(ctx, `SELECT 1 FROM companies WHERE slug = ? LIMIT 1`, slug).Scan(&exists)
		if exists == 0 {
			skipped = append(skipped, slug)
			continue
		}
		res, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO watch (slug) VALUES (?)`, slug)
		if err != nil {
			return nil, nil, fmt.Errorf("watch add %q: %w", slug, err)
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			skipped = append(skipped, slug)
		} else {
			added = append(added, slug)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	return added, skipped, nil
}

// WatchRemove deletes slugs from the watch table.
func WatchRemove(ctx context.Context, db *sql.DB, slugs []string) (removed []string, missed []string, err error) {
	if err := EnsureSchema(db); err != nil {
		return nil, nil, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()
	for _, slug := range slugs {
		if slug == "" {
			continue
		}
		res, err := tx.ExecContext(ctx, `DELETE FROM watch WHERE slug = ?`, slug)
		if err != nil {
			return nil, nil, fmt.Errorf("watch remove %q: %w", slug, err)
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			missed = append(missed, slug)
		} else {
			removed = append(removed, slug)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	return removed, missed, nil
}

// WatchList returns all watched slugs joined with their current company row.
type WatchedCompany struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Batch    string `json:"batch"`
	Status   string `json:"status"`
	TeamSize int    `json:"team_size"`
	IsHiring bool   `json:"is_hiring"`
	AddedAt  string `json:"added_at"`
}

func WatchList(ctx context.Context, db *sql.DB) ([]WatchedCompany, error) {
	if err := EnsureSchema(db); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `
SELECT w.slug,
       COALESCE(c.name, ''),
       COALESCE(c.batch, ''),
       COALESCE(c.status, ''),
       COALESCE(c.team_size, 0),
       COALESCE(c.is_hiring, 0),
       w.added_at
FROM watch w
LEFT JOIN companies c ON c.slug = w.slug
ORDER BY w.added_at DESC, w.slug ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WatchedCompany
	for rows.Next() {
		var w WatchedCompany
		var hiring int
		if err := rows.Scan(&w.Slug, &w.Name, &w.Batch, &w.Status, &w.TeamSize, &hiring, &w.AddedAt); err != nil {
			return nil, err
		}
		w.IsHiring = hiring != 0
		out = append(out, w)
	}
	return out, rows.Err()
}

// WatchedSlugs returns the bare list of watched slugs.
func WatchedSlugs(ctx context.Context, db *sql.DB) ([]string, error) {
	if err := EnsureSchema(db); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `SELECT slug FROM watch ORDER BY slug ASC`)
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
