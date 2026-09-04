// Copyright 2026 Paul Gradeff and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: local SQLite tables for new-since diffing and body watchlists.

package cli

import (
	"context"
	"database/sql"
)

// ensurePMNTables creates the hand-authored tables if they don't exist.
func ensurePMNTables(ctx context.Context, db *sql.DB) error {
	const ddl = `
CREATE TABLE IF NOT EXISTS pmn_seen_notices (
	notice_id     INTEGER PRIMARY KEY,
	first_seen    TEXT NOT NULL,
	public_body   TEXT,
	meeting_start TEXT,
	meeting_title TEXT
);
CREATE TABLE IF NOT EXISTS pmn_watch_bodies (
	name  TEXT PRIMARY KEY,
	added TEXT NOT NULL
);`
	_, err := db.ExecContext(ctx, ddl)
	return err
}

// noticeSeen reports whether a notice ID is already recorded.
func noticeSeen(ctx context.Context, db *sql.DB, id int64) (bool, error) {
	var one int
	err := db.QueryRowContext(ctx, `SELECT 1 FROM pmn_seen_notices WHERE notice_id = ?`, id).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// recordNotice inserts a notice into the seen set (ignoring duplicates).
func recordNotice(ctx context.Context, db *sql.DB, n pmnNotice, now string) error {
	_, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO pmn_seen_notices (notice_id, first_seen, public_body, meeting_start, meeting_title)
		 VALUES (?, ?, ?, ?, ?)`,
		n.NoticeID, now, n.PublicBodyName, n.MeetingStartTime, n.MeetingTitle)
	return err
}

// watchList returns the saved body watchlist (lowercased names).
func watchList(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT name FROM pmn_watch_bodies ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name sql.NullString
		if err := rows.Scan(&name); err != nil {
			continue
		}
		out = append(out, name.String)
	}
	return out, rows.Err()
}

func watchAdd(ctx context.Context, db *sql.DB, name, now string) error {
	_, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO pmn_watch_bodies (name, added) VALUES (?, ?)`, name, now)
	return err
}

func watchRemove(ctx context.Context, db *sql.DB, name string) (int64, error) {
	res, err := db.ExecContext(ctx, `DELETE FROM pmn_watch_bodies WHERE name = ?`, name)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
