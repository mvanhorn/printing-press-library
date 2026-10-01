// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LibrarySchemaVersion is the on-disk schema version of the library DB, which
// holds the D2C content-production tables (generations, brand_profiles,
// briefs, tags, tag_links, platform_targets). It lives in its own SQLite file,
// so its PRAGMA user_version never collides with the generated sync store.
const LibrarySchemaVersion = 1

// libraryMigrations is the ordered DDL for the library DB. All statements are
// idempotent (CREATE ... IF NOT EXISTS) so re-running the migration on an
// already-initialized library.db is a no-op.
//
// Indexes are created from day 1 because the library is read-heavy: list and
// cost-report queries filter by brand, platform, model, and cost, and the
// composite (filter_col, created_at DESC) shape lets SQLite satisfy both the
// WHERE and the ORDER BY from a single index. FTS5 over prompt powers
// `library search`.
var libraryMigrations = []string{
	`CREATE TABLE IF NOT EXISTS generations (
		id TEXT PRIMARY KEY,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		command TEXT,
		brand_profile_id TEXT,
		brand_name TEXT,
		platform_target TEXT,
		model_id TEXT,
		prompt TEXT,
		aspect_ratio TEXT,
		seed INTEGER,
		cost REAL,
		content_hash TEXT,
		path TEXT,
		status TEXT,
		params JSON,
		data JSON
	)`,
	`CREATE INDEX IF NOT EXISTS idx_generations_brand_created ON generations(brand_profile_id, created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_generations_platform_created ON generations(platform_target, created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_generations_model_created ON generations(model_id, created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_generations_cost ON generations(cost)`,
	`CREATE INDEX IF NOT EXISTS idx_generations_content_hash ON generations(content_hash)`,
	`CREATE VIRTUAL TABLE IF NOT EXISTS generations_fts USING fts5(
		prompt,
		content='generations',
		content_rowid='rowid',
		tokenize='porter unicode61'
	)`,
	`CREATE TRIGGER IF NOT EXISTS generations_ai AFTER INSERT ON generations BEGIN
		INSERT INTO generations_fts(rowid, prompt) VALUES (new.rowid, new.prompt);
	END`,
	`CREATE TRIGGER IF NOT EXISTS generations_ad AFTER DELETE ON generations BEGIN
		INSERT INTO generations_fts(generations_fts, rowid, prompt) VALUES ('delete', old.rowid, old.prompt);
	END`,
	`CREATE TRIGGER IF NOT EXISTS generations_au AFTER UPDATE ON generations BEGIN
		INSERT INTO generations_fts(generations_fts, rowid, prompt) VALUES ('delete', old.rowid, old.prompt);
		INSERT INTO generations_fts(rowid, prompt) VALUES (new.rowid, new.prompt);
	END`,
	`CREATE TABLE IF NOT EXISTS brand_profiles (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		data JSON NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS briefs (
		id TEXT PRIMARY KEY,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		source TEXT,
		data JSON NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS tags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE
	)`,
	`CREATE TABLE IF NOT EXISTS tag_links (
		generation_id TEXT NOT NULL,
		tag_id INTEGER NOT NULL,
		PRIMARY KEY (generation_id, tag_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_tag_links_tag ON tag_links(tag_id)`,
	`CREATE TABLE IF NOT EXISTS platform_targets (
		generation_id TEXT NOT NULL,
		platform TEXT NOT NULL,
		format TEXT,
		manifest_path TEXT,
		PRIMARY KEY (generation_id, platform)
	)`,
}

// OpenLibrary opens or creates the library SQLite store at dbPath using the
// background context.
func OpenLibrary(dbPath string) (*Store, error) {
	return OpenLibraryWithContext(context.Background(), dbPath)
}

// OpenLibraryWithContext opens or creates the library store. It builds the
// Store directly and runs migrateLibrary instead of the generated sync-store
// migrations, so library.db never accumulates resources/sync_state tables.
func OpenLibraryWithContext(ctx context.Context, dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}
	hardenSQLiteFiles(dbPath)
	defer hardenSQLiteFiles(dbPath)
	dsn := dbPath + "?_txlock=immediate&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=temp_store(MEMORY)&_pragma=mmap_size(0)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening library database: %w", err)
	}
	db.SetMaxOpenConns(2)
	s := &Store{db: db, path: dbPath}
	if err := s.migrateLibrary(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("running library migrations: %w", err)
	}
	return s, nil
}

// migrateLibrary applies libraryMigrations under the same busy-retry and
// BEGIN IMMEDIATE discipline as the generated migrate().
func (s *Store) migrateLibrary(ctx context.Context) error {
	deadline := time.Now().Add(migrationLockTimeout)
	var conn *sql.Conn
	if err := retryOnBusy(ctx, deadline, "acquiring library migration connection", func() error {
		c, err := s.db.Conn(ctx)
		if err != nil {
			return err
		}
		conn = c
		return nil
	}); err != nil {
		return err
	}
	defer conn.Close()

	var current int
	if err := retryOnBusy(ctx, deadline, "reading library schema version", func() error {
		return conn.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&current)
	}); err != nil {
		return err
	}
	if current > LibrarySchemaVersion {
		return fmt.Errorf("library database schema version %d is newer than supported version %d; upgrade the CLI binary", current, LibrarySchemaVersion)
	}
	return withMigrationLock(ctx, conn, deadline, func() error {
		for _, m := range libraryMigrations {
			if _, err := conn.ExecContext(ctx, m); err != nil {
				return fmt.Errorf("library migration failed: %w", err)
			}
		}
		if _, err := conn.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, LibrarySchemaVersion)); err != nil {
			return fmt.Errorf("stamp library user_version: %w", err)
		}
		return nil
	})
}
