// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
)

// migrateExtras runs after the generated store migrations and before the
// schema-version stamp. It is the canonical place for novel-feature auxiliary
// tables that need to live in the local store.
//
// Edit this file when adding tables for novel commands. Keep migrations
// idempotent with CREATE TABLE IF NOT EXISTS / CREATE INDEX IF NOT EXISTS so
// every store open can safely re-run them.
func (s *Store) migrateExtras(ctx context.Context, conn *sql.Conn) error {
	// The generated `apps` table lacks the Mobbin slug column the novel
	// commands (app, drift) filter on; add it idempotently.
	if err := s.ensureColumn(ctx, conn, "apps", "slug", "TEXT"); err != nil {
		return fmt.Errorf("ensuring apps.slug: %w", err)
	}
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS app_versions (
  id TEXT PRIMARY KEY,
  app_id TEXT,
  version TEXT,
  captured_at TEXT,
  raw_json TEXT,
  synced_at TEXT
);`,
		`CREATE INDEX IF NOT EXISTS idx_app_versions_app_id ON app_versions(app_id);`,
		`CREATE TABLE IF NOT EXISTS screens (
  id TEXT PRIMARY KEY,
  app_id TEXT,
  app_version_id TEXT,
  flow_id TEXT,
  platform TEXT,
  image_url TEXT,
  image_url_full TEXT,
  ocr_text TEXT,
  raw_json TEXT,
  captured_at TEXT,
  synced_at TEXT
);`,
		`CREATE INDEX IF NOT EXISTS idx_screens_app_id ON screens(app_id);`,
		`CREATE INDEX IF NOT EXISTS idx_screens_flow_id ON screens(flow_id);`,
		`CREATE TABLE IF NOT EXISTS flows (
  id TEXT PRIMARY KEY,
  app_id TEXT,
  app_version_id TEXT,
  name TEXT,
  flow_actions TEXT,
  screen_ids TEXT,
  step_count INTEGER,
  platform TEXT,
  raw_json TEXT,
  captured_at TEXT,
  synced_at TEXT
);`,
		`CREATE INDEX IF NOT EXISTS idx_flows_app_id ON flows(app_id);`,
		`CREATE TABLE IF NOT EXISTS patterns (
  id TEXT PRIMARY KEY,
  slug TEXT,
  name TEXT,
  category TEXT,
  definition TEXT,
  platform TEXT,
  raw_json TEXT,
  synced_at TEXT
);`,
		`CREATE TABLE IF NOT EXISTS elements (
  id TEXT PRIMARY KEY,
  slug TEXT,
  name TEXT,
  category TEXT,
  definition TEXT,
  platform TEXT,
  raw_json TEXT,
  synced_at TEXT
);`,
		`CREATE TABLE IF NOT EXISTS flow_actions (
  id TEXT PRIMARY KEY,
  slug TEXT,
  name TEXT,
  category TEXT,
  definition TEXT,
  platform TEXT,
  raw_json TEXT,
  synced_at TEXT
);`,
		`CREATE TABLE IF NOT EXISTS screen_patterns (
  screen_id TEXT,
  pattern_slug TEXT,
  PRIMARY KEY(screen_id, pattern_slug)
);`,
		`CREATE TABLE IF NOT EXISTS screen_elements (
  screen_id TEXT,
  element_slug TEXT,
  PRIMARY KEY(screen_id, element_slug)
);`,
		`CREATE TABLE IF NOT EXISTS collections (
  id TEXT PRIMARY KEY,
  workspace_id TEXT,
  name TEXT,
  description TEXT,
  created_at TEXT,
  raw_json TEXT,
  synced_at TEXT
);`,
		`CREATE TABLE IF NOT EXISTS content_meta (
  key TEXT PRIMARY KEY,
  value TEXT,
  updated_at TEXT
);`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
