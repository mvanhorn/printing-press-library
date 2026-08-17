// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

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
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS runs (
			id TEXT PRIMARY KEY,
			group_id TEXT,
			command TEXT NOT NULL,
			prompt TEXT NOT NULL,
			answer TEXT,
			reasoning TEXT,
			model TEXT,
			seed INTEGER,
			params_json JSON,
			raw_json JSON,
			prompt_tokens INTEGER DEFAULT 0,
			completion_tokens INTEGER DEFAULT 0,
			total_tokens INTEGER DEFAULT 0,
			cost_usd REAL DEFAULT 0,
			latency_ms INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS runs_fts USING fts5(
			id UNINDEXED, prompt, answer, reasoning, tokenize='porter unicode61'
		)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_created ON runs(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_group ON runs(group_id)`,
		`CREATE TABLE IF NOT EXISTS vault (
			token TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			kind TEXT NOT NULL,
			value_hash TEXT NOT NULL UNIQUE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			rotated_at DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_vault_kind ON vault(kind)`,
		`CREATE TABLE IF NOT EXISTS audit (
			id TEXT PRIMARY KEY,
			run_id TEXT,
			command TEXT NOT NULL,
			payload_sha256 TEXT NOT NULL,
			byte_count INTEGER NOT NULL,
			model TEXT,
			guard_model TEXT,
			masked_entities INTEGER DEFAULT 0,
			leaked_pii_count INTEGER DEFAULT 0,
			cost_usd REAL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_created ON audit(created_at)`,
		`CREATE TABLE IF NOT EXISTS ladders (
			id TEXT PRIMARY KEY,
			prompt TEXT NOT NULL,
			rungs TEXT NOT NULL,
			first_confident_model TEXT,
			judge_model TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS models_cache (
			id TEXT PRIMARY KEY,
			context_window INTEGER DEFAULT 131072,
			supports_tools BOOLEAN DEFAULT TRUE,
			supports_reasoning BOOLEAN DEFAULT TRUE,
			input_price_per_million REAL DEFAULT 0,
			output_price_per_million REAL DEFAULT 0,
			raw_json JSON,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
