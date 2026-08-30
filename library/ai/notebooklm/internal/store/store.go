// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

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

	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/nlm"
	_ "modernc.org/sqlite"
)

// StoreSchemaVersion is the on-disk schema version this binary understands.
const StoreSchemaVersion = 1

// Store is a local SQLite cache for notebooks and chat text.
type Store struct {
	db   *sql.DB
	path string
}

// DefaultPath returns ~/.local/share/notebooklm-pp-cli/cache.db
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "notebooklm-pp-cli", "cache.db"), nil
}

// Open opens or creates the SQLite store.
func Open(path string) (*Store, error) {
	return OpenWithContext(context.Background(), path)
}

// OpenWithContext opens the store honoring ctx during migration.
func OpenWithContext(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, path: path}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS notebooks (
  id TEXT PRIMARY KEY,
  title TEXT,
  emoji TEXT,
  source_count INTEGER,
  payload TEXT,
  synced_at TEXT
)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS notebooks_fts USING fts5(id UNINDEXED, title, content, tokenize='porter')`,
		`CREATE TABLE IF NOT EXISTS sync_state (
  resource_type TEXT PRIMARY KEY,
  last_cursor TEXT,
  last_synced_at DATETIME,
  total_count INTEGER DEFAULT 0
)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	var userVersion int
	if err := s.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&userVersion); err != nil {
		return err
	}
	if userVersion > StoreSchemaVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d; upgrade the CLI binary", userVersion, StoreSchemaVersion)
	}
	if userVersion < StoreSchemaVersion {
		if _, err := s.db.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, StoreSchemaVersion)); err != nil {
			return err
		}
	}
	return nil
}

// DB exposes the underlying database for ad-hoc read-only queries.
func (s *Store) DB() *sql.DB {
	return s.db
}

// SchemaVersion returns PRAGMA user_version.
func (s *Store) SchemaVersion() (int, error) {
	var v int
	err := s.db.QueryRow(`PRAGMA user_version`).Scan(&v)
	return v, err
}

// SyncState tracks per-resource sync metadata.
type SyncState struct {
	ResourceType string
	LastCursor   string
	LastSyncedAt time.Time
	TotalCount   int64
}

// GetSyncState reads sync metadata for a resource type.
func (s *Store) GetSyncState(resourceType string) (*SyncState, error) {
	var st SyncState
	var last sql.NullTime
	err := s.db.QueryRow(
		`SELECT resource_type, COALESCE(last_cursor,''), last_synced_at, COALESCE(total_count,0) FROM sync_state WHERE resource_type = ?`,
		resourceType,
	).Scan(&st.ResourceType, &st.LastCursor, &last, &st.TotalCount)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if last.Valid {
		st.LastSyncedAt = last.Time
	}
	return &st, nil
}

// SaveSyncState upserts sync metadata after a successful sync.
func (s *Store) SaveSyncState(st SyncState) error {
	_, err := s.db.Exec(
		`INSERT INTO sync_state(resource_type, last_cursor, last_synced_at, total_count) VALUES(?,?,?,?)
		 ON CONFLICT(resource_type) DO UPDATE SET last_cursor=excluded.last_cursor, last_synced_at=excluded.last_synced_at, total_count=excluded.total_count`,
		st.ResourceType, st.LastCursor, st.LastSyncedAt.UTC(), st.TotalCount,
	)
	return err
}

// UpsertNotebook writes or updates one notebook row in the local cache.
func (s *Store) UpsertNotebook(nb nlm.Notebook, syncedAt string) error {
	b, _ := json.Marshal(nb)
	_, err := s.db.Exec(`INSERT INTO notebooks(id,title,emoji,source_count,payload,synced_at) VALUES(?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET title=excluded.title, emoji=excluded.emoji, source_count=excluded.source_count, payload=excluded.payload, synced_at=excluded.synced_at`,
		nb.ID, nb.Title, nb.Emoji, nb.SourceCount, string(b), syncedAt)
	if err != nil {
		return err
	}
	content := strings.Join([]string{nb.Title, nb.ID}, " ")
	_, _ = s.db.Exec(`DELETE FROM notebooks_fts WHERE id = ?`, nb.ID)
	_, _ = s.db.Exec(`INSERT INTO notebooks_fts(id, title, content) VALUES(?, ?, ?)`, nb.ID, nb.Title, content)
	return nil
}

// SyncNotebooks refreshes the local cache from the live API.
func (s *Store) SyncNotebooks(ctx context.Context, client *nlm.Client) (int, error) {
	nbs, err := client.ListNotebooks(ctx)
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, nb := range nbs {
		if err := s.UpsertNotebook(nb, now); err != nil {
			return 0, err
		}
	}
	if err := s.SaveSyncState(SyncState{
		ResourceType: "notebooks",
		LastSyncedAt: time.Now().UTC(),
		TotalCount:   int64(len(nbs)),
	}); err != nil {
		return 0, err
	}
	return len(nbs), nil
}

// SearchNotebooks finds notebooks by title or id substring in the local cache.
func (s *Store) SearchNotebooks(query string) ([]nlm.Notebook, error) {
	return s.searchNotebooks(query)
}

// ResolveByName returns a notebook when query matches id or title (name-or-ID resolution).
func (s *Store) ResolveByName(query string) (nlm.Notebook, error) {
	nbs, err := s.searchNotebooks(query)
	if err != nil {
		return nlm.Notebook{}, err
	}
	if len(nbs) == 0 {
		return nlm.Notebook{}, ErrNotFound
	}
	return nbs[0], nil
}

// IsUUID reports whether s looks like a NotebookLM notebook id.
func IsUUID(s string) bool {
	s = strings.TrimSpace(s)
	return len(s) >= 8 && !strings.Contains(s, " ")
}

func (s *Store) searchNotebooks(query string) ([]nlm.Notebook, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []nlm.Notebook{}, nil
	}
	rows, err := s.db.Query(`SELECT payload FROM notebooks WHERE title LIKE '%' || ? || '%' OR id LIKE '%' || ? || '%'`, query, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []nlm.Notebook
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var nb nlm.Notebook
		if err := json.Unmarshal([]byte(payload), &nb); err == nil {
			out = append(out, nb)
		}
	}
	if out == nil {
		out = []nlm.Notebook{}
	}
	return out, nil
}

// ReadOnlyQuery runs a bounded read-only SQL statement against the local cache.
func (s *Store) ReadOnlyQuery(ctx context.Context, query string, limit int) ([]map[string]any, error) {
	q := strings.TrimSpace(query)
	upper := strings.ToUpper(q)
	if strings.Contains(upper, "INSERT") || strings.Contains(upper, "UPDATE") ||
		strings.Contains(upper, "DELETE") || strings.Contains(upper, "DROP") ||
		strings.Contains(upper, "ALTER") || strings.Contains(upper, "CREATE") {
		return nil, fmt.Errorf("only read-only SELECT queries are allowed")
	}
	if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "WITH") {
		return nil, fmt.Errorf("query must start with SELECT or WITH")
	}
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var results []map[string]any
	for rows.Next() {
		if len(results) >= limit {
			break
		}
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			switch v := vals[i].(type) {
			case []byte:
				row[c] = string(v)
			default:
				row[c] = v
			}
		}
		results = append(results, row)
	}
	if results == nil {
		results = []map[string]any{}
	}
	return results, rows.Err()
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// PathInfo returns store file path for doctor output.
func (s *Store) PathInfo() string {
	if s.path != "" {
		return s.path
	}
	return "cache.db"
}

// ErrNotFound indicates missing cache entry.
var ErrNotFound = fmt.Errorf("not found in local cache")
