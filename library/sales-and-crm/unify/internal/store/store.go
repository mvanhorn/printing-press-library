// Package store backs the local read layer for unify-pp-cli with a SQLite
// database. The Unify Data API has no LIST or SEARCH endpoint for records, so
// every read workflow is computed against this local mirror, populated by the
// sync command via find-unique calls and direct ID GETs.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store is a thin wrapper around a *sql.DB pointing at the local SQLite
// mirror. Use Open to construct one; Close to release the file handle.
type Store struct {
	DB   *sql.DB
	Path string
}

// DefaultPath returns ~/.local/share/unify-pp-cli/store.db, creating the
// parent dir when needed. We intentionally avoid ~/.cache: the generated
// HTTP client wipes the cache dir whole-sale after every mutation, which
// would take the store with it. ~/.local/share is the XDG data dir, which
// is the correct place for application data anyway.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "store.db"
	}
	dir := filepath.Join(home, ".local", "share", "unify-pp-cli")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "store.db")
}

// Open opens (or creates) the SQLite store at path and runs the schema
// migrations. The returned Store is safe for concurrent reads; writes should
// happen inside a single goroutine or behind explicit transactions.
func Open(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		path = DefaultPath()
	}
	if dir := filepath.Dir(path); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping store: %w", err)
	}
	// Apply pragmas via Exec. Connection-pool note: database/sql may dispatch
	// queries across multiple connections, so PRAGMA settings that need to be
	// per-connection (e.g. foreign_keys) won't always stick. journal_mode and
	// busy_timeout are database-wide and persist across connections.
	for _, p := range []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA foreign_keys = ON",
	} {
		if _, err := db.ExecContext(ctx, p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("pragma %s: %w", p, err)
		}
	}
	// Bound the connection pool to 1 so writes from sequential helpers don't
	// race against a fresh connection that hasn't seen the prior write yet.
	// modernc.org/sqlite serializes well under this; throughput is fine for a
	// CLI's sync workload.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s := &Store{DB: db, Path: path}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close releases the file handle.
func (s *Store) Close() error { return s.DB.Close() }

// migrate creates the static schema tables if they don't exist. Per-object
// record tables are created on demand by EnsureRecordTable.
func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS objects (
			api_name TEXT PRIMARY KEY,
			provider TEXT NOT NULL,
			category TEXT NOT NULL,
			display_name TEXT,
			description TEXT,
			data TEXT NOT NULL,
			synced_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS attributes (
			object_name TEXT NOT NULL,
			api_name TEXT NOT NULL,
			type TEXT,
			display_name TEXT,
			description TEXT,
			is_unique INTEGER DEFAULT 0,
			data TEXT NOT NULL,
			synced_at INTEGER NOT NULL,
			PRIMARY KEY (object_name, api_name)
		)`,
		`CREATE TABLE IF NOT EXISTS attribute_options (
			object_name TEXT NOT NULL,
			attribute_name TEXT NOT NULL,
			option_name TEXT NOT NULL,
			display_name TEXT,
			data TEXT NOT NULL,
			synced_at INTEGER NOT NULL,
			PRIMARY KEY (object_name, attribute_name, option_name)
		)`,
		`CREATE TABLE IF NOT EXISTS watchlist (
			object_name TEXT NOT NULL,
			match_key TEXT NOT NULL,
			match_value TEXT NOT NULL,
			added_at INTEGER NOT NULL,
			PRIMARY KEY (object_name, match_key, match_value)
		)`,
		`CREATE TABLE IF NOT EXISTS schema_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			label TEXT,
			taken_at INTEGER NOT NULL,
			data TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_taken ON schema_snapshots(taken_at DESC)`,
	}
	for _, q := range stmts {
		if _, err := s.DB.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("exec: %s: %w", firstLine(q), err)
		}
	}
	return nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i > 0 {
		return s[:i]
	}
	return s
}

// RecordTable returns the SQLite table name for a given Unify object.
// Examples: "company" -> "record_company", "salesforce_account" -> "record_salesforce_account".
// All non-alnum characters are replaced with underscores.
func RecordTable(objectName string) string {
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, objectName)
	return "record_" + strings.ToLower(safe)
}

// EnsureRecordTable creates the per-object record table on demand. Each
// record table holds one row per record with id (PK), object_name,
// created_at, updated_at, attrs (JSON blob), and a synced_at timestamp.
// Per-attribute typed columns are NOT created automatically — SQL queries
// extract attrs via json_extract(attrs, '$.<name>'). FTS coverage is added
// by EnsureFTS.
func (s *Store) EnsureRecordTable(ctx context.Context, objectName string) error {
	table := RecordTable(objectName)
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %q (
		id TEXT PRIMARY KEY,
		object_name TEXT NOT NULL,
		created_at TEXT,
		updated_at TEXT,
		attrs TEXT NOT NULL,
		synced_at INTEGER NOT NULL
	)`, table)
	if _, err := s.DB.ExecContext(ctx, q); err != nil {
		return fmt.Errorf("create %s: %w", table, err)
	}
	return nil
}

