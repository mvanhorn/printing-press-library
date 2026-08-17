// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored store extension: comment corpus (threads + FTS), post coverage
// metadata, and tagged-post snapshots. Lazy-init from the novel commands that
// need it, per the printing-press separate-file extension pattern.

package store

import (
	"context"
	"database/sql"
	"time"
)

// EnsureCommentCorpus creates the comment rows table, its FTS5 index, and the
// per-post coverage metadata table.
func EnsureCommentCorpus(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sc_comments (
			comment_id TEXT PRIMARY KEY,
			post_url TEXT NOT NULL,
			parent_id TEXT NOT NULL DEFAULT '',
			text TEXT NOT NULL DEFAULT '',
			like_count INTEGER NOT NULL DEFAULT 0,
			platform TEXT NOT NULL DEFAULT 'instagram',
			captured_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sc_comments_post ON sc_comments(post_url)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS sc_comments_fts USING fts5(comment_id UNINDEXED, post_url UNINDEXED, text)`,
		`CREATE TABLE IF NOT EXISTS sc_post_meta (
			post_url TEXT PRIMARY KEY,
			handle TEXT NOT NULL DEFAULT '',
			reported_comment_count INTEGER NOT NULL DEFAULT 0,
			captured_at TEXT NOT NULL
		)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

// CommentRow is one stored comment or reply (replies carry a parent_id).
type CommentRow struct {
	CommentID string
	PostURL   string
	ParentID  string
	Text      string
	LikeCount int64
	Platform  string
}

// UpsertCommentRows writes comment rows and their FTS entries in one
// transaction owned by this helper (never call inside an open write tx).
func UpsertCommentRows(ctx context.Context, db *sql.DB, rows []CommentRow) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, r := range rows {
		if r.CommentID == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO sc_comments (comment_id, post_url, parent_id, text, like_count, platform, captured_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(comment_id) DO UPDATE SET text=excluded.text, like_count=excluded.like_count, captured_at=excluded.captured_at`,
			r.CommentID, r.PostURL, r.ParentID, r.Text, r.LikeCount, r.Platform, now); err != nil {
			_ = tx.Rollback()
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM sc_comments_fts WHERE comment_id = ?`, r.CommentID); err != nil {
			_ = tx.Rollback()
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO sc_comments_fts (comment_id, post_url, text) VALUES (?, ?, ?)`,
			r.CommentID, r.PostURL, r.Text); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// UpsertPostMeta records the API-reported comment count for a post so the
// coverage audit can compare it against stored rows later. An empty handle
// never overwrites a handle a previous caller stored for the same post_url
// (thread runs post-URL-first and does not know the handle sweep knew).
func UpsertPostMeta(ctx context.Context, db *sql.DB, postURL, handle string, reportedCount int64) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO sc_post_meta (post_url, handle, reported_comment_count, captured_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(post_url) DO UPDATE SET handle=CASE WHEN excluded.handle='' THEN sc_post_meta.handle ELSE excluded.handle END, reported_comment_count=excluded.reported_comment_count, captured_at=excluded.captured_at`,
		postURL, handle, reportedCount, time.Now().UTC().Format(time.RFC3339))
	return err
}

// EnsureTaggedSnapshots creates the tagged-post snapshot table used by
// `creator tagged` (same snapshot+diff shape as the ads monitor).
func EnsureTaggedSnapshots(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS sc_tagged_snapshots (
		batch_id TEXT NOT NULL,
		handle TEXT NOT NULL,
		post_id TEXT NOT NULL,
		captured_at TEXT NOT NULL,
		PRIMARY KEY (batch_id, post_id)
	)`)
	return err
}

// LatestTaggedSnapshot returns the newest snapshot batch's post IDs for a handle.
func LatestTaggedSnapshot(ctx context.Context, db *sql.DB, handle string) (batchID string, postIDs []string, err error) {
	row := db.QueryRowContext(ctx,
		`SELECT batch_id FROM sc_tagged_snapshots WHERE handle = ? ORDER BY captured_at DESC, batch_id DESC LIMIT 1`, handle)
	if err := row.Scan(&batchID); err != nil {
		if err == sql.ErrNoRows {
			return "", nil, nil
		}
		return "", nil, err
	}
	rows, err := db.QueryContext(ctx,
		`SELECT post_id FROM sc_tagged_snapshots WHERE handle = ? AND batch_id = ?`, handle, batchID)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", nil, err
		}
		postIDs = append(postIDs, id)
	}
	return batchID, postIDs, rows.Err()
}

// InsertTaggedSnapshot writes a new snapshot batch of tagged-post IDs.
func InsertTaggedSnapshot(ctx context.Context, db *sql.DB, handle string, postIDs []string, capturedAt time.Time) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	batch := capturedAt.UTC().Format("20060102-150405.000000000")
	ts := capturedAt.UTC().Format(time.RFC3339)
	for _, id := range postIDs {
		if id == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO sc_tagged_snapshots (batch_id, handle, post_id, captured_at) VALUES (?, ?, ?, ?)`,
			batch, handle, id, ts); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
