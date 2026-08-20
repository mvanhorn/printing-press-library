// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// migrateExtras runs after the generated store migrations and before the
// schema-version stamp on the ARCHIVE database. It is the canonical place for
// novel-feature auxiliary tables that belong alongside the sync-cache.
//
// The D2C content-production tables live in a physically separate library DB
// (see migrateLibrary), not here, so this hook is intentionally empty: adding
// library tables to the archive DB would mix two independent version domains.
//
// Edit this file when adding tables for novel commands. Keep migrations
// idempotent with CREATE TABLE IF NOT EXISTS / CREATE INDEX IF NOT EXISTS so
// every store open can safely re-run them.
func (s *Store) migrateExtras(ctx context.Context, conn *sql.Conn) error {
	migrations := []string{
		// Archive-side auxiliary tables go here. Library tables live in
		// migrateLibrary against the separate library DB.
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}

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

// migrateLibrary creates the library DB schema under the same connection,
// version-gate, and BEGIN IMMEDIATE lock discipline as the archive migrate().
// The library DB has its own version domain stamped via PRAGMA user_version
// (separate file ⇒ separate user_version), gated by LibrarySchemaVersion.
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
		var current int
		if err := conn.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&current); err != nil {
			return fmt.Errorf("reading library schema version: %w", err)
		}
		if current > LibrarySchemaVersion {
			return fmt.Errorf("library database schema version %d is newer than supported version %d; upgrade the CLI binary", current, LibrarySchemaVersion)
		}
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
