// Copyright 2026 Angelo Pullen and contributors. Licensed under Apache-2.0. See LICENSE.

// Obsidian-specific schema. The generic schema produced by the Printing
// Press models a SaaS REST API (resources table keyed by id+resource_type
// with JSON blobs); the obsidian-pp-cli mirror needs a richer relational
// shape so the Tier-3 commands (health, broken, stale, decay, hotspots,
// reconcile, sql) can run sub-100 ms even on large vaults.
//
// EnsureObsidianSchema is idempotent and runs from `obsidian-pp-cli sync`
// before the first write. Tables live alongside the generic store tables
// — the generic resources table stays empty for obsidian (no API to walk)
// and is harmless.

package store

import "fmt"

// EnsureObsidianSchema creates the obsidian-mirror tables and indexes if
// they do not exist. Safe to call repeatedly; uses the store's writeMu
// to serialize against concurrent UpsertBatch on the generic tables.
func (s *Store) EnsureObsidianSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS notes (
			id               INTEGER PRIMARY KEY,
			path             TEXT UNIQUE NOT NULL,
			title            TEXT,
			created_at       TEXT,
			modified_at      TEXT,
			word_count       INTEGER,
			content_hash     TEXT,
			frontmatter_json TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS obsidian_tags (
			note_id INTEGER NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
			tag     TEXT    NOT NULL,
			PRIMARY KEY (note_id, tag)
		)`,
		`CREATE TABLE IF NOT EXISTS obsidian_links (
			source_id   INTEGER NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
			target_path TEXT    NOT NULL,
			link_type   TEXT    NOT NULL CHECK(link_type IN ('wikilink','embed','external')),
			resolved    INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS frontmatter_kv (
			note_id INTEGER NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
			key     TEXT    NOT NULL,
			value   TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS vault_sync_state (
			id            INTEGER PRIMARY KEY CHECK (id=1),
			vault_path    TEXT    NOT NULL,
			last_sync_at  TEXT,
			notes_synced  INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_notes_modified ON notes(modified_at)`,
		`CREATE INDEX IF NOT EXISTS idx_obsidian_links_target ON obsidian_links(target_path)`,
		`CREATE INDEX IF NOT EXISTS idx_obsidian_tags_tag ON obsidian_tags(tag)`,
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("ensuring obsidian schema: %s: %w", q[:40], err)
		}
	}
	return nil
}