// UpsertRecord inserts or replaces one record into the per-object table.
// attrs is the typed attribute payload (the API's record.attributes object).
func (s *Store) UpsertRecord(ctx context.Context, objectName, id, createdAt, updatedAt string, attrs map[string]any) error {
	if err := s.EnsureRecordTable(ctx, objectName); err != nil {
		return err
	}
	blob, err := json.Marshal(attrs)
	if err != nil {
		return fmt.Errorf("marshal attrs: %w", err)
	}
	table := RecordTable(objectName)
	q := fmt.Sprintf(`INSERT OR REPLACE INTO %q (id, object_name, created_at, updated_at, attrs, synced_at) VALUES (?, ?, ?, ?, ?, ?)`, table)
	if _, err := s.DB.ExecContext(ctx, q, id, objectName, createdAt, updatedAt, string(blob), time.Now().Unix()); err != nil {
		return fmt.Errorf("upsert record: %w", err)
	}
	return s.indexRecordFTS(ctx, objectName, id, attrs)
}

// EnsureFTS creates the records_fts virtual table once. Rows in records_fts
// are keyed by (object_name, id) and hold a flattened text blob of the
// record's stringy attribute values.
func (s *Store) EnsureFTS(ctx context.Context) error {
	q := `CREATE VIRTUAL TABLE IF NOT EXISTS records_fts USING fts5(
		object_name UNINDEXED,
		id UNINDEXED,
		body
	)`
	_, err := s.DB.ExecContext(ctx, q)
	return err
}

func (s *Store) indexRecordFTS(ctx context.Context, objectName, id string, attrs map[string]any) error {
	if err := s.EnsureFTS(ctx); err != nil {
		return err
	}
	body := flattenForFTS(attrs)
	// Replace any prior row for this (object, id) — fts5 has no PK, so delete-then-insert.
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM records_fts WHERE object_name = ? AND id = ?`, objectName, id); err != nil {
		return err
	}
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO records_fts (object_name, id, body) VALUES (?, ?, ?)`, objectName, id, body); err != nil {
		return err
	}
	return nil
}

// flattenForFTS produces a single space-separated string from all stringy
// values in a record's attribute map. Non-string scalars are stringified;
// nested objects/arrays are walked.
func flattenForFTS(attrs map[string]any) string {
	var b strings.Builder
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case string:
			b.WriteString(t)
			b.WriteByte(' ')
		case float64:
			fmt.Fprintf(&b, "%v ", t)
		case bool:
			fmt.Fprintf(&b, "%v ", t)
		case []any:
			for _, it := range t {
				walk(it)
			}
		case map[string]any:
			for _, vv := range t {
				walk(vv)
			}
		}
	}
	for k, v := range attrs {
		b.WriteString(k)
		b.WriteByte(':')
		b.WriteByte(' ')
		walk(v)
	}
	return b.String()
}

// ListRecordTables returns every per-object record table currently in the DB.
func (s *Store) ListRecordTables(ctx context.Context) ([]string, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name LIKE 'record_%'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// CountRecords returns how many records are stored for an object.
func (s *Store) CountRecords(ctx context.Context, objectName string) (int, error) {
	if err := s.EnsureRecordTable(ctx, objectName); err != nil {
		return 0, err
	}
	table := RecordTable(objectName)
	var n int
	q := fmt.Sprintf(`SELECT COUNT(*) FROM %q`, table)
	if err := s.DB.QueryRowContext(ctx, q).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}
