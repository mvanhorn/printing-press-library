// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written schema for the novel (hand-authored) command family:
// duplicates, license audit, history diff, similar/log fail, job
// download/resume, feed/follow, snapshot create/verify, library doctor,
// formats gaps, and designer stats. Deliberately kept in its own file and
// its own migration function, separate from printgoat_migrations.go (owned
// by a parallel task implementing search/download/files), so the two
// efforts never collide on the same file or the same CREATE TABLE
// statements. Safe to call EnsurePrintgoatNovelSchema repeatedly (every
// statement is IF NOT EXISTS).
package store

import "database/sql"

// EnsurePrintgoatNovelSchema creates the tables used by the novel command
// family if they do not already exist. Call it after opening the store and
// before any read/write against these tables — the underlying SQLite file
// may have been created by an older binary, or by a run of the primary
// search/download commands, that never created these tables.
func EnsurePrintgoatNovelSchema(db *sql.DB) error {
	statements := []string{
		// printgoat_model_snapshots: point-in-time capture of a model's
		// remote state, used by `history diff` to report what changed
		// since the last look (files count, rating, likes, license, ...).
		`CREATE TABLE IF NOT EXISTS printgoat_model_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source TEXT NOT NULL,
			model_id TEXT NOT NULL,
			snapshot_json TEXT NOT NULL,
			snapshotted_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_printgoat_model_snapshots_lookup
			ON printgoat_model_snapshots(source, model_id, snapshotted_at)`,

		// printgoat_print_outcomes: user-logged pass/fail history per
		// model+designer, written by `log fail` and read by `designer stats`.
		`CREATE TABLE IF NOT EXISTS printgoat_print_outcomes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source TEXT NOT NULL,
			model_id TEXT NOT NULL,
			designer TEXT,
			outcome TEXT NOT NULL,
			reason TEXT,
			logged_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_printgoat_print_outcomes_designer
			ON printgoat_print_outcomes(designer)`,

		// printgoat_print_jobs / printgoat_print_job_files: crash-safe
		// batch download bookkeeping for `job download` / `job resume`.
		`CREATE TABLE IF NOT EXISTS printgoat_print_jobs (
			id TEXT PRIMARY KEY,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS printgoat_print_job_files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id TEXT NOT NULL,
			source TEXT NOT NULL,
			model_id TEXT NOT NULL,
			file_name TEXT NOT NULL,
			file_url TEXT,
			local_path TEXT,
			total_bytes INTEGER,
			downloaded_bytes INTEGER DEFAULT 0,
			status TEXT DEFAULT 'pending',
			UNIQUE(job_id, source, model_id, file_name)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_printgoat_print_job_files_job
			ON printgoat_print_job_files(job_id)`,

		// printgoat_designer_links / printgoat_feed_seen: `follow designer`
		// bookkeeping and `feed` new-since-last-run tracking.
		`CREATE TABLE IF NOT EXISTS printgoat_designer_links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			alias_group TEXT,
			source TEXT NOT NULL,
			handle TEXT NOT NULL,
			UNIQUE(source, handle)
		)`,
		`CREATE TABLE IF NOT EXISTS printgoat_feed_seen (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source TEXT NOT NULL,
			model_id TEXT NOT NULL,
			seen_at TEXT NOT NULL,
			UNIQUE(source, model_id)
		)`,

		// printgoat_job_snapshots: pinned file hashes for `snapshot
		// create`/`snapshot verify`, so a past print job's exact file
		// versions can be proven later even if upstream has changed.
		`CREATE TABLE IF NOT EXISTS printgoat_job_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			files_json TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}
