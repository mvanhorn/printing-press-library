// Copyright 2026 darin-kishore. Licensed under Apache-2.0. See LICENSE.

package store

import "context"

// PATCH: Add the local Mobbin mirror schema used by sync/search/sql commands.
var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS apps (
  id TEXT PRIMARY KEY,
  slug TEXT,
  app_name TEXT,
  platform TEXT,
  app_categories TEXT,
  thumbnail_url TEXT,
  latest_version_id TEXT,
  created_at TEXT,
  updated_at TEXT,
  raw_json TEXT,
  synced_at TEXT
);`,
	`CREATE INDEX IF NOT EXISTS idx_apps_platform ON apps(platform);`,
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
	`CREATE VIRTUAL TABLE IF NOT EXISTS apps_fts USING fts5(slug, app_name, platform, app_categories, content="apps", content_rowid="rowid");`,
	`CREATE VIRTUAL TABLE IF NOT EXISTS screens_fts USING fts5(image_url, ocr_text, app_id, content="screens", content_rowid="rowid");`,
	`CREATE VIRTUAL TABLE IF NOT EXISTS flows_fts USING fts5(name, flow_actions, app_id, content="flows", content_rowid="rowid");`,
}

func (db *DB) migrate(ctx context.Context) error {
	for _, stmt := range schemaStatements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
