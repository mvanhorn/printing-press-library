// Novel-feature auxiliary tables. CREATE TABLE IF NOT EXISTS makes this
// idempotent across runs; the main store package is generator-emitted and
// must not be hand-edited, so we attach these lazily from the novel commands.

package cli

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/telegram/internal/store"
)

const novelSchemaDDL = `
CREATE TABLE IF NOT EXISTS telegram_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    bot_id TEXT NOT NULL,
    chat_id TEXT NOT NULL,
    message_id INTEGER NOT NULL,
    direction TEXT NOT NULL CHECK (direction IN ('outbound','inbound')),
    text TEXT,
    caption TEXT,
    media_type TEXT,
    parse_mode TEXT,
    date INTEGER NOT NULL,
    error TEXT,
    idempotency_key TEXT,
    publish_slug TEXT,
    publish_chunk_index INTEGER,
    content_hash TEXT,
    raw JSON,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (bot_id, chat_id, message_id)
);
CREATE INDEX IF NOT EXISTS idx_tg_messages_chat_date
    ON telegram_messages (bot_id, chat_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_tg_messages_idempotency
    ON telegram_messages (bot_id, chat_id, idempotency_key);
CREATE INDEX IF NOT EXISTS idx_tg_messages_publish
    ON telegram_messages (publish_slug, publish_chunk_index);

CREATE TABLE IF NOT EXISTS telegram_idempotency (
    bot_id TEXT NOT NULL,
    chat_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    message_id INTEGER NOT NULL,
    payload JSON NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (bot_id, chat_id, idempotency_key)
);

CREATE TABLE IF NOT EXISTS telegram_chats_resolved (
    bot_id TEXT NOT NULL,
    username TEXT NOT NULL,
    chat_id TEXT NOT NULL,
    chat_type TEXT,
    title TEXT,
    resolved_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (bot_id, username)
);
`

// openNovelStore opens the standard store and ensures the novel-feature
// tables exist. Reused by every novel command that touches local state.
func openNovelStore(ctx context.Context) (*store.Store, error) {
	dbPath := defaultDBPath("telegram-pp-cli")
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening store at %s: %w", dbPath, err)
	}
	if err := ensureNovelSchema(ctx, s.DB()); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

func ensureNovelSchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, novelSchemaDDL); err != nil {
		return fmt.Errorf("ensuring novel-feature tables: %w", err)
	}
	return nil
}
