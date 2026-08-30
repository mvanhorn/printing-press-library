// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.

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
		// mail_meta: one row per (account, message) with the headers-first
		// metadata the cleanup engine reasons over. Populated by `sync`,
		// read by `senders` and the novel cleanup commands. label_ids is a
		// JSON array of Gmail label IDs; category is derived from the
		// CATEGORY_* labels (else "primary" when INBOX present, else "");
		// unread mirrors the UNREAD label as 0/1; internal_date is Gmail's
		// internalDate in milliseconds since epoch.
		`CREATE TABLE IF NOT EXISTS mail_meta (
			account TEXT NOT NULL,
			id TEXT NOT NULL,
			thread_id TEXT NOT NULL DEFAULT '',
			from_email TEXT NOT NULL DEFAULT '',
			from_name TEXT NOT NULL DEFAULT '',
			subject TEXT NOT NULL DEFAULT '',
			snippet TEXT NOT NULL DEFAULT '',
			internal_date INTEGER NOT NULL DEFAULT 0,
			size_estimate INTEGER NOT NULL DEFAULT 0,
			label_ids TEXT NOT NULL DEFAULT '[]',
			category TEXT NOT NULL DEFAULT '',
			list_unsubscribe TEXT NOT NULL DEFAULT '',
			list_unsubscribe_post TEXT NOT NULL DEFAULT '',
			unread INTEGER NOT NULL DEFAULT 0,
			auth_results TEXT NOT NULL DEFAULT '',
			list_unsub_domain TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (account, id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_meta_from ON mail_meta(account, from_email)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_meta_date ON mail_meta(account, internal_date)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_meta_category ON mail_meta(account, category)`,
		// mail_sync_state: one row per account holding the Gmail historyId
		// cursor for incremental sync plus bookkeeping timestamps
		// (RFC3339). Named mail_sync_state — NOT sync_state — because the
		// generated store already owns a resource_type-keyed sync_state
		// table for the generic resource sync framework.
		`CREATE TABLE IF NOT EXISTS mail_sync_state (
			account TEXT PRIMARY KEY,
			history_id TEXT NOT NULL DEFAULT '',
			last_full_sync TEXT NOT NULL DEFAULT '',
			last_incremental TEXT NOT NULL DEFAULT ''
		)`,
		// FTS over the searchable mail_meta text columns, following the
		// resources_fts house pattern: standalone (self-contained) fts5
		// table, rows maintained manually alongside every mail_meta write,
		// rowid = ftsRowID(account, id).
		`CREATE VIRTUAL TABLE IF NOT EXISTS mail_meta_fts USING fts5(
			account, id, subject, from_email, from_name, snippet, tokenize='porter unicode61'
		)`,
		// ---- Cleanup engine tables (mailcleanup.go owns all reads/writes) ----
		// mail_nonces: one-time apply-authorization tokens minted by
		// `cleanup plan` / `rules run`. A nonce is bound to exactly one
		// (plan_sha, account) pair, expires ~10 minutes after mint, and is
		// burned atomically with the apply-authorization insert (see
		// Store.AuthorizeMailApply) so a crash between "nonce spent" and
		// "apply recorded" is impossible by construction.
		`CREATE TABLE IF NOT EXISTS mail_nonces (
			nonce TEXT PRIMARY KEY,
			plan_sha TEXT NOT NULL,
			account TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			used INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT ''
		)`,
		// mail_applies: one row per authorized apply attempt.
		// state: authorized -> applying -> done | partial; refusals after
		// authorization land in 'refused'; an authorized row that never got
		// chunks (crash before intent rows) is reconciled to 'abandoned'.
		`CREATE TABLE IF NOT EXISTS mail_applies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			plan_sha TEXT NOT NULL,
			account TEXT NOT NULL,
			action TEXT NOT NULL DEFAULT '',
			state TEXT NOT NULL DEFAULT 'authorized',
			ledger_id TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_applies_state ON mail_applies(state)`,
		// mail_apply_chunks: durable per-chunk mutation intent. Every chunk
		// is inserted as 'pending' BEFORE any network mutation, flipped to
		// 'applying' immediately before its requests go out, and to 'done'
		// after. Recovery re-applies 'applying' label chunks (idempotent)
		// and re-checks 'applying' trash chunks per id.
		`CREATE TABLE IF NOT EXISTS mail_apply_chunks (
			apply_id INTEGER NOT NULL,
			chunk_no INTEGER NOT NULL,
			kind TEXT NOT NULL,
			ids TEXT NOT NULL DEFAULT '[]',
			add_labels TEXT NOT NULL DEFAULT '[]',
			remove_labels TEXT NOT NULL DEFAULT '[]',
			state TEXT NOT NULL DEFAULT 'pending',
			updated_at TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (apply_id, chunk_no)
		)`,
		// mail_ledger + mail_ledger_entries: the delta ledger undo replays.
		// Entries record only the INTRODUCED deltas (grill R1-C6): for trash
		// the +TRASH delta plus the INBOX/CATEGORY_* placement the store
		// showed at plan time (R3-C3); for label ops the exact add/remove
		// lists; for label renames the old/new name pair.
		`CREATE TABLE IF NOT EXISTS mail_ledger (
			ledger_id TEXT PRIMARY KEY,
			account TEXT NOT NULL,
			plan_sha TEXT NOT NULL DEFAULT '',
			apply_id INTEGER NOT NULL DEFAULT 0,
			action TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS mail_ledger_entries (
			ledger_id TEXT NOT NULL,
			id TEXT NOT NULL,
			kind TEXT NOT NULL,
			delta_add TEXT NOT NULL DEFAULT '[]',
			delta_remove TEXT NOT NULL DEFAULT '[]',
			pre_placement TEXT NOT NULL DEFAULT '[]',
			old_name TEXT NOT NULL DEFAULT '',
			new_name TEXT NOT NULL DEFAULT '',
			undone TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (ledger_id, id)
		)`,
		// ---- Unsubscribe engine + novel read-command tables (mailextras.go
		// owns all reads/writes) ----
		// mail_unsub_ledger: one row per unsubscribe POST attempt (or refusal
		// to attempt). status is 'skipped:<reason>' (nothing left this
		// machine), a numeric HTTP status ('200', '302', ...) when a response
		// arrived, or 'unknown' (network error after the connection was
		// established — the POST may or may not have been received; NEVER
		// auto-retried). Rows are inserted 'unknown' BEFORE the POST goes out
		// and updated after, so a crash mid-POST leaves the honest ambiguous
		// record rather than no record.
		`CREATE TABLE IF NOT EXISTS mail_unsub_ledger (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			account TEXT NOT NULL,
			sender TEXT NOT NULL,
			url TEXT NOT NULL DEFAULT '',
			plan_sha TEXT NOT NULL DEFAULT '',
			posted_at TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_unsub_ledger_sender ON mail_unsub_ledger(account, sender)`,
		// mail_checkpoints: per-(account, kind) watermark rows for the delta
		// command family: the max internal_date plus total message count the
		// store held when the checkpoint was last advanced.
		`CREATE TABLE IF NOT EXISTS mail_checkpoints (
			account TEXT NOT NULL,
			kind TEXT NOT NULL,
			watermark_ms INTEGER NOT NULL DEFAULT 0,
			msg_count INTEGER NOT NULL DEFAULT 0,
			taken_at TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (account, kind)
		)`,
		// mail_scores: append-only hygiene-metric snapshots for `score`.
		// metrics is a JSON object of numeric metrics; rows are never updated
		// so delta-vs-previous and delta-vs-first stay reconstructible.
		`CREATE TABLE IF NOT EXISTS mail_scores (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			account TEXT NOT NULL,
			taken_at TEXT NOT NULL DEFAULT '',
			metrics TEXT NOT NULL DEFAULT '{}'
		)`,
		`CREATE INDEX IF NOT EXISTS idx_mail_scores_account ON mail_scores(account, id)`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	// Columns added after mail_meta first shipped. CREATE TABLE IF NOT
	// EXISTS never alters an existing table, so ALTER-if-missing keeps any
	// store created by an earlier binary in step (house backfillColumns
	// pattern). auth_results carries the raw Authentication-Results header
	// for the unsubscribe engine's DKIM-pass verification;
	// list_unsub_domain is reserved for that engine and stays '' here.
	for _, col := range []struct{ name, decl string }{
		{"auth_results", "TEXT NOT NULL DEFAULT ''"},
		{"list_unsub_domain", "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := s.ensureColumn(ctx, conn, "mail_meta", col.name, col.decl); err != nil {
			return err
		}
	}
	return nil
}
