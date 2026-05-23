package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/vault"
)

// Store is the SQLite-backed local index of the vault. The on-disk schema
// is at version 1; bumping the constant triggers a re-sync (drop + recreate).
const SchemaVersion = 1

type Store struct {
	db   *sql.DB
	path string
}

// Open opens (or creates) the SQLite store at the given path, running migrations.
func Open(path string) (*Store, error) {
	return OpenWithContext(context.Background(), path)
}

func OpenWithContext(ctx context.Context, path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating store dir: %w", err)
	}
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, path: path}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// DB returns the underlying *sql.DB. Used by SQL commands and tests.
func (s *Store) DB() *sql.DB { return s.db }

// Path returns the on-disk path.
func (s *Store) Path() string { return s.path }

func (s *Store) migrate(ctx context.Context) error {
	schema := `
CREATE TABLE IF NOT EXISTS meta (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS notes (
	path TEXT PRIMARY KEY,
	abs_path TEXT NOT NULL,
	type TEXT,
	date TEXT,
	description TEXT,
	status TEXT,
	superseded_by TEXT,
	facts_file TEXT,
	mtime INTEGER NOT NULL,
	size INTEGER NOT NULL,
	body_text TEXT,
	body_hash TEXT,
	has_fm INTEGER NOT NULL DEFAULT 0,
	layer TEXT
);
CREATE INDEX IF NOT EXISTS notes_type_idx ON notes(type);
CREATE INDEX IF NOT EXISTS notes_status_idx ON notes(status);
CREATE INDEX IF NOT EXISTS notes_layer_idx ON notes(layer);
CREATE INDEX IF NOT EXISTS notes_mtime_idx ON notes(mtime);
CREATE INDEX IF NOT EXISTS notes_date_idx ON notes(date);

CREATE TABLE IF NOT EXISTS frontmatter_fields (
	path TEXT NOT NULL REFERENCES notes(path) ON DELETE CASCADE,
	key TEXT NOT NULL,
	value TEXT,
	PRIMARY KEY (path, key)
);

CREATE TABLE IF NOT EXISTS tags (
	path TEXT NOT NULL REFERENCES notes(path) ON DELETE CASCADE,
	tag TEXT NOT NULL,
	source TEXT NOT NULL, -- 'frontmatter' or 'inline'
	PRIMARY KEY (path, tag, source)
);
CREATE INDEX IF NOT EXISTS tags_tag_idx ON tags(tag);

CREATE TABLE IF NOT EXISTS links (
	from_path TEXT NOT NULL REFERENCES notes(path) ON DELETE CASCADE,
	to_target TEXT NOT NULL,
	resolved_path TEXT,
	PRIMARY KEY (from_path, to_target)
);
CREATE INDEX IF NOT EXISTS links_to_target_idx ON links(to_target);
CREATE INDEX IF NOT EXISTS links_resolved_idx ON links(resolved_path);

CREATE TABLE IF NOT EXISTS facts (
	id TEXT NOT NULL,
	parent_note_path TEXT NOT NULL REFERENCES notes(path) ON DELETE CASCADE,
	fact TEXT NOT NULL,
	category TEXT,
	timestamp TEXT,
	status TEXT,
	source TEXT,
	decision_trace_id TEXT,
	storage TEXT NOT NULL, -- 'inline' or 'toml'
	PRIMARY KEY (parent_note_path, id, storage)
);
CREATE INDEX IF NOT EXISTS facts_trace_idx ON facts(decision_trace_id);
CREATE INDEX IF NOT EXISTS facts_parent_idx ON facts(parent_note_path);

CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(
	path UNINDEXED,
	title,
	description,
	body,
	frontmatter_text,
	tokenize='porter unicode61'
);
`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	// Record schema version.
	_, _ = s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO meta(key, value) VALUES('schema_version', ?)`,
		fmt.Sprintf("%d", SchemaVersion))
	return nil
}

// NoteRow is the indexed projection of a vault note.
type NoteRow struct {
	Path         string
	AbsPath      string
	Type         string
	Date         string
	Description  string
	Status       string
	SupersededBy string
	FactsFile    string
	Mtime        int64
	Size         int64
	HasFM        bool
	Layer        string
	BodyHash     string
}

// UpsertNote replaces a note row + dependent rows (tags, links, facts, FTS) in one transaction.
func (s *Store) UpsertNote(ctx context.Context, n *vault.Note, mtime int64, size int64, layer string, bodyHash string, extra map[string]string, tags []TagEntry, links []LinkEntry, facts []FactEntry, title string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM notes WHERE path = ?`, n.Path); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO notes(path, abs_path, type, date, description, status, superseded_by, facts_file, mtime, size, body_text, body_hash, has_fm, layer)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.Path, n.AbsPath, n.Frontmatter.Type, n.Frontmatter.Date,
		n.Frontmatter.Description, n.Frontmatter.Status, n.Frontmatter.SupersededBy,
		n.Frontmatter.FactsFile, mtime, size, n.Body, bodyHash, n.HasFM, layer,
	); err != nil {
		return err
	}
	for k, v := range extra {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO frontmatter_fields(path, key, value) VALUES (?, ?, ?)`,
			n.Path, k, v); err != nil {
			return err
		}
	}
	for _, t := range tags {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO tags(path, tag, source) VALUES (?, ?, ?)`,
			n.Path, t.Tag, t.Source); err != nil {
			return err
		}
	}
	for _, l := range links {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO links(from_path, to_target, resolved_path) VALUES (?, ?, ?)`,
			n.Path, l.Target, l.ResolvedPath); err != nil {
			return err
		}
	}
	for _, f := range facts {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO facts(id, parent_note_path, fact, category, timestamp, status, source, decision_trace_id, storage)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			f.ID, n.Path, f.Fact, f.Category, f.Timestamp, f.Status, f.Source, f.DecisionTraceID, f.Storage); err != nil {
			return err
		}
	}
	// FTS upsert
	if _, err := tx.ExecContext(ctx, `DELETE FROM notes_fts WHERE path = ?`, n.Path); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO notes_fts(path, title, description, body, frontmatter_text) VALUES (?, ?, ?, ?, ?)`,
		n.Path, title, n.Frontmatter.Description, n.Body, n.Frontmatter.Raw); err != nil {
		return err
	}
	return tx.Commit()
}

