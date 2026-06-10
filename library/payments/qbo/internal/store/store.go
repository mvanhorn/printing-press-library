// Copyright 2026 Martin Kessler and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db   *sql.DB
	path string
}

func Open() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting user home dir: %w", err)
	}

	dbDir := filepath.Join(home, ".cache", "qbo-pp-cli")
	dbPath := filepath.Join(dbDir, "qbo.db")
	return OpenWithPath(dbPath)
}

func OpenWithPath(dbPath string) (*Store, error) {
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0o700); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite db: %w", err)
	}

	// Enable WAL mode for concurrent execution safety
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}

	s := &Store{db: db, path: dbPath}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) initSchema() error {
	tables := []string{
		"customers",
		"vendors",
		"accounts",
		"invoices",
		"payments",
		"bills",
		"purchases",
		"journal_entries",
	}

	for _, table := range tables {
		query := fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id TEXT PRIMARY KEY,
				name TEXT,
				doc_number TEXT,
				last_updated TEXT,
				raw_json TEXT
			);
		`, table)
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("creating table %s: %w", table, err)
		}

		indexQuery := fmt.Sprintf(`
			CREATE INDEX IF NOT EXISTS idx_%s_last_updated ON %s (last_updated);
		`, table, table)
		if _, err := s.db.Exec(indexQuery); err != nil {
			return fmt.Errorf("creating index idx_%s_last_updated: %w", table, err)
		}

		// Migrate pre-existing timezone-offset timestamps to UTC format
		migrationQuery := fmt.Sprintf(`
			UPDATE %s 
			SET last_updated = strftime('%%Y-%%m-%%dT%%H:%%M:%%SZ', last_updated, 'utc')
			WHERE last_updated NOT LIKE '%%Z' AND last_updated IS NOT NULL AND last_updated != '';
		`, table)
		if _, err := s.db.Exec(migrationQuery); err != nil {
			return fmt.Errorf("migrating timestamps for table %s: %w", table, err)
		}
	}

	// Sync state table to keep track of incremental changes
	syncStateQuery := `
		CREATE TABLE IF NOT EXISTS sync_state (
			key TEXT PRIMARY KEY,
			value TEXT
		);
	`
	if _, err := s.db.Exec(syncStateQuery); err != nil {
		return fmt.Errorf("creating sync_state table: %w", err)
	}

	return nil
}

func (s *Store) UpsertEntity(table string, id string, name string, docNum string, lastUpdated string, rawJSON string) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (id, name, doc_number, last_updated, raw_json)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			doc_number = excluded.doc_number,
			last_updated = excluded.last_updated,
			raw_json = excluded.raw_json
	`, table)
	_, err := s.db.Exec(query, id, name, docNum, lastUpdated, rawJSON)
	return err
}

func (s *Store) GetLastSyncTime() (time.Time, error) {
	var val string
	err := s.db.QueryRow("SELECT value FROM sync_state WHERE key = 'last_sync_time'").Scan(&val)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, val)
}

func (s *Store) SetLastSyncTime(t time.Time) error {
	val := t.Format(time.RFC3339)
	_, err := s.db.Exec(`
		INSERT INTO sync_state (key, value)
		VALUES ('last_sync_time', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, val)
	return err
}
