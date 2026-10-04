package lancet

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode"
)

// The word-match index is an external-content FTS5 table over lancet_works
// (title, topic) using the same tokenizer as the generated store's
// resources_fts. External-content keeps a single copy of the text (the real DB
// is large) and lets 'rebuild' backfill from the base table; the triggers
// below keep it in sync. A contentless table would need the same triggers plus
// a stored copy of the old values for deletes, with no storage benefit here.
const ftsTable = "lancet_works_fts"

// ftsRebuildHook runs inside the setup transaction right after the rebuild.
// Tests set it to inject a failure; production leaves it nil.
var ftsRebuildHook func() error

func ftsTableExists(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}) (bool, error) {
	var n int
	err := q.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, ftsTable).Scan(&n)
	return n > 0, err
}

// ensureWorksFTS creates the index, its triggers and the one-time backfill in a
// single write-locked transaction: either all of it commits or none of it does,
// so an interrupted backfill can never leave an empty index behind.
func ensureWorksFTS(ctx context.Context, db *sql.DB) error {
	if ok, err := ftsTableExists(ctx, db); err != nil {
		return fmt.Errorf("lancet fts: %w", err)
	} else if ok {
		return nil
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("lancet fts: %w", err)
	}
	defer conn.Close()
	// Per-connection wait so a concurrent opener blocks instead of failing; the
	// original value is restored before the connection returns to the pool.
	var prevBusy int
	if err := conn.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&prevBusy); err != nil {
		return fmt.Errorf("lancet fts: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), fmt.Sprintf(`PRAGMA busy_timeout = %d`, prevBusy))
	}()
	if _, err := conn.ExecContext(ctx, `PRAGMA busy_timeout = 30000`); err != nil {
		return fmt.Errorf("lancet fts: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return fmt.Errorf("lancet fts: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), `ROLLBACK`)
		}
	}()
	// Re-check under the write lock: another process may have finished first.
	if ok, err := ftsTableExists(ctx, conn); err != nil {
		return fmt.Errorf("lancet fts: %w", err)
	} else if ok {
		return nil
	}
	stmts := []string{
		`CREATE VIRTUAL TABLE lancet_works_fts USING fts5(
			title, topic, content='lancet_works', content_rowid='rowid', tokenize='porter unicode61')`,
		`CREATE TRIGGER IF NOT EXISTS lancet_works_fts_ai AFTER INSERT ON lancet_works BEGIN
			INSERT INTO lancet_works_fts(rowid, title, topic) VALUES (new.rowid, new.title, new.topic);
		END`,
		`CREATE TRIGGER IF NOT EXISTS lancet_works_fts_ad AFTER DELETE ON lancet_works BEGIN
			INSERT INTO lancet_works_fts(lancet_works_fts, rowid, title, topic) VALUES ('delete', old.rowid, old.title, old.topic);
		END`,
		`CREATE TRIGGER IF NOT EXISTS lancet_works_fts_au AFTER UPDATE ON lancet_works BEGIN
			INSERT INTO lancet_works_fts(lancet_works_fts, rowid, title, topic) VALUES ('delete', old.rowid, old.title, old.topic);
			INSERT INTO lancet_works_fts(rowid, title, topic) VALUES (new.rowid, new.title, new.topic);
		END`,
	}
	for _, s := range stmts {
		if _, err := conn.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("lancet fts: %w (FTS5 is required for curate word matching)", err)
		}
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO lancet_works_fts(lancet_works_fts) VALUES ('rebuild')`); err != nil {
		return fmt.Errorf("lancet fts backfill: %w", err)
	}
	if ftsRebuildHook != nil {
		if err := ftsRebuildHook(); err != nil {
			return fmt.Errorf("lancet fts backfill: %w", err)
		}
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return fmt.Errorf("lancet fts commit: %w", err)
	}
	committed = true
	return nil
}

// ftsMatchQuery turns free text into an FTS5 MATCH expression: the text is split
// on every rune that is not a letter, number (Nd, Nl, No), combining mark or
// private-use rune (mirroring the unicode61 token characters), each part becomes a double-quoted phrase (embedded quotes
// doubled), joined by implicit AND. Returns "" when nothing is left.
func ftsMatchQuery(topic string) string {
	parts := strings.FieldsFunc(topic, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsMark(r) && !unicode.Is(unicode.Co, r)
	})
	for i, p := range parts {
		parts[i] = `"` + strings.ReplaceAll(p, `"`, `""`) + `"`
	}
	return strings.Join(parts, " ")
}