// DeletePath removes a note + dependents (tags, links, facts, fts).
func (s *Store) DeletePath(ctx context.Context, path string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM notes_fts WHERE path = ?`, path); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM notes WHERE path = ?`, path); err != nil {
		return err
	}
	return tx.Commit()
}

// AllPaths returns every indexed note path.
func (s *Store) AllPaths(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT path FROM notes ORDER BY path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// MtimeByPath returns the indexed mtime for a path (0 if not indexed).
func (s *Store) MtimeByPath(ctx context.Context, path string) (int64, error) {
	var mt int64
	err := s.db.QueryRowContext(ctx, `SELECT mtime FROM notes WHERE path = ?`, path).Scan(&mt)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return mt, nil
}

// Search returns paths matching an FTS5 query, ordered by rank.
type SearchHit struct {
	Path        string
	Description string
	Snippet     string
	Rank        float64
}

func (s *Store) Search(ctx context.Context, query string, limit int) ([]SearchHit, error) {
	if limit <= 0 {
		limit = 50
	}
	// Empty/whitespace query returns no rows (FTS5 errors on empty MATCH).
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT n.path,
		       COALESCE(n.description, ''),
		       snippet(notes_fts, 3, '<mark>', '</mark>', '…', 12),
		       bm25(notes_fts) AS rnk
		FROM notes_fts
		JOIN notes n ON n.path = notes_fts.path
		WHERE notes_fts MATCH ?
		ORDER BY rnk
		LIMIT ?`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("fts query: %w", err)
	}
	defer rows.Close()
	var out []SearchHit
	for rows.Next() {
		var h SearchHit
		if err := rows.Scan(&h.Path, &h.Description, &h.Snippet, &h.Rank); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

type TagEntry struct {
	Tag    string
	Source string
}

type LinkEntry struct {
	Target       string
	ResolvedPath string
}

type FactEntry struct {
	ID              string
	Fact            string
	Category        string
	Timestamp       string
	Status          string
	Source          string
	DecisionTraceID string
	Storage         string // "inline" or "toml"
}

// Set/Get meta values.
func (s *Store) SetMeta(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR REPLACE INTO meta(key, value) VALUES(?, ?)`, key, value)
	return err
}
func (s *Store) GetMeta(ctx context.Context, key string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}
