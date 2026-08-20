package store

import (
	"context"
	"database/sql"
	"fmt"
)

// EnsureDrudgeSchema creates the Drudge snapshot schema if it is not already present.
func EnsureDrudgeSchema(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin drudge schema transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	statements := []string{
		`CREATE TABLE IF NOT EXISTS drudge_snapshot (
			snapshot_id TEXT PRIMARY KEY,
			captured_at TEXT NOT NULL,
			source_url TEXT NOT NULL,
			body_hash TEXT NOT NULL,
			story_count INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS drudge_story (
			snapshot_id TEXT NOT NULL,
			story_id TEXT NOT NULL,
			title TEXT NOT NULL,
			url TEXT NOT NULL,
			slot TEXT NOT NULL,
			slot_index INTEGER NOT NULL,
			is_red INTEGER NOT NULL,
			has_image INTEGER NOT NULL,
			image_url TEXT,
			outbound_domain TEXT NOT NULL,
			captured_at TEXT NOT NULL,
			PRIMARY KEY (snapshot_id, story_id),
			FOREIGN KEY (snapshot_id) REFERENCES drudge_snapshot(snapshot_id)
		)`,
		`CREATE TABLE IF NOT EXISTS drudge_slot_event (
			event_id TEXT PRIMARY KEY,
			snapshot_id TEXT NOT NULL,
			story_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			from_slot TEXT,
			to_slot TEXT,
			captured_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS drudge_story_story_id_idx ON drudge_story(story_id)`,
		`CREATE INDEX IF NOT EXISTS drudge_story_slot_idx ON drudge_story(slot)`,
		`CREATE INDEX IF NOT EXISTS drudge_story_captured_at ON drudge_story(captured_at)`,
		`CREATE INDEX IF NOT EXISTS drudge_story_outbound_idx ON drudge_story(outbound_domain)`,
		`CREATE INDEX IF NOT EXISTS drudge_slot_event_story_idx ON drudge_slot_event(story_id)`,
		`CREATE INDEX IF NOT EXISTS drudge_slot_event_ts_idx ON drudge_slot_event(captured_at)`,
		`CREATE INDEX IF NOT EXISTS drudge_snapshot_captured_at ON drudge_snapshot(captured_at)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS drudge_story_fts USING fts5(story_id UNINDEXED, title)`,
		`CREATE TRIGGER IF NOT EXISTS drudge_story_fts_ai AFTER INSERT ON drudge_story BEGIN
			INSERT INTO drudge_story_fts(rowid, story_id, title) VALUES (new.rowid, new.story_id, new.title);
		END`,
		`CREATE TRIGGER IF NOT EXISTS drudge_story_fts_au AFTER UPDATE ON drudge_story BEGIN
			DELETE FROM drudge_story_fts WHERE rowid = old.rowid;
			INSERT INTO drudge_story_fts(rowid, story_id, title) VALUES (new.rowid, new.story_id, new.title);
		END`,
		`CREATE TRIGGER IF NOT EXISTS drudge_story_fts_ad AFTER DELETE ON drudge_story BEGIN
			DELETE FROM drudge_story_fts WHERE rowid = old.rowid;
		END`,
	}
	for _, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("apply drudge schema statement: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit drudge schema transaction: %w", err)
	}
	committed = true
	return nil
}
