// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import "database/sql"

// printgoatSchemaStatements are the CREATE TABLE IF NOT EXISTS statements for
// printgoat's own local tables: the download ledger and the local
// download-preference key/value store. Kept as a plain []string slice
// literal (rather than folded into a single multi-statement string or into
// Store.migrate's versioned schema history) specifically so a follow-up
// novel-features migration can append more statements to this same slice
// without restructuring EnsurePrintgoatSchema or touching the generic
// resource-store schema versioning in store.go.
var printgoatSchemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS printgoat_downloads (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source TEXT NOT NULL,
		model_id TEXT NOT NULL,
		model_name TEXT,
		file_name TEXT NOT NULL,
		file_url TEXT,
		local_path TEXT NOT NULL,
		file_size INTEGER,
		bytes_downloaded INTEGER DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'complete',
		sha256 TEXT,
		downloaded_at TEXT NOT NULL,
		UNIQUE(source, model_id, file_name)
	)`,
	`CREATE TABLE IF NOT EXISTS printgoat_config_kv (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`,
}

// EnsurePrintgoatSchema creates printgoat's local tables if they do not
// already exist. It is idempotent and cheap (CREATE TABLE IF NOT EXISTS),
// so callers invoke it lazily — once per command invocation, right before
// the first read or write against these tables — rather than eagerly at
// every store open. Safe to call on a *sql.DB obtained from Store.DB().
func EnsurePrintgoatSchema(db *sql.DB) error {
	for _, stmt := range printgoatSchemaStatements {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}
