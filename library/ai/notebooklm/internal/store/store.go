package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

const schemaVersion = 1

const createSQL = `
CREATE TABLE IF NOT EXISTS notebooks (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	source_count INTEGER DEFAULT 0,
	updated_at TEXT,
	synced_at TEXT
);
CREATE TABLE IF NOT EXISTS sources (
	id TEXT PRIMARY KEY,
	notebook_id TEXT NOT NULL,
	title TEXT,
	type TEXT,
	url TEXT,
	synced_at TEXT,
	FOREIGN KEY (notebook_id) REFERENCES notebooks(id)
);
CREATE TABLE IF NOT EXISTS content (
	source_id TEXT PRIMARY KEY,
	notebook_id TEXT NOT NULL,
	title TEXT,
	text TEXT,
	FOREIGN KEY (source_id) REFERENCES sources(id),
	FOREIGN KEY (notebook_id) REFERENCES notebooks(id)
);
CREATE VIRTUAL TABLE IF NOT EXISTS content_fts USING fts5(
	source_id, notebook_id, title, text,
	tokenize='porter unicode61'
);
PRAGMA user_version = %d;
`

type Store struct {
	db      *sql.DB
	writeMu sync.Mutex
	path    string
}

func dbPath() string {
	dir := filepath.Join(os.Getenv("HOME"), ".notebooklm-pp-cli")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "notebooklm.db")
}

func Open() (*Store, error) {
	p := dbPath()
	db, err := sql.Open("sqlite", p)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", p, err)
	}
	s := &Store{db: db, path: p}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Path() string  { return s.path }

func (s *Store) migrate() error {
	var v int
	_ = s.db.QueryRow("PRAGMA user_version").Scan(&v)
	if v == schemaVersion {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(fmt.Sprintf(createSQL, schemaVersion))
	return err
}

type NotebookRow struct {
	ID          string
	Title       string
	SourceCount int
	UpdatedAt   string
	SyncedAt    string
}

func (s *Store) UpsertNotebook(nb NotebookRow) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`INSERT OR REPLACE INTO notebooks (id, title, source_count, updated_at, synced_at)
		VALUES (?, ?, ?, ?, coalesce(?, (SELECT synced_at FROM notebooks WHERE id=?)))`,
		nb.ID, nb.Title, nb.SourceCount, nb.UpdatedAt, nb.SyncedAt, nb.ID)
	return err
}

type SourceRow struct {
	ID         string
	NotebookID string
	Title      string
	Type       string
	URL        string
	SyncedAt   string
}

func (s *Store) UpsertSource(src SourceRow) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`INSERT OR REPLACE INTO sources (id, notebook_id, title, type, url, synced_at)
		VALUES (?, ?, ?, ?, ?, ?)`, src.ID, src.NotebookID, src.Title, src.Type, src.URL, src.SyncedAt)
	return err
}

func (s *Store) UpsertContent(sourceID, notebookID, title, text string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`INSERT OR REPLACE INTO content (source_id, notebook_id, title, text)
		VALUES (?, ?, ?, ?)`, sourceID, notebookID, title, text)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO content_fts (source_id, notebook_id, title, text)
		VALUES (?, ?, ?, ?)`, sourceID, notebookID, title, text)
	if err != nil {
		// FTS insert can fail on duplicate; delete and re-insert
		_, _ = tx.Exec(`DELETE FROM content_fts WHERE source_id = ?`, sourceID)
		_, err = tx.Exec(`INSERT INTO content_fts (source_id, notebook_id, title, text)
			VALUES (?, ?, ?, ?)`, sourceID, notebookID, title, text)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

type SearchResult struct {
	SourceID   string
	NotebookID string
	Title      string
	Snippet    string
}

func (s *Store) Search(query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`SELECT source_id, notebook_id, title, snippet(content_fts, 3, '>>>', '<<<', '...', 32) as snippet
		FROM content_fts WHERE content_fts MATCH ? ORDER BY rank LIMIT ?`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.SourceID, &r.NotebookID, &r.Title, &r.Snippet); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (s *Store) NotebookCount() (int, error) {
	var n int
	err := s.db.QueryRow("SELECT count(*) FROM notebooks").Scan(&n)
	return n, err
}

func (s *Store) SourceCount() (int, error) {
	var n int
	err := s.db.QueryRow("SELECT count(*) FROM sources").Scan(&n)
	return n, err
}
