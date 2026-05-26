// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// Hand-coded extension migrations for GHL-specific tables. Kept separate
// from the generator-emitted migrations slice in store.go so a regen of
// the press scaffolding does not blow these away. Callers invoke
// GHLExtensionMigrations() lazily after Open*ContextWith — see
// internal/cli/opp.go for the canonical apply pattern.
package store

// GHLExtensionMigrations returns hand-coded migrations for GHL-specific
// tables (pipelines/stages name maps + stage-transition history). Apply
// these against db.DB() after OpenWithContext. They are idempotent
// (CREATE TABLE IF NOT EXISTS / CREATE INDEX IF NOT EXISTS).
func GHLExtensionMigrations() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS stage_transitions (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            opportunity_id TEXT NOT NULL,
            from_stage_id TEXT,
            to_stage_id TEXT NOT NULL,
            transitioned_at TEXT NOT NULL,
            observed_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
        )`,
		`CREATE INDEX IF NOT EXISTS idx_stage_transitions_opp ON stage_transitions(opportunity_id, transitioned_at)`,
		`CREATE INDEX IF NOT EXISTS idx_stage_transitions_to ON stage_transitions(to_stage_id, transitioned_at)`,
		`CREATE TABLE IF NOT EXISTS pipelines (
            id TEXT PRIMARY KEY,
            name TEXT NOT NULL,
            location_id TEXT,
            data TEXT NOT NULL
        )`,
		`CREATE TABLE IF NOT EXISTS stages (
            id TEXT PRIMARY KEY,
            pipeline_id TEXT NOT NULL,
            name TEXT NOT NULL,
            position INTEGER,
            FOREIGN KEY (pipeline_id) REFERENCES pipelines(id)
        )`,
	}
}
