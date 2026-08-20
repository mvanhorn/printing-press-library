package cli

import (
	"context"
	"database/sql"
	"fmt"
)

// ensureMessagesTables creates the auxiliary tables that the novel
// commands (grep, decisions, mention inbox) query against. Idempotent.
//
// These tables exist alongside the generator-owned migrations. They are
// not part of the schema-version stamp because they are populated by
// novel-feature sync paths (chat.history fanout, transcript.info), not
// by the generic generator-emitted sync of single-id GET tables.
func ensureMessagesTables(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			chat_id TEXT NOT NULL,
			from_user TEXT,
			text TEXT NOT NULL,
			ts DATETIME NOT NULL,
			data JSON,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_chat ON messages(chat_id)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_ts ON messages(ts)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_from ON messages(from_user)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(text, content='messages', content_rowid='rowid')`,
		`CREATE TRIGGER IF NOT EXISTS messages_ai AFTER INSERT ON messages BEGIN
			INSERT INTO messages_fts(rowid, text) VALUES (new.rowid, new.text);
		END`,
		`CREATE TABLE IF NOT EXISTS transcript_text (
			transcript_id TEXT PRIMARY KEY,
			meeting_id TEXT,
			event_name TEXT,
			start DATETIME,
			text TEXT NOT NULL,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_transcript_text_start ON transcript_text(start)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS transcript_text_fts USING fts5(text, content='transcript_text', content_rowid='rowid')`,
		`CREATE TRIGGER IF NOT EXISTS transcript_text_ai AFTER INSERT ON transcript_text BEGIN
			INSERT INTO transcript_text_fts(rowid, text) VALUES (new.rowid, new.text);
		END`,
		`CREATE TABLE IF NOT EXISTS attendance (
			event_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			joined_at DATETIME,
			duration_sec INTEGER,
			data JSON,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (event_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS webhook_deliveries (
			id TEXT PRIMARY KEY,
			subscription_id TEXT,
			event TEXT NOT NULL,
			received_at DATETIME NOT NULL,
			payload JSON,
			http_status INTEGER
		)`,
		`CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_received ON webhook_deliveries(received_at)`,
		`CREATE TABLE IF NOT EXISTS webhook_subscriptions (
			id TEXT PRIMARY KEY,
			event TEXT NOT NULL,
			target_url TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			data JSON
		)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("ensure messages schema: %w", err)
		}
	}
	return nil
}
