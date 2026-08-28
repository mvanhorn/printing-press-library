// Licensed under Apache-2.0. See LICENSE.

// Package store provides local SQLite persistence for bing-ads-pp-cli.
// Uses modernc.org/sqlite (pure Go, no CGO) for zero-dependency cross-compilation.
// FTS5 full-text search indexes are created for searchable content.
package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var isoDatePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}(?:[T ][0-9:.+-Zz]+)?$`)
var ftsQueryTokenRE = regexp.MustCompile(`[\pL\pN_]+`)

var sqliteDriverInit struct {
	mu   sync.Mutex
	done bool
}

// validIdentifierRE pins ListField's `field` argument to a safe SQL
// identifier shape before any Sprintf interpolation. Matches what
// pragma_table_info implicitly enforces on the primary path, so the
// fallback path inherits the same defense without depending on whether
// the parent's typed domain table exists at the moment of the lookup.
var validIdentifierRE = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// IsUUID returns true if the input looks like a UUID.
func IsUUID(s string) bool {
	return uuidPattern.MatchString(s)
}

// StoreSchemaVersion is the on-disk schema version this binary understands.
// It is stamped into SQLite's PRAGMA user_version on fresh databases and
// checked on every open. Learn-enabled CLIs advance to v9 for the
// learn_candidates and learn_events tables (CLI-side capture and
// measurement), on top of the v8 learning_playbooks table for
// hand-authored choreography keyed by query family and the v6 canonical
// learn-loop tables ported from prediction-goat (including the v3
// resources_fts rowid rehash and v4 resources_fts content extraction).
const StoreSchemaVersion = 9

// resourcesFTSContentSchemaVersion pins the schema bump that rewrote
// resources_fts content from raw JSON to searchable leaf values. Keep this
// separate from StoreSchemaVersion — and pinned at 4 regardless of the
// learn shape — so schema bumps that only add tables (the learn
// migrations) never trigger an expensive full FTS content rewrite. A
// store stamped at v4 or later already carries the extracted-leaf FTS
// content; opening it with a newer binary must stay additive-only.
const resourcesFTSContentSchemaVersion = 4

const resourcesFTSCreateSQL = `CREATE VIRTUAL TABLE IF NOT EXISTS resources_fts USING fts5(
	id, resource_type, content, tokenize='porter unicode61'
)`

type Store struct {
	db *sql.DB
	// writeMu serializes all DB writes. Read paths bypass the lock and run
	// concurrently against WAL. Resource-level concurrency in sync.go.tmpl
	// is 1 (one goroutine per resource via len(resources)-sized work channel)
	// — read-then-write sequences (e.g., GetSyncCursor → SaveSyncState) are
	// race-free by construction within a resource.
	writeMu sync.Mutex
	path    string
}

// Open opens or creates the SQLite store at dbPath using the background
// context. Prefer OpenWithContext from a Cobra command so SIGINT during
// a slow migration interrupts the open instead of stranding the caller.
func Open(dbPath string) (*Store, error) {
	return OpenWithContext(context.Background(), dbPath)
}

// OpenReadOnly opens an existing SQLite store at dbPath in read-only mode.
// mode=ro rejects direct and CTE-wrapped writes (INSERT, UPDATE, DELETE,
// REPLACE, "WITH x AS (...) INSERT ...") at the driver level. Skips
// MkdirAll and migrate; the file is expected to exist.
//
// The file: URI prefix is load-bearing: modernc.org/sqlite only honors
// SQLite's URI query parameters (mode, cache, etc.) when the DSN starts
// with "file:". Without the prefix, "?mode=ro" is silently dropped and
// the connection opens read-write. Pragmas use the driver's _pragma=
// name(value) syntax — modernc.org/sqlite does NOT recognize the
// mattn/go-sqlite3 _journal_mode=WAL / _busy_timeout=5000 form and drops
// those keys silently, so the busy_timeout below is what keeps a read
// concurrent with a writer from failing immediately with SQLITE_BUSY.
//
// Deliberately no journal_mode pragma here: journal mode is a property of
// the database file, set by the read-write open, not the connection. Issuing
// PRAGMA journal_mode=WAL on a read-only handle to a DB still in the default
// delete mode (e.g. a pre-WAL database opened by an old binary before its
// first read-write open) errors with "attempt to write a readonly database".
//
// immutable=1 is the WAL-index control. mmap_size(0) only bounds mmap of the
// main database file; SQLite still memory-maps the -shm WAL-index for
// multi-process WAL coordination, and concurrent read-only processes fault
// inside that mapping. The URI flag tells SQLite this connection will not
// observe writers, so it skips shared-memory and reads the main file with
// pread. A WAL writer's last close already checkpoints, so a later
// immutable reader sees the committed snapshot. Uncheckpointed frames from
// a still-open writer are invisible; that is the trade for not mapping -shm.
// nolock=1 and vfs=unix-none cannot open a WAL database; exclusive locking
// mode serializes clients and fails a mode=ro open.
//
// OpenReadOnly uses context.Background(); callers holding a context should use
// OpenReadOnlyContext so a cancelled command (SIGINT, deadline) interrupts the
// SQLITE_BUSY retry during driver init instead of waiting out the full timeout.
func OpenReadOnly(dbPath string) (*Store, error) {
	return OpenReadOnlyContext(context.Background(), dbPath)
}

// OpenReadOnlyContext is OpenReadOnly with a caller-supplied context honored by
// the driver-init SQLITE_BUSY retry.
func OpenReadOnlyContext(ctx context.Context, dbPath string) (*Store, error) {
	dsn := "file:" + dbPath + "?mode=ro&immutable=1&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_pragma=temp_store(MEMORY)&_pragma=mmap_size(0)"
	if err := ensureSQLiteDriverInitialized(ctx, dsn); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database (read-only): %w", err)
	}
	db.SetMaxOpenConns(2)
	return &Store{db: db, path: dbPath}, nil
}

// OpenWithContext opens or creates the SQLite store at dbPath. The
// context is honored by the migration path: cancellation interrupts the
// retry-on-SQLITE_BUSY loop and propagates ctx.Err() back to the caller
// instead of waiting out the full migrationLockTimeout.
func OpenWithContext(ctx context.Context, dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}
	hardenSQLiteFiles(dbPath)
	defer hardenSQLiteFiles(dbPath)
	if err := rejectNewerSchemaBeforeJournalMode(ctx, dbPath); err != nil {
		return nil, err
	}

	// Pragma order is load-bearing: busy_timeout must engage BEFORE the
	// journal-mode conversion so concurrent first-run opens wait instead of
	// racing the exclusive conversion. Cache-enabled profiles write local
	// state during reads, so they use a rollback journal and avoid WAL sidecar
	// teardown races between short-lived processes. Other store profiles keep
	// WAL for concurrent analytical reads.
	dsn := dbPath + "?_txlock=immediate&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=temp_store(MEMORY)&_pragma=mmap_size(0)"
	if err := ensureSQLiteDriverInitialized(ctx, dsn); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Two connections allow one read cursor to remain open while a second query
	// executes (e.g., analytics commands calling helpers during row iteration).
	db.SetMaxOpenConns(2)

	s := &Store{db: db, path: dbPath}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return s, nil
}

// A newer schema must be rejected before a read-write connection can attempt
// journal-mode conversion. This lightweight read-only probe can read a
// committed PRAGMA user_version while a peer writer is active; other probe
// errors remain with the normal open/migration path, which returns the more
// precise corruption, permission, or lock error.
func rejectNewerSchemaBeforeJournalMode(ctx context.Context, dbPath string) error {
	info, err := os.Stat(dbPath)
	if os.IsNotExist(err) || (err == nil && info.Size() == 0) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stating database for schema preflight: %w", err)
	}
	probe, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?mode=ro&immutable=1&_pragma=busy_timeout(1000)&_pragma=mmap_size(0)")
	if err != nil {
		return nil
	}
	defer probe.Close()
	probe.SetMaxOpenConns(1)

	var current int
	if err := probe.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&current); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return nil
	}
	if current > StoreSchemaVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d; upgrade the CLI binary or open an older database", current, StoreSchemaVersion)
	}
	return nil
}

// hardenSQLiteFiles is best-effort so stores on filesystems without Unix modes
// remain usable. The deferred call catches files the SQLite driver creates.
func hardenSQLiteFiles(dbPath string) {
	for _, path := range []string{dbPath, dbPath + "-journal", dbPath + "-wal", dbPath + "-shm"} {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}

		file, err := os.Open(path)
		if err != nil {
			continue
		}
		openInfo, statErr := file.Stat()
		pathInfo, lstatErr := os.Lstat(path)
		if statErr == nil && lstatErr == nil && pathInfo.Mode().IsRegular() && os.SameFile(openInfo, pathInfo) {
			_ = file.Chmod(0o600)
		}
		_ = file.Close()
	}
}

// ensureSQLiteJournalPrivate creates the cache-profile rollback journal before
// SQLite starts a write transaction, so its mode is private for the whole
// transaction rather than only after the journal has been created.
func ensureSQLiteJournalPrivate(dbPath string) {
	journalPath := dbPath + "-journal"
	file, err := os.OpenFile(journalPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err == nil {
		_ = file.Close()
		return
	}
	if os.IsExist(err) {
		hardenSQLiteFiles(dbPath)
	}
}

// lockForWrite and unlockAfterWrite keep SQLite sidecars private across the
// lifetime of every serialized writer. TRUNCATE journaling reuses its journal
// file and can restore its mode when a later transaction starts, after the
// one-time OpenWithContext hardening has already run.
func (s *Store) lockForWrite() {
	s.writeMu.Lock()
	hardenSQLiteFiles(s.path)
}

func (s *Store) unlockAfterWrite() {
	hardenSQLiteFiles(s.path)
	s.writeMu.Unlock()
}

func ensureSQLiteDriverInitialized(ctx context.Context, dsn string) error {
	sqliteDriverInit.mu.Lock()
	defer sqliteDriverInit.mu.Unlock()

	if sqliteDriverInit.done {
		return nil
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("opening database for driver initialization: %w", err)
	}
	defer db.Close()

	// Acquiring the first physical connection runs the DSN _pragma directives,
	// including the journal-mode conversion for a read-write DSN. On a
	// fresh DB opened concurrently — e.g. the scorecard live-check probing
	// sampled commands in parallel — that conversion can return SQLITE_BUSY
	// before the DSN's busy_timeout engages, so retry the acquisition against a
	// bounded deadline. SQLITE_BUSY here is always transient.
	deadline := time.Now().Add(migrationLockTimeout)
	var conn *sql.Conn
	if err := retryOnBusy(ctx, deadline, "initializing sqlite driver", func() error {
		c, err := db.Conn(ctx)
		if err != nil {
			return err
		}
		conn = c
		return nil
	}); err != nil {
		return err
	}
	if err := conn.Close(); err != nil {
		return fmt.Errorf("closing sqlite initialization connection: %w", err)
	}

	sqliteDriverInit.done = true
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// Path returns the on-disk path of the backing SQLite file.
func (s *Store) Path() string {
	return s.path
}

// DB exposes the underlying *sql.DB for callers that need to run ad-hoc
// queries (e.g., doctor's cache inspection, share snapshot import).
// Callers must not call Close on the returned handle.
func (s *Store) DB() *sql.DB {
	return s.db
}

// SchemaVersion reads PRAGMA user_version, which is stamped by migrate().
// A zero value means the database predates the schema-version gate — not
// a bug, but the caller may want to warn.
func (s *Store) SchemaVersion() (int, error) {
	var v int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		return 0, fmt.Errorf("read user_version: %w", err)
	}
	return v, nil
}

// ensureColumn adds a column to an existing table if it isn't already
// present. It is the upgrade-path safety valve for schema additions:
// CREATE TABLE IF NOT EXISTS is a no-op when the table already exists, so
// columns added by newer binaries (e.g. parent_id from the dependent-
// resources work) never land on databases created by older binaries —
// which then trip "no such column" when a follow-on CREATE INDEX runs.
//
// Skips silently if the table doesn't yet exist (fresh install — the
// CREATE TABLE migration will create it with the column already declared)
// or if the column already exists. Runs on the pinned migration
// connection so it sees the writes performed by the in-flight BEGIN
// IMMEDIATE transaction; using s.db here would route through the pool
// and BUSY against the holding writer under concurrent migrators.
func (s *Store) ensureColumn(ctx context.Context, conn *sql.Conn, table, column, decl string) error {
	var name string
	err := conn.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
	).Scan(&name)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("checking table %s: %w", table, err)
	}

	rows, err := conn.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info("%s")`, table))
	if err != nil {
		return fmt.Errorf("table_info %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var n, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &n, &typ, &notnull, &dflt, &pk); err != nil {
			return fmt.Errorf("scan table_info %s: %w", table, err)
		}
		if n == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating table_info %s: %w", table, err)
	}

	if _, err := conn.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN "%s" %s`, table, column, decl)); err != nil {
		// A concurrent Open() may have added the column between our
		// PRAGMA check and this ALTER. SQLite returns SQLITE_ERROR with
		// "duplicate column name", which busy_timeout does not retry.
		// The DB is now in the desired state regardless of who won.
		if strings.Contains(err.Error(), "duplicate column name") {
			return nil
		}
		return fmt.Errorf("add column %s.%s: %w", table, column, err)
	}
	return nil
}

// backfillColumns adds columns that newer binaries declare but that
// pre-existing databases (created before those columns were added) lack.
// Must run before the migrations slice so that subsequent CREATE INDEX
// statements referencing the column can succeed against the upgraded
// table. Idempotent: safe to call on fresh DBs (table-not-found short-
// circuit) and on already-current DBs (column-exists short-circuit).
//
// Table names are emitted bare (no safeName) — ensureColumn double-quotes
// them at SQL emit time and uses parameter binding for the sqlite_master
// lookup, so the values flow as Go string literals first and SQL
// identifiers second. Wrapping with safeName here would embed literal
// double-quote characters into the Go string and break compilation for
// any spec whose dependent-resource snake_cased name is a SQL reserved
// word.
func (s *Store) backfillColumns(ctx context.Context, conn *sql.Conn) error {
	for _, c := range []struct{ table, column, decl string }{
		{table: "ad_insight", column: "currency", decl: "TEXT"},
		{table: "ad_insight", column: "is_impression_too_specific", decl: "INTEGER"},
		{table: "ad_insight", column: "is_privacy_check_passed", decl: "INTEGER"},
		{table: "ad_insight", column: "suggested_bid", decl: "REAL"},
		{table: "ad_insight", column: "events_lost_to_bid", decl: "INTEGER"},
		{table: "ad_insight", column: "events_lost_to_budget", decl: "INTEGER"},
		{table: "ad_insight", column: "suggested_budget", decl: "REAL"},
		{table: "ad_insight", column: "aar_opt_in_status", decl: "INTEGER"},
		{table: "ad_insight", column: "recommendation_type", decl: "TEXT"},
		{table: "ad_insight", column: "ad_group_bid_landscape_type", decl: "TEXT"},
		{table: "ad_insight", column: "ad_group_id", decl: "TEXT"},
		{table: "ad_insight", column: "campaign_bid_landscape_type", decl: "TEXT"},
		{table: "ad_insight", column: "campaign_id", decl: "TEXT"},
		{table: "ad_insight", column: "keyword_id", decl: "TEXT"},
		{table: "ad_insight", column: "current_bid", decl: "REAL"},
		{table: "ad_insight", column: "estimated_increase_in_clicks", decl: "REAL"},
		{table: "ad_insight", column: "estimated_increase_in_cost", decl: "REAL"},
		{table: "ad_insight", column: "estimated_increase_in_impressions", decl: "TEXT"},
		{table: "ad_insight", column: "match_type", decl: "TEXT"},
		{table: "ad_insight", column: "opportunity_key", decl: "TEXT"},
		{table: "ad_insight", column: "budget_type", decl: "TEXT"},
		{table: "ad_insight", column: "current_budget", decl: "REAL"},
		{table: "ad_insight", column: "increase_in_clicks", decl: "REAL"},
		{table: "ad_insight", column: "increase_in_impressions", decl: "TEXT"},
		{table: "ad_insight", column: "percentage_increase_in_clicks", decl: "INTEGER"},
		{table: "ad_insight", column: "percentage_increase_in_impressions", decl: "INTEGER"},
		{table: "ad_insight", column: "recommended_budget", decl: "REAL"},
		{table: "ad_insight", column: "bid", decl: "REAL"},
		{table: "ad_insight", column: "category_name", decl: "TEXT"},
		{table: "ad_insight", column: "coverage", decl: "REAL"},
		{table: "ad_insight", column: "keyword", decl: "TEXT"},
		{table: "ad_insight", column: "category_id", decl: "TEXT"},
		{table: "ad_insight", column: "ad_group_name", decl: "TEXT"},
		{table: "ad_insight", column: "ad_impression_share", decl: "REAL"},
		{table: "ad_insight", column: "competition", decl: "TEXT"},
		{table: "ad_insight", column: "relevance", decl: "REAL"},
		{table: "ad_insight", column: "source", decl: "TEXT"},
		{table: "ad_insight", column: "entity_id", decl: "TEXT"},
		{table: "ad_insight", column: "entity_type", decl: "TEXT"},
		{table: "ad_insight", column: "kpi_type", decl: "TEXT"},
		{table: "ad_insight", column: "account_id", decl: "TEXT"},
		{table: "ad_insight", column: "current_clicks", decl: "TEXT"},
		{table: "ad_insight", column: "current_conversions", decl: "TEXT"},
		{table: "ad_insight", column: "current_cost", decl: "REAL"},
		{table: "ad_insight", column: "current_impressions", decl: "TEXT"},
		{table: "ad_insight", column: "estimated_increase_in_conversions", decl: "TEXT"},
		{table: "ad_insight", column: "recommendation_hash", decl: "TEXT"},
		{table: "ad_insight", column: "recommendation_id", decl: "TEXT"},
		{table: "ad_insight", column: "type", decl: "TEXT"},
		{table: "ad_insight", column: "confidence_score", decl: "REAL"},
		{table: "ad_insight", column: "suggested_keyword", decl: "TEXT"},
		{table: "bulk", column: "download_request_id", decl: "TEXT"},
		{table: "bulk", column: "percent_complete", decl: "INTEGER"},
		{table: "bulk", column: "request_status", decl: "TEXT"},
		{table: "bulk", column: "result_file_url", decl: "TEXT"},
		{table: "bulk", column: "request_id", decl: "TEXT"},
		{table: "bulk", column: "upload_url", decl: "TEXT"},
		{table: "campaign_management", column: "shared_entity_id", decl: "TEXT"},
		{table: "campaign_management", column: "image_url", decl: "TEXT"},
		{table: "campaign_management", column: "prompt_brand_warning", decl: "TEXT"},
		{table: "campaign_management", column: "account_id", decl: "TEXT"},
		{table: "campaign_management", column: "ad_group_type", decl: "TEXT"},
		{table: "campaign_management", column: "ad_schedule_use_searcher_time_zone", decl: "INTEGER"},
		{table: "campaign_management", column: "audience_ads_bid_adjustment", decl: "INTEGER"},
		{table: "campaign_management", column: "final_url_suffix", decl: "TEXT"},
		{table: "campaign_management", column: "language", decl: "TEXT"},
		{table: "campaign_management", column: "multimedia_ads_bid_adjustment", decl: "INTEGER"},
		{table: "campaign_management", column: "name", decl: "TEXT"},
		{table: "campaign_management", column: "network", decl: "TEXT"},
		{table: "campaign_management", column: "privacy_status", decl: "TEXT"},
		{table: "campaign_management", column: "status", decl: "TEXT"},
		{table: "campaign_management", column: "tracking_url_template", decl: "TEXT"},
		{table: "campaign_management", column: "use_optimized_targeting", decl: "INTEGER"},
		{table: "campaign_management", column: "use_predictive_targeting", decl: "INTEGER"},
		{table: "campaign_management", column: "is_account_opt_out", decl: "INTEGER"},
		{table: "campaign_management", column: "is_customer_opt_out", decl: "INTEGER"},
		{table: "campaign_management", column: "is_customer_opt_out_of_everything", decl: "INTEGER"},
		{table: "campaign_management", column: "justification", decl: "TEXT"},
		{table: "campaign_management", column: "asset_group_id", decl: "TEXT"},
		{table: "campaign_management", column: "asset_group_listing_type", decl: "TEXT"},
		{table: "campaign_management", column: "is_excluded", decl: "INTEGER"},
		{table: "campaign_management", column: "listing_group_path", decl: "TEXT"},
		{table: "campaign_management", column: "parent_listing_group_id", decl: "TEXT"},
		{table: "campaign_management", column: "business_name", decl: "TEXT"},
		{table: "campaign_management", column: "campaign_id", decl: "TEXT"},
		{table: "campaign_management", column: "size", decl: "TEXT"},
		{table: "campaign_management", column: "bid_strategy_id", decl: "TEXT"},
		{table: "campaign_management", column: "bid_strategy_scope", decl: "TEXT"},
		{table: "campaign_management", column: "budget_id", decl: "TEXT"},
		{table: "campaign_management", column: "budget_type", decl: "TEXT"},
		{table: "campaign_management", column: "daily_budget", decl: "REAL"},
		{table: "campaign_management", column: "experiment_id", decl: "TEXT"},
		{table: "campaign_management", column: "is_deal_campaign", decl: "INTEGER"},
		{table: "campaign_management", column: "is_political", decl: "INTEGER"},
		{table: "campaign_management", column: "sub_type", decl: "TEXT"},
		{table: "campaign_management", column: "time_zone", decl: "TEXT"},
		{table: "campaign_management", column: "use_campaign_level_dates", decl: "INTEGER"},
		{table: "campaign_management", column: "aspect_ratio", decl: "TEXT"},
		{table: "campaign_management", column: "duration", decl: "INTEGER"},
		{table: "campaign_management", column: "number_of_images", decl: "INTEGER"},
		{table: "campaign_management", column: "number_of_logos", decl: "INTEGER"},
		{table: "campaign_management", column: "number_of_text", decl: "INTEGER"},
		{table: "campaign_management", column: "template_description", decl: "TEXT"},
		{table: "campaign_management", column: "template_id", decl: "TEXT"},
		{table: "campaign_management", column: "template_name", decl: "TEXT"},
		{table: "campaign_management", column: "template_preview_url", decl: "TEXT"},
		{table: "campaign_management", column: "template_thumbnail_url", decl: "TEXT"},
		{table: "campaign_management", column: "config_value", decl: "TEXT"},
		{table: "campaign_management", column: "currency_code", decl: "TEXT"},
		{table: "campaign_management", column: "operation", decl: "TEXT"},
		{table: "campaign_management", column: "value", decl: "REAL"},
		{table: "campaign_management", column: "type", decl: "TEXT"},
		{table: "campaign_management", column: "file_import_upload_url", decl: "TEXT"},
		{table: "campaign_management", column: "file_url", decl: "TEXT"},
		{table: "campaign_management", column: "file_url_expiry_time_utc", decl: "DATETIME"},
		{table: "campaign_management", column: "last_modified_time_utc", decl: "DATETIME"},
		{table: "campaign_management", column: "error_log_url", decl: "TEXT"},
		{table: "campaign_management", column: "start_time_in_utc", decl: "DATETIME"},
		{table: "campaign_management", column: "match_type", decl: "TEXT"},
		{table: "campaign_management", column: "text", decl: "TEXT"},
		{table: "campaign_management", column: "media_type", decl: "TEXT"},
		{table: "campaign_management", column: "additional_value", decl: "REAL"},
		{table: "campaign_management", column: "failure_count", decl: "INTEGER"},
		{table: "campaign_management", column: "success_count", decl: "INTEGER"},
		{table: "campaign_management", column: "upload_date", decl: "DATETIME"},
		{table: "campaign_management", column: "logo_url", decl: "TEXT"},
		{table: "campaign_management", column: "profile_id", decl: "TEXT"},
		{table: "customer_billing", column: "create_time", decl: "DATETIME"},
		{table: "customer_billing", column: "insertion_order_id", decl: "TEXT"},
		{table: "customer_billing", column: "amount", decl: "REAL"},
		{table: "customer_billing", column: "number", decl: "TEXT"},
		{table: "customer_billing", column: "type", decl: "TEXT"},
		{table: "customer_billing", column: "account_id", decl: "TEXT"},
		{table: "customer_billing", column: "account_name", decl: "TEXT"},
		{table: "customer_billing", column: "account_number", decl: "TEXT"},
		{table: "customer_billing", column: "campaign_id", decl: "TEXT"},
		{table: "customer_billing", column: "currency_code", decl: "TEXT"},
		{table: "customer_billing", column: "customer_id", decl: "INTEGER"},
		{table: "customer_billing", column: "document_date", decl: "DATETIME"},
		{table: "customer_billing", column: "document_id", decl: "TEXT"},
		{table: "customer_billing", column: "document_number", decl: "TEXT"},
		{table: "customer_billing", column: "billing_group_id", decl: "TEXT"},
		{table: "customer_billing", column: "description", decl: "TEXT"},
		{table: "customer_billing", column: "name", decl: "TEXT"},
		{table: "customer_billing", column: "active_duration", decl: "INTEGER"},
		{table: "customer_billing", column: "balance", decl: "REAL"},
		{table: "customer_billing", column: "claim_date", decl: "DATETIME"},
		{table: "customer_billing", column: "coupon_id", decl: "INTEGER"},
		{table: "customer_billing", column: "coupon_type", decl: "INTEGER"},
		{table: "customer_billing", column: "coupon_value", decl: "REAL"},
		{table: "customer_billing", column: "end_date", decl: "DATETIME"},
		{table: "customer_billing", column: "expiration_date", decl: "DATETIME"},
		{table: "customer_billing", column: "feature_id", decl: "INTEGER"},
		{table: "customer_billing", column: "percent_off", decl: "REAL"},
		{table: "customer_billing", column: "redemption_date", decl: "DATETIME"},
		{table: "customer_billing", column: "spend_to_threshold", decl: "REAL"},
		{table: "customer_billing", column: "start_date", decl: "DATETIME"},
		{table: "customer_billing", column: "status", decl: "INTEGER"},
		{table: "customer_billing", column: "upfront_spending", decl: "REAL"},
		{table: "customer_billing", column: "class_name", decl: "TEXT"},
		{table: "customer_billing", column: "coupon_code", decl: "TEXT"},
		{table: "customer_billing", column: "is_redeemed", decl: "INTEGER"},
		{table: "customer_billing", column: "send_to_date", decl: "DATETIME"},
		{table: "customer_billing", column: "send_to_email", decl: "TEXT"},
		{table: "customer_billing", column: "spend_threshold", decl: "REAL"},
		{table: "customer_billing", column: "booking_country_code", decl: "TEXT"},
		{table: "customer_billing", column: "budget_remaining", decl: "REAL"},
		{table: "customer_billing", column: "budget_remaining_percent", decl: "REAL"},
		{table: "customer_billing", column: "budget_spent", decl: "REAL"},
		{table: "customer_billing", column: "budget_spent_percent", decl: "REAL"},
		{table: "customer_billing", column: "comment", decl: "TEXT"},
		{table: "customer_billing", column: "is_endless", decl: "INTEGER"},
		{table: "customer_billing", column: "is_in_series", decl: "INTEGER"},
		{table: "customer_billing", column: "is_unlimited", decl: "INTEGER"},
		{table: "customer_billing", column: "last_modified_by_user_id", decl: "TEXT"},
		{table: "customer_billing", column: "last_modified_time", decl: "DATETIME"},
		{table: "customer_billing", column: "notification_threshold", decl: "REAL"},
		{table: "customer_billing", column: "purchase_order", decl: "TEXT"},
		{table: "customer_billing", column: "reference_id", decl: "TEXT"},
		{table: "customer_billing", column: "series_frequency_type", decl: "TEXT"},
		{table: "customer_billing", column: "series_name", decl: "TEXT"},
		{table: "customer_billing", column: "spend_cap_amount", decl: "REAL"},
		{table: "customer_management", column: "account_id", decl: "TEXT"},
		{table: "customer_management", column: "account_number", decl: "TEXT"},
		{table: "customer_management", column: "create_time", decl: "DATETIME"},
		{table: "customer_management", column: "account_life_cycle_status", decl: "TEXT"},
		{table: "customer_management", column: "name", decl: "TEXT"},
		{table: "customer_management", column: "number", decl: "TEXT"},
		{table: "customer_management", column: "pause_reason", decl: "INTEGER"},
		{table: "customer_management", column: "account_mode", decl: "TEXT"},
		{table: "customer_management", column: "account_name", decl: "TEXT"},
		{table: "customer_management", column: "customer_id", decl: "TEXT"},
		{table: "customer_management", column: "customer_name", decl: "TEXT"},
		{table: "customer_management", column: "valid_fields", decl: "INTEGER"},
		{table: "customer_management", column: "message", decl: "TEXT"},
		{table: "customer_management", column: "severity", decl: "INTEGER"},
		{table: "customer_management", column: "title", decl: "TEXT"},
		{table: "customer_management", column: "type_id", decl: "INTEGER"},
		{table: "customer_management", column: "customer_link_permission", decl: "TEXT"},
		{table: "customer_management", column: "role_id", decl: "INTEGER"},
		{table: "customer_management", column: "user_name", decl: "TEXT"},
		{table: "customer_management", column: "account_financial_status", decl: "TEXT"},
		{table: "customer_management", column: "auto_tag_type", decl: "TEXT"},
		{table: "customer_management", column: "back_up_payment_instrument_id", decl: "TEXT"},
		{table: "customer_management", column: "bill_to_customer_id", decl: "TEXT"},
		{table: "customer_management", column: "billing_threshold_amount", decl: "REAL"},
		{table: "customer_management", column: "currency_code", decl: "TEXT"},
		{table: "customer_management", column: "language", decl: "TEXT"},
		{table: "customer_management", column: "last_modified_by_user_id", decl: "TEXT"},
		{table: "customer_management", column: "last_modified_time", decl: "DATETIME"},
		{table: "customer_management", column: "parent_customer_id", decl: "TEXT"},
		{table: "customer_management", column: "payment_method_id", decl: "TEXT"},
		{table: "customer_management", column: "payment_method_type", decl: "TEXT"},
		{table: "customer_management", column: "primary_user_id", decl: "TEXT"},
		{table: "customer_management", column: "sales_house_customer_id", decl: "TEXT"},
		{table: "customer_management", column: "sold_to_payment_instrument_id", decl: "TEXT"},
		{table: "customer_management", column: "time_stamp", decl: "TEXT"},
		{table: "customer_management", column: "time_zone", decl: "TEXT"},
		{table: "customer_management", column: "client_entity_customer_number", decl: "TEXT"},
		{table: "customer_management", column: "client_entity_id", decl: "TEXT"},
		{table: "customer_management", column: "client_entity_name", decl: "TEXT"},
		{table: "customer_management", column: "client_entity_number", decl: "TEXT"},
		{table: "customer_management", column: "inviter_email", decl: "TEXT"},
		{table: "customer_management", column: "inviter_name", decl: "TEXT"},
		{table: "customer_management", column: "inviter_phone", decl: "TEXT"},
		{table: "customer_management", column: "is_bill_to_client", decl: "INTEGER"},
		{table: "customer_management", column: "last_modified_date_time", decl: "DATETIME"},
		{table: "customer_management", column: "managing_customer_id", decl: "TEXT"},
		{table: "customer_management", column: "managing_customer_name", decl: "TEXT"},
		{table: "customer_management", column: "managing_customer_number", decl: "TEXT"},
		{table: "customer_management", column: "note", decl: "TEXT"},
		{table: "customer_management", column: "start_date", decl: "DATETIME"},
		{table: "customer_management", column: "status", decl: "TEXT"},
		{table: "customer_management", column: "suppress_notification", decl: "INTEGER"},
		{table: "customer_management", column: "timestamp", decl: "TEXT"},
		{table: "customer_management", column: "type", decl: "TEXT"},
		{table: "customer_management", column: "customer_financial_status", decl: "TEXT"},
		{table: "customer_management", column: "customer_life_cycle_status", decl: "TEXT"},
		{table: "customer_management", column: "industry", decl: "TEXT"},
		{table: "customer_management", column: "market_country", decl: "TEXT"},
		{table: "customer_management", column: "market_language", decl: "TEXT"},
		{table: "customer_management", column: "service_level", decl: "TEXT"},
		{table: "customer_management", column: "email", decl: "TEXT"},
		{table: "customer_management", column: "expiration_date", decl: "DATETIME"},
		{table: "customer_management", column: "first_name", decl: "TEXT"},
		{table: "customer_management", column: "last_name", decl: "TEXT"},
		{table: "customer_management", column: "lcid", decl: "TEXT"},
		{table: "customer_management", column: "user_invitation_id", decl: "TEXT"},
		{table: "customer_management", column: "customer_number", decl: "TEXT"},
		{table: "customer_management", column: "city_name", decl: "TEXT"},
		{table: "customer_management", column: "country_code", decl: "TEXT"},
		{table: "customer_management", column: "postal_code", decl: "TEXT"},
		{table: "customer_management", column: "province_code", decl: "TEXT"},
		{table: "customer_management", column: "province_name", decl: "TEXT"},
		{table: "customer_management", column: "street_address", decl: "TEXT"},
		{table: "customer_management", column: "street_address2", decl: "TEXT"},
		{table: "reporting", column: "report_request_id", decl: "TEXT"},
		{table: "sync_state", column: "last_cursor", decl: "TEXT"},
		{table: "sync_state", column: "last_synced_at", decl: "DATETIME"},
		{table: "sync_state", column: "total_count", decl: "INTEGER DEFAULT 0"},
	} {
		if err := s.ensureColumn(ctx, conn, c.table, c.column, c.decl); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) migrate(ctx context.Context) error {
	// Acquiring the migration connection establishes a physical SQLite
	// connection, which runs the DSN _pragma directives — including the
	// journal_mode(WAL) conversion. On a fresh DB opened by several
	// processes at once, that conversion briefly needs an exclusive lock
	// and can return SQLITE_BUSY before any statement-level busy handler
	// applies, so retry the acquisition against the shared deadline.
	deadline := time.Now().Add(migrationLockTimeout)
	var conn *sql.Conn
	if err := retryOnBusy(ctx, deadline, "acquiring migration connection", func() error {
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

	// Read user_version before the migration lock so an old binary
	// opening a newer-schema DB rejects immediately. WAL readers don't
	// normally block on writers, but the fresh-DB WAL-init race can BUSY
	// a SELECT — share the lock's deadline so total budget stays bounded.
	var current int
	if err := retryOnBusy(ctx, deadline, "reading schema version", func() error {
		return conn.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&current)
	}); err != nil {
		return err
	}
	if current > StoreSchemaVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d; upgrade the CLI binary or open an older database", current, StoreSchemaVersion)
	}

	migrations := []string{
		`CREATE TABLE IF NOT EXISTS resources (
			id TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			data JSON NOT NULL,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (resource_type, id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_resources_type ON resources(resource_type)`,
		`CREATE INDEX IF NOT EXISTS idx_resources_synced ON resources(synced_at)`,
		`CREATE TABLE IF NOT EXISTS sync_state (
			resource_type TEXT PRIMARY KEY,
			last_cursor TEXT,
			last_synced_at DATETIME,
			total_count INTEGER DEFAULT 0
		)`,
		resourcesFTSCreateSQL,
		// CLI Printing Press: learn migrations
		//
		// search_learnings: LLM-driven per-query reranking. Populated by
		// the `teach` command (silent, backgrounded by the LLM after a
		// successful response) and read by the rerank layer to
		// boost/hide/alias hits on subsequent queries. See learnings.go
		// for the full semantics. Per-user table; stays small.
		//
		// query_entities: JSON array of case-preserving entity tokens
		// extracted from query_pattern at teach time. Used by the recall
		// match validator to reject cross-entity matches that would
		// otherwise score high on non-entity Jaccard.
		`CREATE TABLE IF NOT EXISTS search_learnings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			query_pattern TEXT NOT NULL,
			query_entities TEXT,
			venue TEXT,
			resource_type TEXT,
			resource_id TEXT NOT NULL,
			action TEXT NOT NULL,
			alias_target TEXT,
			source TEXT NOT NULL,
			confidence INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_observed_at DATETIME,
			notes TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_learn_query ON search_learnings(query_pattern)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_learn_unique ON search_learnings(query_pattern, resource_id, action)`,
		// entity_lookups: canonical-to-value reference data for the
		// pattern substitution engine in internal/learn/patterns. Seeded
		// at migration time by the consumer (e.g., a CLI may register
		// country codes, sports team abbreviations, etc.); per-user
		// additions land via the `teach-lookup` CLI command with
		// source='taught'. PK is the (kind, canonical, value) triple so
		// multiple aliases under the same kind coexist without
		// collision.
		`CREATE TABLE IF NOT EXISTS entity_lookups (
			kind TEXT NOT NULL,
			canonical TEXT NOT NULL,
			value TEXT NOT NULL,
			source TEXT NOT NULL DEFAULT 'seeded',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (kind, canonical, value)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_entity_lookup_canonical ON entity_lookups(canonical)`,
		`CREATE INDEX IF NOT EXISTS idx_entity_lookup_kind ON entity_lookups(kind)`,
		// search_patterns: inferred and taught templates for the
		// generalization layer in internal/learn/patterns. Each row
		// encodes a query_template with one {entity[:kind]} slot and a
		// resource_template that names how the entity substitutes into
		// the resource ID. Extract() writes "inferred" rows whenever
		// two or more search_learnings rows share a structural shape;
		// the teach-pattern CLI command writes "taught" rows directly
		// for explicit template authorship.
		//
		// Idempotency leans on idx_patterns_unique: a re-Extract pass
		// over the same source learnings re-asserts the same
		// (query_template, resource_template, strategy) triple, which
		// bumps confidence and refreshes last_observed_at on the
		// existing row rather than spawning a duplicate.
		`CREATE TABLE IF NOT EXISTS search_patterns (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			query_template TEXT NOT NULL,
			resource_template TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			venue TEXT,
			strategy TEXT NOT NULL,
			entity_kind TEXT NOT NULL,
			confidence INTEGER NOT NULL DEFAULT 2,
			source TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_observed_at DATETIME,
			example_query TEXT,
			example_resource TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_patterns_query_template ON search_patterns(query_template)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_patterns_unique ON search_patterns(query_template, resource_template, strategy)`,
		// learning_playbooks (v7): hand-authored playbook primitive
		// keyed on the structural query family (all entities stripped;
		// see learn.QueryFamily). One row per family holds the optional
		// structured playbook (ordered CLI command sequence with entity
		// slots) and the optional free-text notes (gotchas, workarounds
		// the CLI surface doesn't expose). Either field may be empty;
		// non-empty in both is the strongest signal.
		//
		// Read at recall time by query_family; surfaces to the agent
		// alongside the existing per-resource hits so a future inquiry
		// of the same shape can skip rediscovery of the choreography.
		//
		// Distinct concept from search_patterns (which auto-extracts
		// generalization templates from search_learnings); playbooks
		// are hand-authored choreography + notes attached by family.
		`CREATE TABLE IF NOT EXISTS learning_playbooks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			query_family TEXT NOT NULL UNIQUE,
			playbook_json TEXT,
			notes_text TEXT,
			source TEXT NOT NULL DEFAULT 'taught',
			confidence INTEGER NOT NULL DEFAULT 2,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_observed_at TIMESTAMP
		)`,
		// query_family already carries a column-level UNIQUE constraint
		// (SQLite auto-creates the backing index), so no separate
		// CREATE UNIQUE INDEX is needed -- a second named unique index
		// would just double the write cost on every upsert.
		`CREATE INDEX IF NOT EXISTS idx_playbooks_source ON learning_playbooks(source)`,
		`CREATE INDEX IF NOT EXISTS idx_playbooks_last_observed_at ON learning_playbooks(last_observed_at)`,
		// learn_candidates (v9): CLI-derived improvement candidates
		// awaiting explicit agent judgment. Rows are written by the
		// post-run derivation pass (flag corrections, repeated
		// discovery shapes) and surfaced read-only in the recall
		// envelope. Candidates are structurally quarantined: they
		// never become search_learnings rows and sightings never
		// grant skip authority — only an explicit confirm promotes
		// the payload. derivation_signature dedupes re-derivations of
		// the same observation into a sightings bump instead of a
		// duplicate row.
		`CREATE TABLE IF NOT EXISTS learn_candidates (
			id INTEGER PRIMARY KEY,
			class TEXT NOT NULL CHECK(class IN ('flag_alias','playbook_candidate')),
			payload TEXT NOT NULL,
			derivation_signature TEXT NOT NULL UNIQUE,
			sightings INTEGER NOT NULL DEFAULT 1,
			status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open','confirmed','rejected','expired')),
			query_family TEXT,
			command_path TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			last_seen_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_learn_candidates_status ON learn_candidates(status)`,
		`CREATE INDEX IF NOT EXISTS idx_learn_candidates_family ON learn_candidates(query_family)`,
		// learn_events (v9): capped, best-effort telemetry for the
		// learn loop's measurement layer. recall logs hit/miss with
		// the matched row id so teach-to-reuse joins by row id (family
		// hash as fallback); `learnings stats` aggregates over it.
		// Inserts are telemetry-class — they never fail the command
		// and never hold writeMu across a recall match.
		`CREATE TABLE IF NOT EXISTS learn_events (
			id INTEGER PRIMARY KEY,
			ts TEXT NOT NULL,
			event TEXT NOT NULL CHECK(event IN ('recall_hit','recall_miss','recall_playbook_hit','teach','teach_playbook','amend','forget','candidate_confirmed','candidate_rejected')),
			query_family_hash TEXT,
			matched_row_id INTEGER,
			entity_match INTEGER,
			surface TEXT CHECK(surface IN ('cli','mcp'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_learn_events_event_ts ON learn_events(event, ts)`,
		`CREATE TABLE IF NOT EXISTS "ad_insight" (
			"id" TEXT PRIMARY KEY,
			"data" JSON NOT NULL,
			"synced_at" DATETIME DEFAULT CURRENT_TIMESTAMP,
			"currency" TEXT,
			"is_impression_too_specific" INTEGER,
			"is_privacy_check_passed" INTEGER,
			"suggested_bid" REAL,
			"events_lost_to_bid" INTEGER,
			"events_lost_to_budget" INTEGER,
			"suggested_budget" REAL,
			"aar_opt_in_status" INTEGER,
			"recommendation_type" TEXT,
			"ad_group_bid_landscape_type" TEXT,
			"ad_group_id" TEXT,
			"campaign_bid_landscape_type" TEXT,
			"campaign_id" TEXT,
			"keyword_id" TEXT,
			"current_bid" REAL,
			"estimated_increase_in_clicks" REAL,
			"estimated_increase_in_cost" REAL,
			"estimated_increase_in_impressions" TEXT,
			"match_type" TEXT,
			"opportunity_key" TEXT,
			"budget_type" TEXT,
			"current_budget" REAL,
			"increase_in_clicks" REAL,
			"increase_in_impressions" TEXT,
			"percentage_increase_in_clicks" INTEGER,
			"percentage_increase_in_impressions" INTEGER,
			"recommended_budget" REAL,
			"bid" REAL,
			"category_name" TEXT,
			"coverage" REAL,
			"keyword" TEXT,
			"category_id" TEXT,
			"ad_group_name" TEXT,
			"ad_impression_share" REAL,
			"competition" TEXT,
			"relevance" REAL,
			"source" TEXT,
			"entity_id" TEXT,
			"entity_type" TEXT,
			"kpi_type" TEXT,
			"account_id" TEXT,
			"current_clicks" TEXT,
			"current_conversions" TEXT,
			"current_cost" REAL,
			"current_impressions" TEXT,
			"estimated_increase_in_conversions" TEXT,
			"recommendation_hash" TEXT,
			"recommendation_id" TEXT,
			"type" TEXT,
			"confidence_score" REAL,
			"suggested_keyword" TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS "bulk" (
			"id" TEXT PRIMARY KEY,
			"data" JSON NOT NULL,
			"synced_at" DATETIME DEFAULT CURRENT_TIMESTAMP,
			"download_request_id" TEXT,
			"percent_complete" INTEGER,
			"request_status" TEXT,
			"result_file_url" TEXT,
			"request_id" TEXT,
			"upload_url" TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS "campaign_management" (
			"id" TEXT PRIMARY KEY,
			"data" JSON NOT NULL,
			"synced_at" DATETIME DEFAULT CURRENT_TIMESTAMP,
			"shared_entity_id" TEXT,
			"image_url" TEXT,
			"prompt_brand_warning" TEXT,
			"account_id" TEXT,
			"ad_group_type" TEXT,
			"ad_schedule_use_searcher_time_zone" INTEGER,
			"audience_ads_bid_adjustment" INTEGER,
			"final_url_suffix" TEXT,
			"language" TEXT,
			"multimedia_ads_bid_adjustment" INTEGER,
			"name" TEXT,
			"network" TEXT,
			"privacy_status" TEXT,
			"status" TEXT,
			"tracking_url_template" TEXT,
			"use_optimized_targeting" INTEGER,
			"use_predictive_targeting" INTEGER,
			"is_account_opt_out" INTEGER,
			"is_customer_opt_out" INTEGER,
			"is_customer_opt_out_of_everything" INTEGER,
			"justification" TEXT,
			"asset_group_id" TEXT,
			"asset_group_listing_type" TEXT,
			"is_excluded" INTEGER,
			"listing_group_path" TEXT,
			"parent_listing_group_id" TEXT,
			"business_name" TEXT,
			"campaign_id" TEXT,
			"size" TEXT,
			"bid_strategy_id" TEXT,
			"bid_strategy_scope" TEXT,
			"budget_id" TEXT,
			"budget_type" TEXT,
			"daily_budget" REAL,
			"experiment_id" TEXT,
			"is_deal_campaign" INTEGER,
			"is_political" INTEGER,
			"sub_type" TEXT,
			"time_zone" TEXT,
			"use_campaign_level_dates" INTEGER,
			"aspect_ratio" TEXT,
			"duration" INTEGER,
			"number_of_images" INTEGER,
			"number_of_logos" INTEGER,
			"number_of_text" INTEGER,
			"template_description" TEXT,
			"template_id" TEXT,
			"template_name" TEXT,
			"template_preview_url" TEXT,
			"template_thumbnail_url" TEXT,
			"config_value" TEXT,
			"currency_code" TEXT,
			"operation" TEXT,
			"value" REAL,
			"type" TEXT,
			"file_import_upload_url" TEXT,
			"file_url" TEXT,
			"file_url_expiry_time_utc" DATETIME,
			"last_modified_time_utc" DATETIME,
			"error_log_url" TEXT,
			"start_time_in_utc" DATETIME,
			"match_type" TEXT,
			"text" TEXT,
			"media_type" TEXT,
			"additional_value" REAL,
			"failure_count" INTEGER,
			"success_count" INTEGER,
			"upload_date" DATETIME,
			"logo_url" TEXT,
			"profile_id" TEXT
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS "campaign_management_fts" USING fts5(
			"name",
			"text",
			content='campaign_management',
			content_rowid='rowid'
		)`,
		`CREATE TRIGGER IF NOT EXISTS "campaign_management_ai" AFTER INSERT ON "campaign_management" BEGIN
			INSERT INTO "campaign_management_fts"(rowid, "name", "text")
			VALUES (new.rowid,new."name", new."text");
		END`,
		`CREATE TRIGGER IF NOT EXISTS "campaign_management_ad" AFTER DELETE ON "campaign_management" BEGIN
			INSERT INTO "campaign_management_fts"("campaign_management_fts", rowid, "name", "text")
			VALUES ('delete', old.rowid,old."name", old."text");
		END`,
		`CREATE TRIGGER IF NOT EXISTS "campaign_management_au" AFTER UPDATE ON "campaign_management" BEGIN
			INSERT INTO "campaign_management_fts"("campaign_management_fts", rowid, "name", "text")
			VALUES ('delete', old.rowid,old."name", old."text");
			INSERT INTO "campaign_management_fts"(rowid, "name", "text")
			VALUES (new.rowid,new."name", new."text");
		END`,
		`CREATE TABLE IF NOT EXISTS "customer_billing" (
			"id" TEXT PRIMARY KEY,
			"data" JSON NOT NULL,
			"synced_at" DATETIME DEFAULT CURRENT_TIMESTAMP,
			"create_time" DATETIME,
			"insertion_order_id" TEXT,
			"amount" REAL,
			"number" TEXT,
			"type" TEXT,
			"account_id" TEXT,
			"account_name" TEXT,
			"account_number" TEXT,
			"campaign_id" TEXT,
			"currency_code" TEXT,
			"customer_id" INTEGER,
			"document_date" DATETIME,
			"document_id" TEXT,
			"document_number" TEXT,
			"billing_group_id" TEXT,
			"description" TEXT,
			"name" TEXT,
			"active_duration" INTEGER,
			"balance" REAL,
			"claim_date" DATETIME,
			"coupon_id" INTEGER,
			"coupon_type" INTEGER,
			"coupon_value" REAL,
			"end_date" DATETIME,
			"expiration_date" DATETIME,
			"feature_id" INTEGER,
			"percent_off" REAL,
			"redemption_date" DATETIME,
			"spend_to_threshold" REAL,
			"start_date" DATETIME,
			"status" INTEGER,
			"upfront_spending" REAL,
			"class_name" TEXT,
			"coupon_code" TEXT,
			"is_redeemed" INTEGER,
			"send_to_date" DATETIME,
			"send_to_email" TEXT,
			"spend_threshold" REAL,
			"booking_country_code" TEXT,
			"budget_remaining" REAL,
			"budget_remaining_percent" REAL,
			"budget_spent" REAL,
			"budget_spent_percent" REAL,
			"comment" TEXT,
			"is_endless" INTEGER,
			"is_in_series" INTEGER,
			"is_unlimited" INTEGER,
			"last_modified_by_user_id" TEXT,
			"last_modified_time" DATETIME,
			"notification_threshold" REAL,
			"purchase_order" TEXT,
			"reference_id" TEXT,
			"series_frequency_type" TEXT,
			"series_name" TEXT,
			"spend_cap_amount" REAL
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS "customer_billing_fts" USING fts5(
			"description",
			"name",
			"comment",
			content='customer_billing',
			content_rowid='rowid'
		)`,
		`CREATE TRIGGER IF NOT EXISTS "customer_billing_ai" AFTER INSERT ON "customer_billing" BEGIN
			INSERT INTO "customer_billing_fts"(rowid, "description", "name", "comment")
			VALUES (new.rowid,new."description", new."name", new."comment");
		END`,
		`CREATE TRIGGER IF NOT EXISTS "customer_billing_ad" AFTER DELETE ON "customer_billing" BEGIN
			INSERT INTO "customer_billing_fts"("customer_billing_fts", rowid, "description", "name", "comment")
			VALUES ('delete', old.rowid,old."description", old."name", old."comment");
		END`,
		`CREATE TRIGGER IF NOT EXISTS "customer_billing_au" AFTER UPDATE ON "customer_billing" BEGIN
			INSERT INTO "customer_billing_fts"("customer_billing_fts", rowid, "description", "name", "comment")
			VALUES ('delete', old.rowid,old."description", old."name", old."comment");
			INSERT INTO "customer_billing_fts"(rowid, "description", "name", "comment")
			VALUES (new.rowid,new."description", new."name", new."comment");
		END`,
		`CREATE TABLE IF NOT EXISTS "customer_management" (
			"id" TEXT PRIMARY KEY,
			"data" JSON NOT NULL,
			"synced_at" DATETIME DEFAULT CURRENT_TIMESTAMP,
			"account_id" TEXT,
			"account_number" TEXT,
			"create_time" DATETIME,
			"account_life_cycle_status" TEXT,
			"name" TEXT,
			"number" TEXT,
			"pause_reason" INTEGER,
			"account_mode" TEXT,
			"account_name" TEXT,
			"customer_id" TEXT,
			"customer_name" TEXT,
			"valid_fields" INTEGER,
			"message" TEXT,
			"severity" INTEGER,
			"title" TEXT,
			"type_id" INTEGER,
			"customer_link_permission" TEXT,
			"role_id" INTEGER,
			"user_name" TEXT,
			"account_financial_status" TEXT,
			"auto_tag_type" TEXT,
			"back_up_payment_instrument_id" TEXT,
			"bill_to_customer_id" TEXT,
			"billing_threshold_amount" REAL,
			"currency_code" TEXT,
			"language" TEXT,
			"last_modified_by_user_id" TEXT,
			"last_modified_time" DATETIME,
			"parent_customer_id" TEXT,
			"payment_method_id" TEXT,
			"payment_method_type" TEXT,
			"primary_user_id" TEXT,
			"sales_house_customer_id" TEXT,
			"sold_to_payment_instrument_id" TEXT,
			"time_stamp" TEXT,
			"time_zone" TEXT,
			"client_entity_customer_number" TEXT,
			"client_entity_id" TEXT,
			"client_entity_name" TEXT,
			"client_entity_number" TEXT,
			"inviter_email" TEXT,
			"inviter_name" TEXT,
			"inviter_phone" TEXT,
			"is_bill_to_client" INTEGER,
			"last_modified_date_time" DATETIME,
			"managing_customer_id" TEXT,
			"managing_customer_name" TEXT,
			"managing_customer_number" TEXT,
			"note" TEXT,
			"start_date" DATETIME,
			"status" TEXT,
			"suppress_notification" INTEGER,
			"timestamp" TEXT,
			"type" TEXT,
			"customer_financial_status" TEXT,
			"customer_life_cycle_status" TEXT,
			"industry" TEXT,
			"market_country" TEXT,
			"market_language" TEXT,
			"service_level" TEXT,
			"email" TEXT,
			"expiration_date" DATETIME,
			"first_name" TEXT,
			"last_name" TEXT,
			"lcid" TEXT,
			"user_invitation_id" TEXT,
			"customer_number" TEXT,
			"city_name" TEXT,
			"country_code" TEXT,
			"postal_code" TEXT,
			"province_code" TEXT,
			"province_name" TEXT,
			"street_address" TEXT,
			"street_address2" TEXT
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS "customer_management_fts" USING fts5(
			"name",
			"message",
			"title",
			"note",
			content='customer_management',
			content_rowid='rowid'
		)`,
		`CREATE TRIGGER IF NOT EXISTS "customer_management_ai" AFTER INSERT ON "customer_management" BEGIN
			INSERT INTO "customer_management_fts"(rowid, "name", "message", "title", "note")
			VALUES (new.rowid,new."name", new."message", new."title", new."note");
		END`,
		`CREATE TRIGGER IF NOT EXISTS "customer_management_ad" AFTER DELETE ON "customer_management" BEGIN
			INSERT INTO "customer_management_fts"("customer_management_fts", rowid, "name", "message", "title", "note")
			VALUES ('delete', old.rowid,old."name", old."message", old."title", old."note");
		END`,
		`CREATE TRIGGER IF NOT EXISTS "customer_management_au" AFTER UPDATE ON "customer_management" BEGIN
			INSERT INTO "customer_management_fts"("customer_management_fts", rowid, "name", "message", "title", "note")
			VALUES ('delete', old.rowid,old."name", old."message", old."title", old."note");
			INSERT INTO "customer_management_fts"(rowid, "name", "message", "title", "note")
			VALUES (new.rowid,new."name", new."message", new."title", new."note");
		END`,
		`CREATE TABLE IF NOT EXISTS "reporting" (
			"id" TEXT PRIMARY KEY,
			"data" JSON NOT NULL,
			"synced_at" DATETIME DEFAULT CURRENT_TIMESTAMP,
			"report_request_id" TEXT
		)`,
	}

	// Run every migration — including the column backfill and the
	// schema-version stamp — inside a single BEGIN IMMEDIATE transaction
	// pinned to one connection. IMMEDIATE acquires SQLite's RESERVED lock
	// at BEGIN time so concurrent migrators serialize on it instead of
	// racing per-statement and tripping SQLITE_BUSY despite busy_timeout.
	// modernc.org/sqlite's busy_timeout does not always cover write-write
	// contention at BEGIN/COMMIT time, so we retry both explicitly on
	// SQLITE_BUSY for up to migrationLockTimeout.
	return withMigrationLock(ctx, conn, deadline, func() error {
		// Re-read user_version inside the lock. This is load-bearing,
		// not paranoid: between the pre-lock read above and our
		// successful BEGIN IMMEDIATE, a newer-binary peer may have
		// committed a higher version stamp. Without this re-read, an
		// older binary (smaller StoreSchemaVersion) would proceed to
		// stamp its own lower version at the end of the closure,
		// silently downgrading user_version on a schema that's already
		// at the newer level. Future maintainers: leave this read in.
		var current int
		if err := conn.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&current); err != nil {
			return fmt.Errorf("reading schema version: %w", err)
		}
		if current > StoreSchemaVersion {
			return fmt.Errorf("database schema version %d is newer than supported version %d; upgrade the CLI binary or open an older database", current, StoreSchemaVersion)
		}

		if current < 2 {
			if err := s.migrateResourcesCompositeKey(ctx, conn); err != nil {
				return fmt.Errorf("migrating resources composite key: %w", err)
			}
		}
		if current == 2 {
			if err := s.migrateResourcesFTSRowIDs(ctx, conn); err != nil {
				return fmt.Errorf("migrating resources FTS rowids: %w", err)
			}
		}

		if err := s.backfillColumns(ctx, conn); err != nil {
			return fmt.Errorf("backfilling columns: %w", err)
		}
		for _, m := range migrations {
			if _, err := conn.ExecContext(ctx, m); err != nil {
				return fmt.Errorf("migration failed: %w", err)
			}
		}
		if err := s.migrateExtras(ctx, conn); err != nil {
			return fmt.Errorf("running extra migrations: %w", err)
		}
		if current < resourcesFTSContentSchemaVersion {
			if err := s.migrateResourcesFTSContent(ctx, conn); err != nil {
				return fmt.Errorf("migrating resources FTS content: %w", err)
			}
		}
		// Stamp the schema version. On a fresh DB this writes the current
		// StoreSchemaVersion; on an already-stamped DB this is a no-op
		// write of the same value.
		// An older DB with user_version = 0 and pre-existing tables gets
		// stamped here after any version-gated rewrites and idempotent
		// CREATE TABLE IF NOT EXISTS statements have completed.
		if _, err := conn.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, StoreSchemaVersion)); err != nil {
			return fmt.Errorf("stamp user_version: %w", err)
		}
		return nil
	})
}

func (s *Store) migrateResourcesCompositeKey(ctx context.Context, conn *sql.Conn) error {
	exists, err := tableExists(ctx, conn, "resources")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	composite, err := resourcesTableHasCompositeKey(ctx, conn)
	if err != nil {
		return err
	}
	if !composite {
		if _, err := conn.ExecContext(ctx, `CREATE TABLE resources_v2 (
			id TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			data JSON NOT NULL,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (resource_type, id)
		)`); err != nil {
			return fmt.Errorf("creating resources_v2: %w", err)
		}
		if _, err := conn.ExecContext(ctx, `INSERT INTO resources_v2 (id, resource_type, data, synced_at, updated_at)
			SELECT id, resource_type, data, synced_at, updated_at FROM resources`); err != nil {
			return fmt.Errorf("copying resources rows: %w", err)
		}
		if _, err := conn.ExecContext(ctx, `DROP TABLE resources`); err != nil {
			return fmt.Errorf("dropping old resources table: %w", err)
		}
		if _, err := conn.ExecContext(ctx, `ALTER TABLE resources_v2 RENAME TO resources`); err != nil {
			return fmt.Errorf("renaming resources_v2: %w", err)
		}
	}

	// Always rebuild FTS during the v2 transition. The resources table may
	// already have the composite key, but v1 FTS rowids were scoped by id
	// alone and must be replaced with resource_type + id rowids.
	if _, err := conn.ExecContext(ctx, `DROP TABLE IF EXISTS resources_fts`); err != nil {
		return fmt.Errorf("dropping resources_fts: %w", err)
	}
	if _, err := conn.ExecContext(ctx, resourcesFTSCreateSQL); err != nil {
		return fmt.Errorf("creating resources_fts: %w", err)
	}
	if err := rebuildResourcesFTS(ctx, conn); err != nil {
		return fmt.Errorf("rebuilding resources_fts: %w", err)
	}
	return nil
}

func (s *Store) migrateResourcesFTSRowIDs(ctx context.Context, conn *sql.Conn) error {
	exists, err := tableExists(ctx, conn, "resources")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	if _, err := conn.ExecContext(ctx, `DROP TABLE IF EXISTS resources_fts`); err != nil {
		return fmt.Errorf("dropping resources_fts: %w", err)
	}
	if _, err := conn.ExecContext(ctx, resourcesFTSCreateSQL); err != nil {
		return fmt.Errorf("creating resources_fts: %w", err)
	}
	if err := rebuildResourcesFTS(ctx, conn); err != nil {
		return fmt.Errorf("rebuilding resources_fts: %w", err)
	}
	return nil
}

func (s *Store) migrateResourcesFTSContent(ctx context.Context, conn *sql.Conn) error {
	exists, err := tableExists(ctx, conn, "resources")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	if _, err := conn.ExecContext(ctx, `DROP TABLE IF EXISTS resources_fts`); err != nil {
		return fmt.Errorf("dropping resources_fts: %w", err)
	}
	if _, err := conn.ExecContext(ctx, resourcesFTSCreateSQL); err != nil {
		return fmt.Errorf("creating resources_fts: %w", err)
	}
	if err := rebuildResourcesFTS(ctx, conn); err != nil {
		return fmt.Errorf("rebuilding resources_fts: %w", err)
	}
	return nil
}

func tableExists(ctx context.Context, conn *sql.Conn, name string) (bool, error) {
	var count int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&count); err != nil {
		return false, fmt.Errorf("checking table %s: %w", name, err)
	}
	return count > 0, nil
}

func resourcesTableHasCompositeKey(ctx context.Context, conn *sql.Conn) (bool, error) {
	rows, err := conn.QueryContext(ctx, `PRAGMA table_info(resources)`)
	if err != nil {
		return false, fmt.Errorf("reading resources table info: %w", err)
	}
	defer rows.Close()

	pk := map[string]int{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pkOrder int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pkOrder); err != nil {
			return false, fmt.Errorf("scanning resources table info: %w", err)
		}
		pk[name] = pkOrder
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("reading resources table info rows: %w", err)
	}
	return pk["resource_type"] == 1 && pk["id"] == 2, nil
}

func rebuildResourcesFTS(ctx context.Context, conn *sql.Conn) error {
	rows, err := conn.QueryContext(ctx, `SELECT id, resource_type, data FROM resources`)
	if err != nil {
		return fmt.Errorf("querying resources: %w", err)
	}

	type resourceRow struct {
		id           string
		resourceType string
		data         string
	}
	var resources []resourceRow
	for rows.Next() {
		var r resourceRow
		if err := rows.Scan(&r.id, &r.resourceType, &r.data); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scanning resource: %w", err)
		}
		resources = append(resources, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("reading resource rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("closing resource rows: %w", err)
	}

	for _, r := range resources {
		if _, err := conn.ExecContext(ctx,
			`INSERT INTO resources_fts (rowid, id, resource_type, content) VALUES (?, ?, ?, ?)`,
			ftsRowID(r.resourceType, r.id), r.id, r.resourceType, searchableResourceContent(json.RawMessage(r.data)),
		); err != nil {
			return fmt.Errorf("indexing resource %s/%s: %w", r.resourceType, r.id, err)
		}
	}
	return nil
}

const (
	migrationLockTimeout    = 30 * time.Second
	migrationLockBackoffMin = 5 * time.Millisecond
	migrationLockBackoffMax = 100 * time.Millisecond
)

// withMigrationLock runs fn inside a BEGIN IMMEDIATE / COMMIT pair on
// conn, retrying both BEGIN and COMMIT on SQLITE_BUSY against the
// caller-provided deadline. Sharing the deadline with the pre-lock
// version read keeps total Open() latency bounded by a single budget.
// The real upper bound is deadline + one trailing backoff interval
// (≤100ms) + the driver's busy_timeout for the in-flight Exec, since
// the deadline is checked after each failed attempt rather than as a
// hard wall-clock cutoff. fn must use conn (not s.db) so its writes
// participate in the held transaction.
func withMigrationLock(ctx context.Context, conn *sql.Conn, deadline time.Time, fn func() error) error {
	if err := execWithBusyRetry(ctx, conn, "BEGIN IMMEDIATE", "begin migration transaction", deadline); err != nil {
		return err
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		// ROLLBACK uses context.Background() so caller-context cancellation
		// can't strand the connection in an open transaction. A failed
		// rollback is rare on local SQLite (broken file handle, fatal
		// driver error) but worth surfacing — silent swallow leaves a
		// pinned connection returned to the pool with state that will
		// confuse later queries.
		if _, rerr := conn.ExecContext(context.Background(), "ROLLBACK"); rerr != nil {
			fmt.Fprintf(os.Stderr, "warning: store migration rollback failed: %v\n", rerr)
		}
	}()

	if err := fn(); err != nil {
		return err
	}

	if err := execWithBusyRetry(ctx, conn, "COMMIT", "commit migration transaction", deadline); err != nil {
		return err
	}
	committed = true
	return nil
}

// execWithBusyRetry runs stmt on conn and retries on SQLITE_BUSY until
// deadline. It covers BEGIN IMMEDIATE and COMMIT contention;
// modernc.org/sqlite's busy_timeout does not reliably cover either when
// multiple connections race for the WAL write lock.
func execWithBusyRetry(ctx context.Context, conn *sql.Conn, stmt, label string, deadline time.Time) error {
	return retryOnBusy(ctx, deadline, label, func() error {
		_, err := conn.ExecContext(ctx, stmt)
		return err
	})
}

// retryOnBusy runs op and retries it on SQLITE_BUSY/LOCKED until
// deadline. The same retry shape covers Exec, Query, and any other
// SQLite call that can race the WAL writer lock — including the
// pre-lock user_version read, where the WAL initialization race on a
// fresh DB can BUSY a SELECT that should otherwise succeed under WAL
// reader/writer concurrency.
func retryOnBusy(ctx context.Context, deadline time.Time, label string, op func() error) error {
	backoff := migrationLockBackoffMin
	for {
		err := op()
		if err == nil {
			return nil
		}
		if !isSQLiteBusy(err) {
			return fmt.Errorf("%s: %w", label, err)
		}
		if time.Now().After(deadline) {
			// The label carries the operation context (e.g. "begin
			// migration transaction", "reading schema version") — we
			// don't hardcode "waiting for write lock" because pre-lock
			// reads also flow through this helper.
			return fmt.Errorf("%s: timed out after %s under SQLite contention: %w", label, migrationLockTimeout, err)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s: %w", label, ctx.Err())
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, migrationLockBackoffMax)
	}
}

// isSQLiteBusy reports whether err is a retryable SQLite lock condition.
// Covers both the file-level WAL writer race (SQLITE_BUSY / "database is
// locked") and the table-level shared-cache contention (SQLITE_LOCKED /
// "database table is locked"). The match is on the error string because
// modernc.org/sqlite does not export an error type the generated code
// can switch on without dragging the driver package into every store
// consumer.
func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "SQLITE_BUSY") ||
		strings.Contains(msg, "SQLITE_LOCKED") ||
		strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "database table is locked")
}

func (s *Store) upsertGenericResourceTx(tx *sql.Tx, resourceType, id string, data json.RawMessage) error {
	_, err := tx.Exec(
		`INSERT INTO resources (id, resource_type, data, synced_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(resource_type, id) DO UPDATE SET data = excluded.data, synced_at = excluded.synced_at, updated_at = excluded.updated_at`,
		id, resourceType, string(data), time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return err
	}

	ftsRowid := ftsRowID(resourceType, id)
	// Use explicit rowid for FTS5 compatibility with modernc.org/sqlite.
	// Standard DELETE WHERE column=? may not work on FTS5 virtual tables.
	if _, err = tx.Exec(`DELETE FROM resources_fts WHERE rowid = ?`, ftsRowid); err != nil {
		fmt.Fprintf(os.Stderr, "warning: FTS index cleanup failed: %v\n", err)
	}

	if _, err = tx.Exec(
		`INSERT INTO resources_fts (rowid, id, resource_type, content)
		 VALUES (?, ?, ?, ?)`,
		ftsRowid, id, resourceType, searchableResourceContent(data),
	); err != nil {
		// FTS insert failure is non-fatal
		fmt.Fprintf(os.Stderr, "warning: FTS index update failed: %v\n", err)
	}

	return nil
}

func (s *Store) Upsert(resourceType, id string, data json.RawMessage) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.upsertGenericResourceTx(tx, resourceType, id, data); err != nil {
		return err
	}

	return tx.Commit()
}

// Propagates sql.ErrNoRows on a miss so callers can distinguish absence from
// other scan errors via errors.Is.
func (s *Store) Get(resourceType, id string) (json.RawMessage, error) {
	var data string
	err := s.db.QueryRow(
		`SELECT data FROM resources WHERE resource_type = ? AND id = ?`,
		resourceType, id,
	).Scan(&data)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

// List returns resources of the given type. A positive limit caps the result
// count; zero or negative means no limit.
func (s *Store) List(resourceType string, limit int) ([]json.RawMessage, error) {
	query := `SELECT data FROM resources WHERE resource_type = ? ORDER BY updated_at DESC`
	args := []any{resourceType}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []json.RawMessage
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		results = append(results, json.RawMessage(data))
	}
	return results, rows.Err()
}

func (s *Store) Search(query string, limit int, resourceTypes ...string) ([]json.RawMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	matchQuery := FTSMatchQuery(query)
	if matchQuery == "" {
		return nil, nil
	}
	resourceType := ""
	if len(resourceTypes) > 0 {
		resourceType = strings.TrimSpace(resourceTypes[0])
	}
	if resourceType != "" {
		rows, err := s.db.Query(
			`SELECT r.data FROM resources r
			 JOIN resources_fts f ON r.id = f.id AND r.resource_type = f.resource_type
			 WHERE resources_fts MATCH ?
			 AND r.resource_type = ?
			 ORDER BY f.rank
			 LIMIT ?`,
			matchQuery, resourceType, limit,
		)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var results []json.RawMessage
		for rows.Next() {
			var data string
			if err := rows.Scan(&data); err != nil {
				return nil, err
			}
			results = append(results, json.RawMessage(data))
		}
		return results, rows.Err()
	}
	rows, err := s.db.Query(
		`SELECT r.data FROM resources r
		 JOIN resources_fts f ON r.id = f.id AND r.resource_type = f.resource_type
		 WHERE resources_fts MATCH ?
		 ORDER BY f.rank
		 LIMIT ?`,
		matchQuery, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []json.RawMessage
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		results = append(results, json.RawMessage(data))
	}
	return results, rows.Err()
}

func searchableResourceContent(data json.RawMessage) string {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return ""
	}
	var parts []string
	collectSearchableStrings(&parts, "", value)
	return strings.Join(parts, " ")
}

func collectSearchableStrings(parts *[]string, key string, value any) {
	switch v := value.(type) {
	case map[string]any:
		for childKey, child := range v {
			collectSearchableStrings(parts, childKey, child)
		}
	case []any:
		for _, child := range v {
			collectSearchableStrings(parts, key, child)
		}
	case string:
		if shouldIndexSearchString(key, v) {
			*parts = append(*parts, strings.TrimSpace(v))
		}
	}
}

func shouldIndexSearchString(key, value string) bool {
	s := strings.TrimSpace(value)
	if len(s) < 2 {
		return false
	}
	if isIdentifierKey(key) {
		return false
	}
	lower := strings.ToLower(s)
	switch {
	case IsUUID(s):
		return false
	case isoDatePattern.MatchString(s):
		return false
	case strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://"):
		return false
	}
	tokens := ftsQueryTokenRE.FindAllString(s, -1)
	return len(tokens) > 0
}

func isIdentifierKey(key string) bool {
	if key == "" {
		return false
	}
	lower := strings.ToLower(key)
	return lower == "id" ||
		lower == "uuid" ||
		strings.HasSuffix(lower, "_id") ||
		strings.HasSuffix(lower, "-id") ||
		strings.HasSuffix(key, "Id") ||
		strings.HasSuffix(key, "ID")
}

// FTSMatchQuery converts arbitrary text into a safe FTS5 MATCH expression.
func FTSMatchQuery(query string) string {
	tokens := ftsQueryTokenRE.FindAllString(query, -1)
	if len(tokens) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(tokens))
	for _, token := range tokens {
		quoted = append(quoted, `"`+token+`"`)
	}
	return strings.Join(quoted, " ")
}

func extractObjectID(obj map[string]any) string {
	for _, key := range []string{"id", "ID", "_id", "id_", "uuid", "slug", "name"} {
		if s := canonicalIDFromKey(obj, key); s != "" {
			return s
		}
	}
	return ""
}

// ftsRowID derives a deterministic rowid from a string ID for use with FTS5.
// Any change to this derivation requires a StoreSchemaVersion bump and a
// resources_fts rebuild migration for already-stamped databases.
// modernc.org/sqlite's FTS5 implementation may not support DELETE WHERE column=?
// on virtual tables, so we use explicit rowids and DELETE WHERE rowid=? instead.
func ftsRowID(scope, id string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(scope))
	_, _ = h.Write([]byte{0}) // separator so ("ab","c") != ("a","bc")
	_, _ = h.Write([]byte(id))
	return int64(h.Sum64() & 0x7FFFFFFFFFFFFFFF) // ensure positive
}

// LookupFieldValue resolves a field value from a JSON object map. A dotted
// key is walked as a path (`entityInfo.entityId`); each segment tries snake,
// camel, and Pascal spellings, then the Python-style trailing-underscore
// sibling (`id` → `id_`). Exported so the sync command's extractID and the
// upsert path resolve fields the same way — a divergence here produces
// silent drops on heterogeneous payloads. The PascalCase pass handles
// .NET-shaped responses (`Id`, `Name`, `OrderId`) without forcing each spec
// to declare casing.
func LookupFieldValue(obj map[string]any, snakeKey string) any {
	v, ok := lookupRawFieldValue(obj, snakeKey)
	if !ok {
		return nil
	}
	return sqliteFieldValue(v)
}

func lookupRawFieldValue(obj map[string]any, key string) (any, bool) {
	if obj == nil || key == "" {
		return nil, false
	}
	if strings.Contains(key, ".") {
		return lookupRawDottedFieldValue(obj, key)
	}
	return lookupRawFlatFieldValue(obj, key)
}

func lookupRawDottedFieldValue(obj map[string]any, path string) (any, bool) {
	if v, ok := lookupRawFlatFieldValue(obj, path); ok {
		return v, true
	}
	segments := strings.Split(path, ".")
	if len(segments) < 2 {
		return nil, false
	}
	current := obj
	for i, segment := range segments {
		if segment == "" {
			return nil, false
		}
		v, ok := lookupRawFlatFieldValue(current, segment)
		if !ok {
			return nil, false
		}
		if i == len(segments)-1 {
			return v, true
		}
		next, ok := v.(map[string]any)
		if !ok {
			return nil, false
		}
		current = next
	}
	return nil, false
}

func lookupRawFlatFieldValue(obj map[string]any, snakeKey string) (any, bool) {
	for _, key := range fieldKeySpellings(snakeKey) {
		if v, ok := obj[key]; ok {
			return v, true
		}
	}
	return nil, false
}

func fieldKeySpellings(snakeKey string) []string {
	seen := make(map[string]struct{}, 8)
	out := make([]string, 0, 8)
	add := func(s string) {
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}

	add(snakeKey)
	parts := strings.Split(snakeKey, "_")
	for i := 1; i < len(parts); i++ {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	add(strings.Join(parts, ""))
	if parts[0] != "" {
		add(strings.ToUpper(parts[0][:1]) + parts[0][1:] + strings.Join(parts[1:], ""))
	}
	n := len(out)
	for i := 0; i < n; i++ {
		if !strings.HasSuffix(out[i], "_") {
			add(out[i] + "_")
		}
	}
	return out
}

func sqliteFieldValue(v any) any {
	switch t := v.(type) {
	case nil, string, bool, int, int64, float64, []byte:
		return v
	case json.Number:
		return strings.TrimSpace(t.String())
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(data)
	}
}

// lookupFieldValue is kept as an unexported alias for in-package callers so
// the existing UpsertBatch code reads naturally without prefixing every call
// with the package name.
func lookupFieldValue(obj map[string]any, snakeKey string) any {
	return LookupFieldValue(obj, snakeKey)
}

// DecodeJSONObject decodes data into an object while preserving JSON numbers.
// Plain json.Unmarshal turns numbers into float64, and fmt on those values can
// render large integer IDs as scientific notation before they reach resources.id.
func DecodeJSONObject(data json.RawMessage) (map[string]any, error) {
	var obj map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&obj); err != nil {
		return nil, err
	}
	return obj, nil
}

// CanonicalResourceID is the identity invariant for generated stores: a value
// becomes resources.id only when it can stably distinguish a row. ResourceIDString
// will stringify zeros, timestamps, and booleans, and writing those keys
// silently collapses or duplicates records on the next sync.
func CanonicalResourceID(v any) string {
	switch v.(type) {
	case nil, bool:
		return ""
	}
	s := strings.TrimSpace(ResourceIDString(v))
	if unusableResourceID(s) {
		return ""
	}
	return s
}

func unusableResourceID(s string) bool {
	if s == "" || s == "<nil>" {
		return true
	}
	if isoDatePattern.MatchString(s) {
		return true
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && f == 0 {
		return true
	}
	return false
}

func canonicalIDFromKey(obj map[string]any, key string) string {
	if v, ok := lookupRawFieldValue(obj, key); ok {
		if s := CanonicalResourceID(v); s != "" {
			return s
		}
	}
	if obj == nil || strings.HasSuffix(key, "_") {
		return ""
	}
	v, ok := obj[key+"_"]
	if !ok {
		return ""
	}
	return CanonicalResourceID(v)
}

// ResourceIDString returns the stable text form used for resources.id.
func ResourceIDString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return extendedJSONIDString(t)
	case map[string]any:
		return extendedJSONIDMapString(t)
	case json.Number:
		return strings.TrimSpace(t.String())
	case float64:
		if math.IsNaN(t) || math.IsInf(t, 0) {
			return ""
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case float32:
		f := float64(t)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return ""
		}
		return strconv.FormatFloat(f, 'f', -1, 32)
	default:
		// fmt.Sprint on typed nil pointers returns "<nil>"; callers still guard
		// that sentinel so unresolved IDs do not become stored resource keys.
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func extendedJSONIDString(value string) string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "{") {
		return value
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(value), &object); err != nil {
		return value
	}
	if id := extendedJSONIDMapString(object); id != "" {
		return id
	}
	return value
}

func extendedJSONIDMapString(object map[string]any) string {
	for _, key := range []string{"$oid", "$numberLong", "$numberInt"} {
		if value, ok := object[key]; ok {
			return ResourceIDString(value)
		}
	}
	return ""
}

// upsertAdInsightTx writes the per-resource domain-table portion of a
// ad_insight upsert inside an existing transaction. The caller is
// responsible for the generic resources insert (via upsertGenericResourceTx)
// and for committing the tx. Splitting this out lets UpsertBatch dispatch
// domain inserts per item without opening a per-item transaction.
func (s *Store) upsertAdInsightTx(tx *sql.Tx, id string, obj map[string]any, data json.RawMessage) error {
	if _, err := tx.Exec(
		`INSERT INTO "ad_insight" ("id", "data", "synced_at", "currency", "is_impression_too_specific", "is_privacy_check_passed", "suggested_bid", "events_lost_to_bid", "events_lost_to_budget", "suggested_budget", "aar_opt_in_status", "recommendation_type", "ad_group_bid_landscape_type", "ad_group_id", "campaign_bid_landscape_type", "campaign_id", "keyword_id", "current_bid", "estimated_increase_in_clicks", "estimated_increase_in_cost", "estimated_increase_in_impressions", "match_type", "opportunity_key", "budget_type", "current_budget", "increase_in_clicks", "increase_in_impressions", "percentage_increase_in_clicks", "percentage_increase_in_impressions", "recommended_budget", "bid", "category_name", "coverage", "keyword", "category_id", "ad_group_name", "ad_impression_share", "competition", "relevance", "source", "entity_id", "entity_type", "kpi_type", "account_id", "current_clicks", "current_conversions", "current_cost", "current_impressions", "estimated_increase_in_conversions", "recommendation_hash", "recommendation_id", "type", "confidence_score", "suggested_keyword")
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT("id") DO UPDATE SET "data" = excluded."data", "synced_at" = excluded."synced_at", "currency" = excluded."currency", "is_impression_too_specific" = excluded."is_impression_too_specific", "is_privacy_check_passed" = excluded."is_privacy_check_passed", "suggested_bid" = excluded."suggested_bid", "events_lost_to_bid" = excluded."events_lost_to_bid", "events_lost_to_budget" = excluded."events_lost_to_budget", "suggested_budget" = excluded."suggested_budget", "aar_opt_in_status" = excluded."aar_opt_in_status", "recommendation_type" = excluded."recommendation_type", "ad_group_bid_landscape_type" = excluded."ad_group_bid_landscape_type", "ad_group_id" = excluded."ad_group_id", "campaign_bid_landscape_type" = excluded."campaign_bid_landscape_type", "campaign_id" = excluded."campaign_id", "keyword_id" = excluded."keyword_id", "current_bid" = excluded."current_bid", "estimated_increase_in_clicks" = excluded."estimated_increase_in_clicks", "estimated_increase_in_cost" = excluded."estimated_increase_in_cost", "estimated_increase_in_impressions" = excluded."estimated_increase_in_impressions", "match_type" = excluded."match_type", "opportunity_key" = excluded."opportunity_key", "budget_type" = excluded."budget_type", "current_budget" = excluded."current_budget", "increase_in_clicks" = excluded."increase_in_clicks", "increase_in_impressions" = excluded."increase_in_impressions", "percentage_increase_in_clicks" = excluded."percentage_increase_in_clicks", "percentage_increase_in_impressions" = excluded."percentage_increase_in_impressions", "recommended_budget" = excluded."recommended_budget", "bid" = excluded."bid", "category_name" = excluded."category_name", "coverage" = excluded."coverage", "keyword" = excluded."keyword", "category_id" = excluded."category_id", "ad_group_name" = excluded."ad_group_name", "ad_impression_share" = excluded."ad_impression_share", "competition" = excluded."competition", "relevance" = excluded."relevance", "source" = excluded."source", "entity_id" = excluded."entity_id", "entity_type" = excluded."entity_type", "kpi_type" = excluded."kpi_type", "account_id" = excluded."account_id", "current_clicks" = excluded."current_clicks", "current_conversions" = excluded."current_conversions", "current_cost" = excluded."current_cost", "current_impressions" = excluded."current_impressions", "estimated_increase_in_conversions" = excluded."estimated_increase_in_conversions", "recommendation_hash" = excluded."recommendation_hash", "recommendation_id" = excluded."recommendation_id", "type" = excluded."type", "confidence_score" = excluded."confidence_score", "suggested_keyword" = excluded."suggested_keyword"`,
		id,
		string(data),
		time.Now().UTC().Format(time.RFC3339),
		lookupFieldValue(obj, "currency"),
		lookupFieldValue(obj, "is_impression_too_specific"),
		lookupFieldValue(obj, "is_privacy_check_passed"),
		lookupFieldValue(obj, "suggested_bid"),
		lookupFieldValue(obj, "events_lost_to_bid"),
		lookupFieldValue(obj, "events_lost_to_budget"),
		lookupFieldValue(obj, "suggested_budget"),
		lookupFieldValue(obj, "aar_opt_in_status"),
		lookupFieldValue(obj, "recommendation_type"),
		lookupFieldValue(obj, "ad_group_bid_landscape_type"),
		lookupFieldValue(obj, "ad_group_id"),
		lookupFieldValue(obj, "campaign_bid_landscape_type"),
		lookupFieldValue(obj, "campaign_id"),
		lookupFieldValue(obj, "keyword_id"),
		lookupFieldValue(obj, "current_bid"),
		lookupFieldValue(obj, "estimated_increase_in_clicks"),
		lookupFieldValue(obj, "estimated_increase_in_cost"),
		lookupFieldValue(obj, "estimated_increase_in_impressions"),
		lookupFieldValue(obj, "match_type"),
		lookupFieldValue(obj, "opportunity_key"),
		lookupFieldValue(obj, "budget_type"),
		lookupFieldValue(obj, "current_budget"),
		lookupFieldValue(obj, "increase_in_clicks"),
		lookupFieldValue(obj, "increase_in_impressions"),
		lookupFieldValue(obj, "percentage_increase_in_clicks"),
		lookupFieldValue(obj, "percentage_increase_in_impressions"),
		lookupFieldValue(obj, "recommended_budget"),
		lookupFieldValue(obj, "bid"),
		lookupFieldValue(obj, "category_name"),
		lookupFieldValue(obj, "coverage"),
		lookupFieldValue(obj, "keyword"),
		lookupFieldValue(obj, "category_id"),
		lookupFieldValue(obj, "ad_group_name"),
		lookupFieldValue(obj, "ad_impression_share"),
		lookupFieldValue(obj, "competition"),
		lookupFieldValue(obj, "relevance"),
		lookupFieldValue(obj, "source"),
		lookupFieldValue(obj, "entity_id"),
		lookupFieldValue(obj, "entity_type"),
		lookupFieldValue(obj, "kpi_type"),
		lookupFieldValue(obj, "account_id"),
		lookupFieldValue(obj, "current_clicks"),
		lookupFieldValue(obj, "current_conversions"),
		lookupFieldValue(obj, "current_cost"),
		lookupFieldValue(obj, "current_impressions"),
		lookupFieldValue(obj, "estimated_increase_in_conversions"),
		lookupFieldValue(obj, "recommendation_hash"),
		lookupFieldValue(obj, "recommendation_id"),
		lookupFieldValue(obj, "type"),
		lookupFieldValue(obj, "confidence_score"),
		lookupFieldValue(obj, "suggested_keyword"),
	); err != nil {
		return fmt.Errorf("insert into ad_insight: %w", err)
	}

	return nil
}

// UpsertAdInsight inserts or updates a ad_insight record with domain-specific columns.
func (s *Store) UpsertAdInsight(data json.RawMessage) error {
	obj, err := DecodeJSONObject(data)
	if err != nil {
		return fmt.Errorf("unmarshaling ad_insight: %w", err)
	}

	id := ExtractResourceID("ad-insight", obj)
	if id == "" {
		return fmt.Errorf("missing id for ad_insight")
	}
	storageID := resourceStorageID("ad-insight", id, obj)

	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.upsertGenericResourceTx(tx, "ad-insight", storageID, data); err != nil {
		return err
	}
	if err := s.upsertAdInsightTx(tx, storageID, obj, data); err != nil {
		return err
	}

	return tx.Commit()
}

// upsertBulkTx writes the per-resource domain-table portion of a
// bulk upsert inside an existing transaction. The caller is
// responsible for the generic resources insert (via upsertGenericResourceTx)
// and for committing the tx. Splitting this out lets UpsertBatch dispatch
// domain inserts per item without opening a per-item transaction.
func (s *Store) upsertBulkTx(tx *sql.Tx, id string, obj map[string]any, data json.RawMessage) error {
	if _, err := tx.Exec(
		`INSERT INTO "bulk" ("id", "data", "synced_at", "download_request_id", "percent_complete", "request_status", "result_file_url", "request_id", "upload_url")
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT("id") DO UPDATE SET "data" = excluded."data", "synced_at" = excluded."synced_at", "download_request_id" = excluded."download_request_id", "percent_complete" = excluded."percent_complete", "request_status" = excluded."request_status", "result_file_url" = excluded."result_file_url", "request_id" = excluded."request_id", "upload_url" = excluded."upload_url"`,
		id,
		string(data),
		time.Now().UTC().Format(time.RFC3339),
		lookupFieldValue(obj, "download_request_id"),
		lookupFieldValue(obj, "percent_complete"),
		lookupFieldValue(obj, "request_status"),
		lookupFieldValue(obj, "result_file_url"),
		lookupFieldValue(obj, "request_id"),
		lookupFieldValue(obj, "upload_url"),
	); err != nil {
		return fmt.Errorf("insert into bulk: %w", err)
	}

	return nil
}

// UpsertBulk inserts or updates a bulk record with domain-specific columns.
func (s *Store) UpsertBulk(data json.RawMessage) error {
	obj, err := DecodeJSONObject(data)
	if err != nil {
		return fmt.Errorf("unmarshaling bulk: %w", err)
	}

	id := ExtractResourceID("bulk", obj)
	if id == "" {
		return fmt.Errorf("missing id for bulk")
	}
	storageID := resourceStorageID("bulk", id, obj)

	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.upsertGenericResourceTx(tx, "bulk", storageID, data); err != nil {
		return err
	}
	if err := s.upsertBulkTx(tx, storageID, obj, data); err != nil {
		return err
	}

	return tx.Commit()
}

// upsertCampaignManagementTx writes the per-resource domain-table portion of a
// campaign_management upsert inside an existing transaction. The caller is
// responsible for the generic resources insert (via upsertGenericResourceTx)
// and for committing the tx. Splitting this out lets UpsertBatch dispatch
// domain inserts per item without opening a per-item transaction.
func (s *Store) upsertCampaignManagementTx(tx *sql.Tx, id string, obj map[string]any, data json.RawMessage) error {
	if _, err := tx.Exec(
		`INSERT INTO "campaign_management" ("id", "data", "synced_at", "shared_entity_id", "image_url", "prompt_brand_warning", "account_id", "ad_group_type", "ad_schedule_use_searcher_time_zone", "audience_ads_bid_adjustment", "final_url_suffix", "language", "multimedia_ads_bid_adjustment", "name", "network", "privacy_status", "status", "tracking_url_template", "use_optimized_targeting", "use_predictive_targeting", "is_account_opt_out", "is_customer_opt_out", "is_customer_opt_out_of_everything", "justification", "asset_group_id", "asset_group_listing_type", "is_excluded", "listing_group_path", "parent_listing_group_id", "business_name", "campaign_id", "size", "bid_strategy_id", "bid_strategy_scope", "budget_id", "budget_type", "daily_budget", "experiment_id", "is_deal_campaign", "is_political", "sub_type", "time_zone", "use_campaign_level_dates", "aspect_ratio", "duration", "number_of_images", "number_of_logos", "number_of_text", "template_description", "template_id", "template_name", "template_preview_url", "template_thumbnail_url", "config_value", "currency_code", "operation", "value", "type", "file_import_upload_url", "file_url", "file_url_expiry_time_utc", "last_modified_time_utc", "error_log_url", "start_time_in_utc", "match_type", "text", "media_type", "additional_value", "failure_count", "success_count", "upload_date", "logo_url", "profile_id")
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT("id") DO UPDATE SET "data" = excluded."data", "synced_at" = excluded."synced_at", "shared_entity_id" = excluded."shared_entity_id", "image_url" = excluded."image_url", "prompt_brand_warning" = excluded."prompt_brand_warning", "account_id" = excluded."account_id", "ad_group_type" = excluded."ad_group_type", "ad_schedule_use_searcher_time_zone" = excluded."ad_schedule_use_searcher_time_zone", "audience_ads_bid_adjustment" = excluded."audience_ads_bid_adjustment", "final_url_suffix" = excluded."final_url_suffix", "language" = excluded."language", "multimedia_ads_bid_adjustment" = excluded."multimedia_ads_bid_adjustment", "name" = excluded."name", "network" = excluded."network", "privacy_status" = excluded."privacy_status", "status" = excluded."status", "tracking_url_template" = excluded."tracking_url_template", "use_optimized_targeting" = excluded."use_optimized_targeting", "use_predictive_targeting" = excluded."use_predictive_targeting", "is_account_opt_out" = excluded."is_account_opt_out", "is_customer_opt_out" = excluded."is_customer_opt_out", "is_customer_opt_out_of_everything" = excluded."is_customer_opt_out_of_everything", "justification" = excluded."justification", "asset_group_id" = excluded."asset_group_id", "asset_group_listing_type" = excluded."asset_group_listing_type", "is_excluded" = excluded."is_excluded", "listing_group_path" = excluded."listing_group_path", "parent_listing_group_id" = excluded."parent_listing_group_id", "business_name" = excluded."business_name", "campaign_id" = excluded."campaign_id", "size" = excluded."size", "bid_strategy_id" = excluded."bid_strategy_id", "bid_strategy_scope" = excluded."bid_strategy_scope", "budget_id" = excluded."budget_id", "budget_type" = excluded."budget_type", "daily_budget" = excluded."daily_budget", "experiment_id" = excluded."experiment_id", "is_deal_campaign" = excluded."is_deal_campaign", "is_political" = excluded."is_political", "sub_type" = excluded."sub_type", "time_zone" = excluded."time_zone", "use_campaign_level_dates" = excluded."use_campaign_level_dates", "aspect_ratio" = excluded."aspect_ratio", "duration" = excluded."duration", "number_of_images" = excluded."number_of_images", "number_of_logos" = excluded."number_of_logos", "number_of_text" = excluded."number_of_text", "template_description" = excluded."template_description", "template_id" = excluded."template_id", "template_name" = excluded."template_name", "template_preview_url" = excluded."template_preview_url", "template_thumbnail_url" = excluded."template_thumbnail_url", "config_value" = excluded."config_value", "currency_code" = excluded."currency_code", "operation" = excluded."operation", "value" = excluded."value", "type" = excluded."type", "file_import_upload_url" = excluded."file_import_upload_url", "file_url" = excluded."file_url", "file_url_expiry_time_utc" = excluded."file_url_expiry_time_utc", "last_modified_time_utc" = excluded."last_modified_time_utc", "error_log_url" = excluded."error_log_url", "start_time_in_utc" = excluded."start_time_in_utc", "match_type" = excluded."match_type", "text" = excluded."text", "media_type" = excluded."media_type", "additional_value" = excluded."additional_value", "failure_count" = excluded."failure_count", "success_count" = excluded."success_count", "upload_date" = excluded."upload_date", "logo_url" = excluded."logo_url", "profile_id" = excluded."profile_id"`,
		id,
		string(data),
		time.Now().UTC().Format(time.RFC3339),
		lookupFieldValue(obj, "shared_entity_id"),
		lookupFieldValue(obj, "image_url"),
		lookupFieldValue(obj, "prompt_brand_warning"),
		lookupFieldValue(obj, "account_id"),
		lookupFieldValue(obj, "ad_group_type"),
		lookupFieldValue(obj, "ad_schedule_use_searcher_time_zone"),
		lookupFieldValue(obj, "audience_ads_bid_adjustment"),
		lookupFieldValue(obj, "final_url_suffix"),
		lookupFieldValue(obj, "language"),
		lookupFieldValue(obj, "multimedia_ads_bid_adjustment"),
		lookupFieldValue(obj, "name"),
		lookupFieldValue(obj, "network"),
		lookupFieldValue(obj, "privacy_status"),
		lookupFieldValue(obj, "status"),
		lookupFieldValue(obj, "tracking_url_template"),
		lookupFieldValue(obj, "use_optimized_targeting"),
		lookupFieldValue(obj, "use_predictive_targeting"),
		lookupFieldValue(obj, "is_account_opt_out"),
		lookupFieldValue(obj, "is_customer_opt_out"),
		lookupFieldValue(obj, "is_customer_opt_out_of_everything"),
		lookupFieldValue(obj, "justification"),
		lookupFieldValue(obj, "asset_group_id"),
		lookupFieldValue(obj, "asset_group_listing_type"),
		lookupFieldValue(obj, "is_excluded"),
		lookupFieldValue(obj, "listing_group_path"),
		lookupFieldValue(obj, "parent_listing_group_id"),
		lookupFieldValue(obj, "business_name"),
		lookupFieldValue(obj, "campaign_id"),
		lookupFieldValue(obj, "size"),
		lookupFieldValue(obj, "bid_strategy_id"),
		lookupFieldValue(obj, "bid_strategy_scope"),
		lookupFieldValue(obj, "budget_id"),
		lookupFieldValue(obj, "budget_type"),
		lookupFieldValue(obj, "daily_budget"),
		lookupFieldValue(obj, "experiment_id"),
		lookupFieldValue(obj, "is_deal_campaign"),
		lookupFieldValue(obj, "is_political"),
		lookupFieldValue(obj, "sub_type"),
		lookupFieldValue(obj, "time_zone"),
		lookupFieldValue(obj, "use_campaign_level_dates"),
		lookupFieldValue(obj, "aspect_ratio"),
		lookupFieldValue(obj, "duration"),
		lookupFieldValue(obj, "number_of_images"),
		lookupFieldValue(obj, "number_of_logos"),
		lookupFieldValue(obj, "number_of_text"),
		lookupFieldValue(obj, "template_description"),
		lookupFieldValue(obj, "template_id"),
		lookupFieldValue(obj, "template_name"),
		lookupFieldValue(obj, "template_preview_url"),
		lookupFieldValue(obj, "template_thumbnail_url"),
		lookupFieldValue(obj, "config_value"),
		lookupFieldValue(obj, "currency_code"),
		lookupFieldValue(obj, "operation"),
		lookupFieldValue(obj, "value"),
		lookupFieldValue(obj, "type"),
		lookupFieldValue(obj, "file_import_upload_url"),
		lookupFieldValue(obj, "file_url"),
		lookupFieldValue(obj, "file_url_expiry_time_utc"),
		lookupFieldValue(obj, "last_modified_time_utc"),
		lookupFieldValue(obj, "error_log_url"),
		lookupFieldValue(obj, "start_time_in_utc"),
		lookupFieldValue(obj, "match_type"),
		lookupFieldValue(obj, "text"),
		lookupFieldValue(obj, "media_type"),
		lookupFieldValue(obj, "additional_value"),
		lookupFieldValue(obj, "failure_count"),
		lookupFieldValue(obj, "success_count"),
		lookupFieldValue(obj, "upload_date"),
		lookupFieldValue(obj, "logo_url"),
		lookupFieldValue(obj, "profile_id"),
	); err != nil {
		return fmt.Errorf("insert into campaign_management: %w", err)
	}

	return nil
}

// UpsertCampaignManagement inserts or updates a campaign_management record with domain-specific columns.
func (s *Store) UpsertCampaignManagement(data json.RawMessage) error {
	obj, err := DecodeJSONObject(data)
	if err != nil {
		return fmt.Errorf("unmarshaling campaign_management: %w", err)
	}

	id := ExtractResourceID("campaign-management", obj)
	if id == "" {
		return fmt.Errorf("missing id for campaign_management")
	}
	storageID := resourceStorageID("campaign-management", id, obj)

	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.upsertGenericResourceTx(tx, "campaign-management", storageID, data); err != nil {
		return err
	}
	if err := s.upsertCampaignManagementTx(tx, storageID, obj, data); err != nil {
		return err
	}

	return tx.Commit()
}

// upsertCustomerBillingTx writes the per-resource domain-table portion of a
// customer_billing upsert inside an existing transaction. The caller is
// responsible for the generic resources insert (via upsertGenericResourceTx)
// and for committing the tx. Splitting this out lets UpsertBatch dispatch
// domain inserts per item without opening a per-item transaction.
func (s *Store) upsertCustomerBillingTx(tx *sql.Tx, id string, obj map[string]any, data json.RawMessage) error {
	if _, err := tx.Exec(
		`INSERT INTO "customer_billing" ("id", "data", "synced_at", "create_time", "insertion_order_id", "amount", "number", "type", "account_id", "account_name", "account_number", "campaign_id", "currency_code", "customer_id", "document_date", "document_id", "document_number", "billing_group_id", "description", "name", "active_duration", "balance", "claim_date", "coupon_id", "coupon_type", "coupon_value", "end_date", "expiration_date", "feature_id", "percent_off", "redemption_date", "spend_to_threshold", "start_date", "status", "upfront_spending", "class_name", "coupon_code", "is_redeemed", "send_to_date", "send_to_email", "spend_threshold", "booking_country_code", "budget_remaining", "budget_remaining_percent", "budget_spent", "budget_spent_percent", "comment", "is_endless", "is_in_series", "is_unlimited", "last_modified_by_user_id", "last_modified_time", "notification_threshold", "purchase_order", "reference_id", "series_frequency_type", "series_name", "spend_cap_amount")
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT("id") DO UPDATE SET "data" = excluded."data", "synced_at" = excluded."synced_at", "create_time" = excluded."create_time", "insertion_order_id" = excluded."insertion_order_id", "amount" = excluded."amount", "number" = excluded."number", "type" = excluded."type", "account_id" = excluded."account_id", "account_name" = excluded."account_name", "account_number" = excluded."account_number", "campaign_id" = excluded."campaign_id", "currency_code" = excluded."currency_code", "customer_id" = excluded."customer_id", "document_date" = excluded."document_date", "document_id" = excluded."document_id", "document_number" = excluded."document_number", "billing_group_id" = excluded."billing_group_id", "description" = excluded."description", "name" = excluded."name", "active_duration" = excluded."active_duration", "balance" = excluded."balance", "claim_date" = excluded."claim_date", "coupon_id" = excluded."coupon_id", "coupon_type" = excluded."coupon_type", "coupon_value" = excluded."coupon_value", "end_date" = excluded."end_date", "expiration_date" = excluded."expiration_date", "feature_id" = excluded."feature_id", "percent_off" = excluded."percent_off", "redemption_date" = excluded."redemption_date", "spend_to_threshold" = excluded."spend_to_threshold", "start_date" = excluded."start_date", "status" = excluded."status", "upfront_spending" = excluded."upfront_spending", "class_name" = excluded."class_name", "coupon_code" = excluded."coupon_code", "is_redeemed" = excluded."is_redeemed", "send_to_date" = excluded."send_to_date", "send_to_email" = excluded."send_to_email", "spend_threshold" = excluded."spend_threshold", "booking_country_code" = excluded."booking_country_code", "budget_remaining" = excluded."budget_remaining", "budget_remaining_percent" = excluded."budget_remaining_percent", "budget_spent" = excluded."budget_spent", "budget_spent_percent" = excluded."budget_spent_percent", "comment" = excluded."comment", "is_endless" = excluded."is_endless", "is_in_series" = excluded."is_in_series", "is_unlimited" = excluded."is_unlimited", "last_modified_by_user_id" = excluded."last_modified_by_user_id", "last_modified_time" = excluded."last_modified_time", "notification_threshold" = excluded."notification_threshold", "purchase_order" = excluded."purchase_order", "reference_id" = excluded."reference_id", "series_frequency_type" = excluded."series_frequency_type", "series_name" = excluded."series_name", "spend_cap_amount" = excluded."spend_cap_amount"`,
		id,
		string(data),
		time.Now().UTC().Format(time.RFC3339),
		lookupFieldValue(obj, "create_time"),
		lookupFieldValue(obj, "insertion_order_id"),
		lookupFieldValue(obj, "amount"),
		lookupFieldValue(obj, "number"),
		lookupFieldValue(obj, "type"),
		lookupFieldValue(obj, "account_id"),
		lookupFieldValue(obj, "account_name"),
		lookupFieldValue(obj, "account_number"),
		lookupFieldValue(obj, "campaign_id"),
		lookupFieldValue(obj, "currency_code"),
		lookupFieldValue(obj, "customer_id"),
		lookupFieldValue(obj, "document_date"),
		lookupFieldValue(obj, "document_id"),
		lookupFieldValue(obj, "document_number"),
		lookupFieldValue(obj, "billing_group_id"),
		lookupFieldValue(obj, "description"),
		lookupFieldValue(obj, "name"),
		lookupFieldValue(obj, "active_duration"),
		lookupFieldValue(obj, "balance"),
		lookupFieldValue(obj, "claim_date"),
		lookupFieldValue(obj, "coupon_id"),
		lookupFieldValue(obj, "coupon_type"),
		lookupFieldValue(obj, "coupon_value"),
		lookupFieldValue(obj, "end_date"),
		lookupFieldValue(obj, "expiration_date"),
		lookupFieldValue(obj, "feature_id"),
		lookupFieldValue(obj, "percent_off"),
		lookupFieldValue(obj, "redemption_date"),
		lookupFieldValue(obj, "spend_to_threshold"),
		lookupFieldValue(obj, "start_date"),
		lookupFieldValue(obj, "status"),
		lookupFieldValue(obj, "upfront_spending"),
		lookupFieldValue(obj, "class_name"),
		lookupFieldValue(obj, "coupon_code"),
		lookupFieldValue(obj, "is_redeemed"),
		lookupFieldValue(obj, "send_to_date"),
		lookupFieldValue(obj, "send_to_email"),
		lookupFieldValue(obj, "spend_threshold"),
		lookupFieldValue(obj, "booking_country_code"),
		lookupFieldValue(obj, "budget_remaining"),
		lookupFieldValue(obj, "budget_remaining_percent"),
		lookupFieldValue(obj, "budget_spent"),
		lookupFieldValue(obj, "budget_spent_percent"),
		lookupFieldValue(obj, "comment"),
		lookupFieldValue(obj, "is_endless"),
		lookupFieldValue(obj, "is_in_series"),
		lookupFieldValue(obj, "is_unlimited"),
		lookupFieldValue(obj, "last_modified_by_user_id"),
		lookupFieldValue(obj, "last_modified_time"),
		lookupFieldValue(obj, "notification_threshold"),
		lookupFieldValue(obj, "purchase_order"),
		lookupFieldValue(obj, "reference_id"),
		lookupFieldValue(obj, "series_frequency_type"),
		lookupFieldValue(obj, "series_name"),
		lookupFieldValue(obj, "spend_cap_amount"),
	); err != nil {
		return fmt.Errorf("insert into customer_billing: %w", err)
	}

	return nil
}

// UpsertCustomerBilling inserts or updates a customer_billing record with domain-specific columns.
func (s *Store) UpsertCustomerBilling(data json.RawMessage) error {
	obj, err := DecodeJSONObject(data)
	if err != nil {
		return fmt.Errorf("unmarshaling customer_billing: %w", err)
	}

	id := ExtractResourceID("customer-billing", obj)
	if id == "" {
		return fmt.Errorf("missing id for customer_billing")
	}
	storageID := resourceStorageID("customer-billing", id, obj)

	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.upsertGenericResourceTx(tx, "customer-billing", storageID, data); err != nil {
		return err
	}
	if err := s.upsertCustomerBillingTx(tx, storageID, obj, data); err != nil {
		return err
	}

	return tx.Commit()
}

// upsertCustomerManagementTx writes the per-resource domain-table portion of a
// customer_management upsert inside an existing transaction. The caller is
// responsible for the generic resources insert (via upsertGenericResourceTx)
// and for committing the tx. Splitting this out lets UpsertBatch dispatch
// domain inserts per item without opening a per-item transaction.
func (s *Store) upsertCustomerManagementTx(tx *sql.Tx, id string, obj map[string]any, data json.RawMessage) error {
	if _, err := tx.Exec(
		`INSERT INTO "customer_management" ("id", "data", "synced_at", "account_id", "account_number", "create_time", "account_life_cycle_status", "name", "number", "pause_reason", "account_mode", "account_name", "customer_id", "customer_name", "valid_fields", "message", "severity", "title", "type_id", "customer_link_permission", "role_id", "user_name", "account_financial_status", "auto_tag_type", "back_up_payment_instrument_id", "bill_to_customer_id", "billing_threshold_amount", "currency_code", "language", "last_modified_by_user_id", "last_modified_time", "parent_customer_id", "payment_method_id", "payment_method_type", "primary_user_id", "sales_house_customer_id", "sold_to_payment_instrument_id", "time_stamp", "time_zone", "client_entity_customer_number", "client_entity_id", "client_entity_name", "client_entity_number", "inviter_email", "inviter_name", "inviter_phone", "is_bill_to_client", "last_modified_date_time", "managing_customer_id", "managing_customer_name", "managing_customer_number", "note", "start_date", "status", "suppress_notification", "timestamp", "type", "customer_financial_status", "customer_life_cycle_status", "industry", "market_country", "market_language", "service_level", "email", "expiration_date", "first_name", "last_name", "lcid", "user_invitation_id", "customer_number", "city_name", "country_code", "postal_code", "province_code", "province_name", "street_address", "street_address2")
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT("id") DO UPDATE SET "data" = excluded."data", "synced_at" = excluded."synced_at", "account_id" = excluded."account_id", "account_number" = excluded."account_number", "create_time" = excluded."create_time", "account_life_cycle_status" = excluded."account_life_cycle_status", "name" = excluded."name", "number" = excluded."number", "pause_reason" = excluded."pause_reason", "account_mode" = excluded."account_mode", "account_name" = excluded."account_name", "customer_id" = excluded."customer_id", "customer_name" = excluded."customer_name", "valid_fields" = excluded."valid_fields", "message" = excluded."message", "severity" = excluded."severity", "title" = excluded."title", "type_id" = excluded."type_id", "customer_link_permission" = excluded."customer_link_permission", "role_id" = excluded."role_id", "user_name" = excluded."user_name", "account_financial_status" = excluded."account_financial_status", "auto_tag_type" = excluded."auto_tag_type", "back_up_payment_instrument_id" = excluded."back_up_payment_instrument_id", "bill_to_customer_id" = excluded."bill_to_customer_id", "billing_threshold_amount" = excluded."billing_threshold_amount", "currency_code" = excluded."currency_code", "language" = excluded."language", "last_modified_by_user_id" = excluded."last_modified_by_user_id", "last_modified_time" = excluded."last_modified_time", "parent_customer_id" = excluded."parent_customer_id", "payment_method_id" = excluded."payment_method_id", "payment_method_type" = excluded."payment_method_type", "primary_user_id" = excluded."primary_user_id", "sales_house_customer_id" = excluded."sales_house_customer_id", "sold_to_payment_instrument_id" = excluded."sold_to_payment_instrument_id", "time_stamp" = excluded."time_stamp", "time_zone" = excluded."time_zone", "client_entity_customer_number" = excluded."client_entity_customer_number", "client_entity_id" = excluded."client_entity_id", "client_entity_name" = excluded."client_entity_name", "client_entity_number" = excluded."client_entity_number", "inviter_email" = excluded."inviter_email", "inviter_name" = excluded."inviter_name", "inviter_phone" = excluded."inviter_phone", "is_bill_to_client" = excluded."is_bill_to_client", "last_modified_date_time" = excluded."last_modified_date_time", "managing_customer_id" = excluded."managing_customer_id", "managing_customer_name" = excluded."managing_customer_name", "managing_customer_number" = excluded."managing_customer_number", "note" = excluded."note", "start_date" = excluded."start_date", "status" = excluded."status", "suppress_notification" = excluded."suppress_notification", "timestamp" = excluded."timestamp", "type" = excluded."type", "customer_financial_status" = excluded."customer_financial_status", "customer_life_cycle_status" = excluded."customer_life_cycle_status", "industry" = excluded."industry", "market_country" = excluded."market_country", "market_language" = excluded."market_language", "service_level" = excluded."service_level", "email" = excluded."email", "expiration_date" = excluded."expiration_date", "first_name" = excluded."first_name", "last_name" = excluded."last_name", "lcid" = excluded."lcid", "user_invitation_id" = excluded."user_invitation_id", "customer_number" = excluded."customer_number", "city_name" = excluded."city_name", "country_code" = excluded."country_code", "postal_code" = excluded."postal_code", "province_code" = excluded."province_code", "province_name" = excluded."province_name", "street_address" = excluded."street_address", "street_address2" = excluded."street_address2"`,
		id,
		string(data),
		time.Now().UTC().Format(time.RFC3339),
		lookupFieldValue(obj, "account_id"),
		lookupFieldValue(obj, "account_number"),
		lookupFieldValue(obj, "create_time"),
		lookupFieldValue(obj, "account_life_cycle_status"),
		lookupFieldValue(obj, "name"),
		lookupFieldValue(obj, "number"),
		lookupFieldValue(obj, "pause_reason"),
		lookupFieldValue(obj, "account_mode"),
		lookupFieldValue(obj, "account_name"),
		lookupFieldValue(obj, "customer_id"),
		lookupFieldValue(obj, "customer_name"),
		lookupFieldValue(obj, "valid_fields"),
		lookupFieldValue(obj, "message"),
		lookupFieldValue(obj, "severity"),
		lookupFieldValue(obj, "title"),
		lookupFieldValue(obj, "type_id"),
		lookupFieldValue(obj, "customer_link_permission"),
		lookupFieldValue(obj, "role_id"),
		lookupFieldValue(obj, "user_name"),
		lookupFieldValue(obj, "account_financial_status"),
		lookupFieldValue(obj, "auto_tag_type"),
		lookupFieldValue(obj, "back_up_payment_instrument_id"),
		lookupFieldValue(obj, "bill_to_customer_id"),
		lookupFieldValue(obj, "billing_threshold_amount"),
		lookupFieldValue(obj, "currency_code"),
		lookupFieldValue(obj, "language"),
		lookupFieldValue(obj, "last_modified_by_user_id"),
		lookupFieldValue(obj, "last_modified_time"),
		lookupFieldValue(obj, "parent_customer_id"),
		lookupFieldValue(obj, "payment_method_id"),
		lookupFieldValue(obj, "payment_method_type"),
		lookupFieldValue(obj, "primary_user_id"),
		lookupFieldValue(obj, "sales_house_customer_id"),
		lookupFieldValue(obj, "sold_to_payment_instrument_id"),
		lookupFieldValue(obj, "time_stamp"),
		lookupFieldValue(obj, "time_zone"),
		lookupFieldValue(obj, "client_entity_customer_number"),
		lookupFieldValue(obj, "client_entity_id"),
		lookupFieldValue(obj, "client_entity_name"),
		lookupFieldValue(obj, "client_entity_number"),
		lookupFieldValue(obj, "inviter_email"),
		lookupFieldValue(obj, "inviter_name"),
		lookupFieldValue(obj, "inviter_phone"),
		lookupFieldValue(obj, "is_bill_to_client"),
		lookupFieldValue(obj, "last_modified_date_time"),
		lookupFieldValue(obj, "managing_customer_id"),
		lookupFieldValue(obj, "managing_customer_name"),
		lookupFieldValue(obj, "managing_customer_number"),
		lookupFieldValue(obj, "note"),
		lookupFieldValue(obj, "start_date"),
		lookupFieldValue(obj, "status"),
		lookupFieldValue(obj, "suppress_notification"),
		lookupFieldValue(obj, "timestamp"),
		lookupFieldValue(obj, "type"),
		lookupFieldValue(obj, "customer_financial_status"),
		lookupFieldValue(obj, "customer_life_cycle_status"),
		lookupFieldValue(obj, "industry"),
		lookupFieldValue(obj, "market_country"),
		lookupFieldValue(obj, "market_language"),
		lookupFieldValue(obj, "service_level"),
		lookupFieldValue(obj, "email"),
		lookupFieldValue(obj, "expiration_date"),
		lookupFieldValue(obj, "first_name"),
		lookupFieldValue(obj, "last_name"),
		lookupFieldValue(obj, "lcid"),
		lookupFieldValue(obj, "user_invitation_id"),
		lookupFieldValue(obj, "customer_number"),
		lookupFieldValue(obj, "city_name"),
		lookupFieldValue(obj, "country_code"),
		lookupFieldValue(obj, "postal_code"),
		lookupFieldValue(obj, "province_code"),
		lookupFieldValue(obj, "province_name"),
		lookupFieldValue(obj, "street_address"),
		lookupFieldValue(obj, "street_address2"),
	); err != nil {
		return fmt.Errorf("insert into customer_management: %w", err)
	}

	return nil
}

// UpsertCustomerManagement inserts or updates a customer_management record with domain-specific columns.
func (s *Store) UpsertCustomerManagement(data json.RawMessage) error {
	obj, err := DecodeJSONObject(data)
	if err != nil {
		return fmt.Errorf("unmarshaling customer_management: %w", err)
	}

	id := ExtractResourceID("customer-management", obj)
	if id == "" {
		return fmt.Errorf("missing id for customer_management")
	}
	storageID := resourceStorageID("customer-management", id, obj)

	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.upsertGenericResourceTx(tx, "customer-management", storageID, data); err != nil {
		return err
	}
	if err := s.upsertCustomerManagementTx(tx, storageID, obj, data); err != nil {
		return err
	}

	return tx.Commit()
}

// upsertReportingTx writes the per-resource domain-table portion of a
// reporting upsert inside an existing transaction. The caller is
// responsible for the generic resources insert (via upsertGenericResourceTx)
// and for committing the tx. Splitting this out lets UpsertBatch dispatch
// domain inserts per item without opening a per-item transaction.
func (s *Store) upsertReportingTx(tx *sql.Tx, id string, obj map[string]any, data json.RawMessage) error {
	if _, err := tx.Exec(
		`INSERT INTO "reporting" ("id", "data", "synced_at", "report_request_id")
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT("id") DO UPDATE SET "data" = excluded."data", "synced_at" = excluded."synced_at", "report_request_id" = excluded."report_request_id"`,
		id,
		string(data),
		time.Now().UTC().Format(time.RFC3339),
		lookupFieldValue(obj, "report_request_id"),
	); err != nil {
		return fmt.Errorf("insert into reporting: %w", err)
	}

	return nil
}

// UpsertReporting inserts or updates a reporting record with domain-specific columns.
func (s *Store) UpsertReporting(data json.RawMessage) error {
	obj, err := DecodeJSONObject(data)
	if err != nil {
		return fmt.Errorf("unmarshaling reporting: %w", err)
	}

	id := ExtractResourceID("reporting", obj)
	if id == "" {
		return fmt.Errorf("missing id for reporting")
	}
	storageID := resourceStorageID("reporting", id, obj)

	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.upsertGenericResourceTx(tx, "reporting", storageID, data); err != nil {
		return err
	}
	if err := s.upsertReportingTx(tx, storageID, obj, data); err != nil {
		return err
	}

	return tx.Commit()
}

// resourceIDFieldOverrides projects per-resource IDField (set by the profiler
// from x-resource-id or response-schema fallback) into a runtime lookup map.
// UpsertBatch consults this first so the templated path wins over the
// generic fallback list. Empty when no resource declared an override; the
// runtime fallback list still applies.
//
// Includes both flat resources and dependent (parent-child) resources so a
// child path-item annotated with x-resource-id resolves the same as a flat
// path-item.
var resourceIDFieldOverrides = map[string]string{}

// Generic ID fields are split around the resource-specific suffix probe.
// Stable vendor identifiers win first; then fields derived from the resource
// name (accountId, workspaceId); descriptive fallbacks are last. Keeping name
// ahead of the resource-specific probe silently keys rows by display labels.
// `id_` is the Python-style trailing-underscore sibling of `id`; LookupFieldValue
// also probes that spelling for every other key in this list.
var genericIDFieldFallbacks = []string{"id", "ID", "_id", "id_", "gid", "sid", "uid", "uuid", "guid", "api_id"}
var genericDescriptiveIDFieldFallbacks = []string{"name", "slug", "key", "code"}

// resourceIDBaseOverrides preserves the complete final collection name for
// composed dependents whose child segment is itself multiword.
var resourceIDBaseOverrides = map[string]string{}

// resourceParentKeyColumns identifies generated dependent resources whose
// local mirror rows need the parent context in the storage key. Without this,
// many-to-many sub-collections collapse every parent association onto the
// child's bare id and silently keep only the last synced parent.
var resourceParentKeyColumns = map[string][]string{}

// ExtractResourceID resolves the bare resource id field that UpsertBatch
// extracts from a resource item. For dependent resource types, UpsertBatch
// derives the actual storage key by combining this id with the parent value;
// use resourceStorageID if you need the key as it appears in the database.
// Callers that need to gate best-effort writes can use this to avoid passing
// non-entity envelopes into the batch path.
func ExtractResourceID(resourceType string, obj map[string]any) string {
	if override, ok := resourceIDFieldOverrides[resourceType]; ok && override != "" {
		if s := canonicalIDFromKey(obj, override); s != "" {
			return s
		}
	}
	for _, key := range genericIDFieldFallbacks {
		if s := canonicalIDFromKey(obj, key); s != "" {
			return s
		}
	}
	if s := suffixIDFieldFallback(resourceType, obj); s != "" {
		return s
	}
	for _, key := range genericDescriptiveIDFieldFallbacks {
		if s := canonicalIDFromKey(obj, key); s != "" {
			return s
		}
	}
	return ""
}

// suffixIDFieldFallback resolves an id-less resource that keys on its own
// "<name>_code" / "<name>_id" / "<name>_key" / "<name>_slug" field (e.g. the
// "currencies" resource keying on "currency_code" — see #2327). It is scoped to
// the resource's OWN name so a foreign key like account_id/parent_id is never
// promoted to the primary key, and it walks the same key spellings as
// LookupFieldValue in a fixed suffix order so the chosen id is deterministic.
func suffixIDFieldFallback(resourceType string, obj map[string]any) string {
	for _, base := range resourceIDBaseNames(resourceType) {
		for _, suffix := range []string{"_id", "_code", "_key", "_slug"} {
			if s := canonicalScalarIDFromKey(obj, base+suffix); s != "" {
				return s
			}
		}
		camelBase := lowerCamelResourceIDBase(base)
		for _, suffix := range []string{"Id", "Code", "Key", "Slug"} {
			if s := canonicalScalarIDFromKey(obj, camelBase+suffix); s != "" {
				return s
			}
		}
	}
	return ""
}

func canonicalScalarIDFromKey(obj map[string]any, key string) string {
	v, ok := lookupRawFieldValue(obj, key)
	if !ok || scalarIDString(v) == "" {
		return ""
	}
	return CanonicalResourceID(v)
}

// resourceIDBaseNames returns lowercase candidate singular/plural stems of a
// resource name to build "<base>_id"-style key probes from (e.g. "currencies"
// -> ["currencies","currency"]). Composed dependent names also probe their
// final segment ("containers_workspaces" -> "workspaces","workspace"), which
// is the child entity's own ID convention. OpenAPI-/path-derived names can
// carry a leading verb token ("get-currencies"), so the same probes are also
// attempted on the de-verbed stem.
func resourceIDBaseNames(resourceType string) []string {
	r := strings.ToLower(strings.TrimSpace(resourceType))
	if r == "" {
		return nil
	}
	var stems []string
	addStem := func(stem string) {
		if stem == "" {
			return
		}
		for _, existing := range stems {
			if existing == stem {
				return
			}
		}
		stems = append(stems, stem)
	}
	addStem(resourceIDBaseOverrides[r])
	addStem(r)
	if d := stripLeadingResourceVerb(r); d != "" && d != r {
		addStem(d)
	}
	var bases []string
	seen := map[string]bool{}
	add := func(s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			bases = append(bases, s)
		}
	}
	for _, stem := range stems {
		add(stem)
		add(depluralizeResourceStem(stem))
		if i := strings.LastIndexAny(stem, "_-"); i >= 0 && i+1 < len(stem) {
			leaf := stem[i+1:]
			add(leaf)
			add(depluralizeResourceStem(leaf))
		}
	}
	return bases
}

func stripLeadingResourceVerb(r string) string {
	for _, verb := range []string{"get", "list", "fetch", "find", "retrieve", "read", "show", "all"} {
		for _, sep := range []string{"-", "_"} {
			prefix := verb + sep
			if strings.HasPrefix(r, prefix) && len(r) > len(prefix) {
				return r[len(prefix):]
			}
		}
	}
	return ""
}

func depluralizeResourceStem(r string) string {
	switch {
	case strings.HasSuffix(r, "ies") && len(r) > 3:
		return strings.TrimSuffix(r, "ies") + "y" // currencies -> currency
	// Plurals formed by adding "es" to a base ending in s/x/z/ch/sh. The
	// double-s "sses" guard (not bare "ses") keeps soft-e plurals — where the
	// singular already ends in a silent "e" (cases, databases, licenses,
	// purchases) — out of this branch; they fall through to the "-s" case below
	// (cases -> case, not cas). Trade-off: a genuine "-es" plural of an s-ending
	// singular (buses, statuses) depluralizes imperfectly, but those are rare as
	// resource names and this stem only feeds best-effort id-field probing.
	case strings.HasSuffix(r, "sses") || strings.HasSuffix(r, "xes") ||
		strings.HasSuffix(r, "zes") || strings.HasSuffix(r, "ches") ||
		strings.HasSuffix(r, "shes"):
		return strings.TrimSuffix(r, "es") // classes -> class, boxes -> box, dishes -> dish
	case strings.HasSuffix(r, "s") && !strings.HasSuffix(r, "ss") && len(r) > 1:
		return strings.TrimSuffix(r, "s") // languages -> language, cases -> case
	}
	return r
}

func lowerCamelResourceIDBase(base string) string {
	parts := strings.FieldsFunc(base, func(r rune) bool {
		return r == '_' || r == '-'
	})
	if len(parts) == 0 {
		return base
	}
	for i := range parts {
		if i == 0 {
			parts[i] = strings.ToLower(parts[i])
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
	}
	return strings.Join(parts, "")
}

func scalarIDString(value any) string {
	switch value.(type) {
	case string, bool, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, json.Number, []byte:
		return ResourceIDString(value)
	default:
		return ""
	}
}

func resourceStorageID(resourceType, id string, obj map[string]any) string {
	for _, parentKey := range resourceParentKeyColumns[resourceType] {
		parentValue := ResourceIDString(lookupFieldValue(obj, parentKey))
		if parentValue != "" && parentValue != "<nil>" {
			return id + string([]byte{0}) + parentValue
		}
	}
	return id
}

// BareResourceID strips the NUL-delimited parent suffix that resourceStorageID
// appends to dependent resource types, returning the bare entity id. ListIDs
// returns composite keys for parent-keyed resources, so callers comparing those
// ids against bare API ids must run them through this first. For non-composite
// ids it returns the input unchanged, so it is safe to apply to every id.
func BareResourceID(storageID string) string {
	if i := strings.IndexByte(storageID, 0); i >= 0 {
		return storageID[:i]
	}
	return storageID
}

// childScopeColumnSources maps a typed child table's path-placeholder scope
// column (the FK the dependent sync injects per item, e.g. "projects_id") to
// the singular parent-reference field the API body carries natively (e.g.
// "project"). deriveScopeColumns consults this so write-through cache paths —
// which pass RAW API items to UpsertBatch and never carry the path-injected
// scope column — still satisfy the typed table's NOT NULL scope column instead
// of stranding the row in generic resources.
var childScopeColumnSources = map[string]string{}

// deriveScopeColumns backfills a typed child table's scope column from the
// item's own parent reference when path injection is absent. A value already
// present (valid injection) is never overwritten.
func deriveScopeColumns(obj map[string]any) {
	for scopeKey, sourceKey := range childScopeColumnSources {
		if v := lookupFieldValue(obj, scopeKey); v != nil {
			if s, ok := v.(string); !ok || s != "" {
				continue // path injection already supplied a usable value
			}
		}
		src := lookupFieldValue(obj, sourceKey)
		if src == nil {
			continue
		}
		if s, ok := src.(string); ok && s == "" {
			continue
		}
		obj[scopeKey] = src
	}
}

// UpsertBatch inserts or replaces multiple records in a single transaction.
// The detailed variant also reports typed-table projection failures so sync can
// treat a generic-only write as an incomplete local mirror.
//
// For resource types that have a domain-specific typed table, the per-item
// generic insert is followed by a dispatch to the matching upsert<Pascal>Tx
// inside the same transaction. Without that dispatch, paginated syncs would
// only populate the generic resources table — typed tables (and indexed
// columns like parent_id added by dependent-resource sync) would stay empty.
//
// Each typed-table dispatch runs inside a per-item SAVEPOINT so a constraint
// failure in the typed insert (e.g. NOT NULL parent FK when the generator
// didn't populate the parent path placeholder) rolls back only that typed
// upsert. The generic resources row inserted just above it survives the
// rollback, so successful API fetches never strand in memory because one
// downstream typed table is misconfigured.
func (s *Store) UpsertBatch(resourceType string, items []json.RawMessage) (int, int, error) {
	stored, extractFailures, _, err := s.UpsertBatchDetailed(resourceType, items)
	return stored, extractFailures, err
}

func (s *Store) UpsertBatchDetailed(resourceType string, items []json.RawMessage) (int, int, int, error) {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("starting batch transaction: %w", err)
	}
	defer tx.Rollback()

	var stored, skippedCount, extractFailures, typedFailures int
	for i, item := range items {
		obj, err := DecodeJSONObject(item)
		if err != nil {
			skippedCount++
			continue
		}
		// Templated IDField wins; generic fallback list runs second when
		// the override is empty OR the override field is absent on this
		// particular item (response shape mismatches happen even when the
		// spec declares x-resource-id).
		id := ExtractResourceID(resourceType, obj)
		if id == "" {
			if keyObj, rowObj, rowItem, ok := unwrapIDBearingEnvelopeItem(resourceType, item, obj); ok {
				id = ExtractResourceID(resourceType, keyObj)
				obj = rowObj
				item = rowItem
			}
		}
		if id == "" {
			skippedCount++
			extractFailures++
			continue
		}
		storageID := resourceStorageID(resourceType, id, obj)

		if err := s.upsertGenericResourceTx(tx, resourceType, storageID, item); err != nil {
			// Return the running stored count rather than zero so callers
			// inspecting partial progress on failure see what already
			// landed in earlier loop iterations.
			return stored, extractFailures, typedFailures, fmt.Errorf("upserting %s/%s: %w", resourceType, storageID, err)
		}
		stored++

		// Backfill the typed child table's NOT NULL scope column from the item's
		// own parent reference when the dependent-sync path injection is absent
		// (write-through cache feeds RAW API items here).
		deriveScopeColumns(obj)

		savepoint := fmt.Sprintf("pp_typed_%d", i)
		if _, err := tx.Exec("SAVEPOINT " + savepoint); err != nil {
			return stored, extractFailures, typedFailures, fmt.Errorf("savepoint begin for %s/%s: %w", resourceType, storageID, err)
		}

		var typedErr error
		switch resourceType {
		case "ad-insight":
			typedErr = s.upsertAdInsightTx(tx, storageID, obj, item)
		case "bulk":
			typedErr = s.upsertBulkTx(tx, storageID, obj, item)
		case "campaign-management":
			typedErr = s.upsertCampaignManagementTx(tx, storageID, obj, item)
		case "customer-billing":
			typedErr = s.upsertCustomerBillingTx(tx, storageID, obj, item)
		case "customer-management":
			typedErr = s.upsertCustomerManagementTx(tx, storageID, obj, item)
		case "reporting":
			typedErr = s.upsertReportingTx(tx, storageID, obj, item)
		}

		if typedErr != nil {
			if _, rbErr := tx.Exec("ROLLBACK TO SAVEPOINT " + savepoint); rbErr != nil {
				return stored, extractFailures, typedFailures, fmt.Errorf("rollback to savepoint for %s/%s (typed err: %v): %w", resourceType, storageID, typedErr, rbErr)
			}
			if _, relErr := tx.Exec("RELEASE SAVEPOINT " + savepoint); relErr != nil {
				return stored, extractFailures, typedFailures, fmt.Errorf("release savepoint after rollback for %s/%s: %w", resourceType, storageID, relErr)
			}
			typedFailures++
			continue
		}
		if _, err := tx.Exec("RELEASE SAVEPOINT " + savepoint); err != nil {
			return stored, extractFailures, typedFailures, fmt.Errorf("release savepoint for %s/%s: %w", resourceType, storageID, err)
		}
	}

	// Warn when every decoded item in a batch lacks an extractable ID — this
	// likely means the API uses a primary key field we don't recognize yet.
	// Partial misses still surface through extractFailures so sync can emit
	// a structured primary_key_unresolved anomaly without spamming stderr for
	// write-through cache batches that did persist useful rows.
	if extractFailures > 0 && stored == 0 && len(items) > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d/%d %s items returned but not cached locally (no extractable ID field; offline lookup against these rows will be incomplete; live queries unaffected)\n", skippedCount, len(items), resourceType)
	}
	// Surface typed-table failures without aborting the batch. Generic rows
	// already committed; only the typed projection failed.
	if typedFailures > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d/%d %s items: typed-table upsert failed; generic resources rows preserved\n", typedFailures, len(items), resourceType)
	}

	if err := tx.Commit(); err != nil {
		return 0, extractFailures, typedFailures, err
	}
	return stored, extractFailures, typedFailures, nil
}

// Multi-field wrappers keep their outer row because scalar siblings may be
// resource data; true single-field envelopes unwrap to the inner object.
func unwrapIDBearingEnvelopeItem(resourceType string, item json.RawMessage, obj map[string]any) (map[string]any, map[string]any, json.RawMessage, bool) {
	var candidate map[string]any
	candidateKey := ""
	candidates := 0
	for key, value := range obj {
		inner, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if ExtractResourceID(resourceType, inner) != "" {
			candidate = inner
			candidateKey = key
			candidates++
		}
	}
	if candidates != 1 || candidate == nil || candidateKey == "" {
		return nil, nil, nil, false
	}
	if len(obj) != 1 {
		return candidate, obj, item, true
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(item, &raw); err != nil {
		return nil, nil, nil, false
	}
	data, ok := raw[candidateKey]
	if !ok {
		return nil, nil, nil, false
	}
	return candidate, candidate, data, true
}

// SearchCampaignManagement searches the campaign_management_fts index with optional filters.
func (s *Store) SearchCampaignManagement(query string, limit int) ([]json.RawMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	matchQuery := FTSMatchQuery(query)
	if matchQuery == "" {
		return nil, nil
	}
	rows, err := s.db.Query(
		`SELECT t.data FROM "campaign_management" t
		 JOIN "campaign_management_fts" ON "campaign_management_fts".rowid = t.rowid
		 WHERE "campaign_management_fts" MATCH ?
		 ORDER BY "campaign_management_fts".rank LIMIT ?`,
		matchQuery, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []json.RawMessage
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		results = append(results, json.RawMessage(data))
	}
	return results, rows.Err()
}

// SearchCustomerBilling searches the customer_billing_fts index with optional filters.
func (s *Store) SearchCustomerBilling(query string, limit int) ([]json.RawMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	matchQuery := FTSMatchQuery(query)
	if matchQuery == "" {
		return nil, nil
	}
	rows, err := s.db.Query(
		`SELECT t.data FROM "customer_billing" t
		 JOIN "customer_billing_fts" ON "customer_billing_fts".rowid = t.rowid
		 WHERE "customer_billing_fts" MATCH ?
		 ORDER BY "customer_billing_fts".rank LIMIT ?`,
		matchQuery, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []json.RawMessage
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		results = append(results, json.RawMessage(data))
	}
	return results, rows.Err()
}

// SearchCustomerManagement searches the customer_management_fts index with optional filters.
func (s *Store) SearchCustomerManagement(query string, limit int) ([]json.RawMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	matchQuery := FTSMatchQuery(query)
	if matchQuery == "" {
		return nil, nil
	}
	rows, err := s.db.Query(
		`SELECT t.data FROM "customer_management" t
		 JOIN "customer_management_fts" ON "customer_management_fts".rowid = t.rowid
		 WHERE "customer_management_fts" MATCH ?
		 ORDER BY "customer_management_fts".rank LIMIT ?`,
		matchQuery, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []json.RawMessage
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		results = append(results, json.RawMessage(data))
	}
	return results, rows.Err()
}

func (s *Store) SaveSyncState(resourceType, cursor string, count int) error {
	return s.SaveSyncStateAt(resourceType, cursor, count, time.Now().UTC())
}

// SaveSyncStateAt stores both pagination progress and the incremental
// watermark represented by at. Callers use this when the watermark belongs to
// the data just fetched rather than to the instant the checkpoint is written.
func (s *Store) SaveSyncStateAt(resourceType, cursor string, count int, at time.Time) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	_, err := s.db.Exec(
		`INSERT INTO sync_state (resource_type, last_cursor, last_synced_at, total_count)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(resource_type) DO UPDATE SET last_cursor = excluded.last_cursor,
		 last_synced_at = excluded.last_synced_at, total_count = excluded.total_count`,
		resourceType, cursor, at.UTC().Format(time.RFC3339), count,
	)
	return err
}

// SaveSyncProgress stores pagination progress without changing the
// incremental watermark. A new row gets a parseable zero timestamp so
// GetSyncState can scan it into time.Time without a NULL conversion error.
func (s *Store) SaveSyncProgress(resourceType, cursor string, count int) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	_, err := s.db.Exec(
		`INSERT INTO sync_state (resource_type, last_cursor, last_synced_at, total_count)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(resource_type) DO UPDATE SET last_cursor = excluded.last_cursor,
		 total_count = excluded.total_count`,
		resourceType, cursor, time.Time{}.UTC().Format(time.RFC3339), count,
	)
	return err
}

func (s *Store) GetSyncState(resourceType string) (cursor string, lastSynced time.Time, count int, err error) {
	err = s.db.QueryRow(
		`SELECT last_cursor, last_synced_at, total_count FROM sync_state WHERE resource_type = ?`,
		resourceType,
	).Scan(&cursor, &lastSynced, &count)
	if err == sql.ErrNoRows {
		return "", time.Time{}, 0, nil
	}
	return
}

// SaveSyncCursor stores the pagination cursor for a resource type.
func (s *Store) SaveSyncCursor(resourceType, cursor string) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT INTO sync_state (resource_type, last_cursor, last_synced_at, total_count)
		 VALUES (?, ?, ?, 0)
		 ON CONFLICT(resource_type) DO UPDATE SET last_cursor = ?, last_synced_at = ?`,
		resourceType, cursor, now, cursor, now,
	)
	return err
}

// GetSyncCursor returns the last pagination cursor for a resource type.
func (s *Store) GetSyncCursor(resourceType string) string {
	var cursor sql.NullString
	s.db.QueryRow("SELECT last_cursor FROM sync_state WHERE resource_type = ?", resourceType).Scan(&cursor)
	if cursor.Valid {
		return cursor.String
	}
	return ""
}

// ListIDs returns all IDs from a resource's domain table, or from the generic
// resources table if no domain table exists. Used by dependent sync to iterate parents.
// For parent-keyed resource types these are composite storage keys; run them
// through BareResourceID before comparing against bare API ids.
//
// resourceType is never interpolated into SQL directly. We resolve it to a real
// table name via a parameterized sqlite_master lookup; only that trusted name is
// substituted (double-quoted) into the SELECT. Callers may pass any string.
func (s *Store) ListIDs(resourceType string) ([]string, error) {
	var table string
	err := s.db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name=?`,
		resourceType,
	).Scan(&table)
	var rows *sql.Rows
	if err == nil && table != "" {
		rows, err = s.db.Query(fmt.Sprintf(`SELECT id FROM "%s"`, strings.ReplaceAll(table, `"`, `""`)))
	}
	if err != nil || table == "" {
		// Fall back to generic resources table
		rows, err = s.db.Query("SELECT id FROM resources WHERE resource_type = ?", resourceType)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListIDsScoped is ListIDs with an optional tenant filter. scopeValue=="" =>
// unscoped (identical to ListIDs). When the typed table exists AND has
// scopeColumn (validated via validIdentifierRE + pragma_table_info), the IDs are
// filtered by that bound column. When the typed table exists but LACKS the
// column, it degrades to unscoped ListIDs (never silently returns zero parents).
// When no typed table exists, it filters the generic resources table via
// json_extract. scopeColumn is validated; scopeValue is always bound.
func (s *Store) ListIDsScoped(resourceType, scopeColumn, scopeValue string) ([]string, error) {
	if scopeValue == "" || scopeColumn == "" {
		return s.ListIDs(resourceType)
	}
	if !validIdentifierRE.MatchString(scopeColumn) {
		return nil, fmt.Errorf("ListIDsScoped: invalid scope column %q", scopeColumn)
	}
	var table string
	err := s.db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name=?`,
		resourceType,
	).Scan(&table)
	if err == nil && table != "" {
		var colName string
		colErr := s.db.QueryRow(
			`SELECT name FROM pragma_table_info(?) WHERE name=?`,
			table, scopeColumn,
		).Scan(&colName)
		if colErr != nil || colName == "" {
			// Typed table exists but lacks the scope column: degrade to unscoped
			// rather than returning zero parents.
			return s.ListIDs(resourceType)
		}
		qTable := strings.ReplaceAll(table, `"`, `""`)
		qCol := strings.ReplaceAll(colName, `"`, `""`)
		rows, qerr := s.db.Query(
			fmt.Sprintf(`SELECT id FROM "%s" WHERE "%s" = ?`, qTable, qCol), scopeValue)
		if qerr != nil {
			return nil, qerr
		}
		defer rows.Close()
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				continue
			}
			ids = append(ids, id)
		}
		return ids, rows.Err()
	}
	// No typed table: filter the generic resources table by body field.
	rows, qerr := s.db.Query(
		fmt.Sprintf(`SELECT id FROM resources WHERE resource_type = ? AND (CASE WHEN json_valid(data) THEN json_extract(data, '$.%s') END) = ?`, scopeColumn),
		resourceType, scopeValue,
	)
	if qerr != nil {
		return nil, qerr
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListField returns values of a named field from a resource's domain table,
// or from the generic resources table via json_extract when no typed column
// exists. Used by dependent sync to iterate parents when a spec-declared
// walker extracts a non-PK field (Endpoint.Walker.KeyField in the upstream
// printing-press repo) for the child path's placeholder.
//
// Defense in depth: field is validated against validIdentifierRE at entry
// — the regex pins it to SQL-safe identifier shape covering both the
// typed-column primary path AND the json_extract fallback (where
// pragma_table_info validation would never run if the parent's domain
// table doesn't exist yet). resourceType is never interpolated into SQL
// directly; we resolve it to a real table name via a parameterized
// sqlite_master lookup. Only validated names are substituted
// (double-quoted) into the SELECT. Mirrors ListIDs's defense pattern so
// callers may pass any string.
func (s *Store) ListField(resourceType, field string) ([]string, error) {
	if !validIdentifierRE.MatchString(field) {
		return nil, fmt.Errorf("ListField: invalid field name %q (must match %s)", field, validIdentifierRE.String())
	}
	var table string
	err := s.db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name=?`,
		resourceType,
	).Scan(&table)
	var rows *sql.Rows
	if err == nil && table != "" {
		// Validate the column exists on the resolved table before splicing
		// it into the SELECT. pragma_table_info is parameterizable.
		var colName string
		colErr := s.db.QueryRow(
			`SELECT name FROM pragma_table_info(?) WHERE name=?`,
			table, field,
		).Scan(&colName)
		if colErr == nil && colName != "" {
			qTable := strings.ReplaceAll(table, `"`, `""`)
			qCol := strings.ReplaceAll(colName, `"`, `""`)
			// DISTINCT: callers iterate the returned values as parent keys
			// for child-resource fan-out. Multiple parent rows sharing a
			// key_field value (legal for non-PK fields) would otherwise
			// cause the child endpoint to be fetched once per duplicate row.
			rows, err = s.db.Query(fmt.Sprintf(
				`SELECT DISTINCT "%s" FROM "%s" WHERE "%s" IS NOT NULL AND "%s" != ''`,
				qCol, qTable, qCol, qCol,
			))
		} else {
			err = colErr
		}
	}
	if err != nil || rows == nil {
		// Fall back to generic resources table via json_extract. Path is
		// Sprintf'd into the SQL string (matches ResolveByName below).
		// DISTINCT for the same reason as the typed-column path above.
		fallback := fmt.Sprintf(
			`SELECT DISTINCT json_extract(data, '$.%s') FROM resources WHERE resource_type = ? AND json_extract(data, '$.%s') IS NOT NULL`,
			field, field,
		)
		rows, err = s.db.Query(fallback, resourceType)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()

	var values []string
	for rows.Next() {
		var v sql.NullString
		if err := rows.Scan(&v); err == nil && v.Valid && v.String != "" {
			values = append(values, v.String)
		}
	}
	return values, rows.Err()
}

// ListFieldSets returns row-correlated values from the generic resources
// table. Dependent sync uses this for multi-placeholder paths where values
// such as owner/repo or server/webapp must stay paired per parent row.
func (s *Store) ListFieldSets(resourceType string, fields []string) ([]map[string]string, error) {
	if len(fields) == 0 {
		return nil, nil
	}
	for _, field := range fields {
		if !validIdentifierRE.MatchString(field) {
			return nil, fmt.Errorf("ListFieldSets: invalid field name %q (must match %s)", field, validIdentifierRE.String())
		}
	}

	rows, err := s.db.Query(`SELECT id, data FROM resources WHERE resource_type = ?`, resourceType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]string
	seenRows := map[string]bool{}
	for rows.Next() {
		var id string
		var data []byte
		if err := rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		var obj map[string]any
		if len(data) > 0 {
			var err error
			obj, err = DecodeJSONObject(data)
			if err != nil {
				return nil, fmt.Errorf("decode %s parent row %s: %w", resourceType, id, err)
			}
		}
		values := make(map[string]string, len(fields))
		complete := true
		for _, field := range fields {
			var value any
			if field == "id" {
				value = id
			} else {
				value = LookupFieldValue(obj, field)
			}
			valueString := ResourceIDString(value)
			if value == nil || valueString == "" {
				complete = false
				break
			}
			values[field] = valueString
		}
		if complete {
			keyParts := make([]string, 0, len(fields))
			for _, field := range fields {
				keyParts = append(keyParts, values[field])
			}
			key := strings.Join(keyParts, "\x00")
			if seenRows[key] {
				continue
			}
			seenRows[key] = true
			out = append(out, values)
		}
	}
	return out, rows.Err()
}

// GetLastSyncedAt returns the last sync timestamp for a resource type.
func (s *Store) GetLastSyncedAt(resourceType string) string {
	var ts sql.NullString
	s.db.QueryRow("SELECT last_synced_at FROM sync_state WHERE resource_type = ?", resourceType).Scan(&ts)
	if ts.Valid {
		return ts.String
	}
	return ""
}

// ClearSyncCursors resets all sync state for a full resync.
func (s *Store) ClearSyncCursors() error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	_, err := s.db.Exec("DELETE FROM sync_state")
	return err
}

// Query executes a raw SQL query and returns the rows.
// Used by workflow commands that need custom queries against the local store.
func (s *Store) Query(query string, args ...any) (*sql.Rows, error) {
	return s.db.Query(query, args...)
}

func (s *Store) Count(resourceType string) (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM resources WHERE resource_type = ?`,
		resourceType,
	).Scan(&count)
	return count, err
}

func (s *Store) Status() (map[string]int, error) {
	rows, err := s.db.Query(
		`SELECT resource_type, COUNT(*) FROM resources GROUP BY resource_type ORDER BY resource_type`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	status := make(map[string]int)
	for rows.Next() {
		var rt string
		var count int
		if err := rows.Scan(&rt, &count); err != nil {
			return nil, err
		}
		status[rt] = count
	}
	return status, rows.Err()
}

// CascadeJunction names a junction table + the FK column referencing the
// reconciled resource's primary key, to be cleaned when a row is swept.
type CascadeJunction struct {
	Table    string
	FKColumn string
}

var (
	cascadeMu        sync.Mutex
	cascadeJunctions = map[string][]CascadeJunction{}
)

// RegisterCascadeJunction records a junction to clean when rows of resourceType
// are reconciled away. Used for runtime-created junctions (e.g. module_issues)
// that the generated schema does not declare.
//
// Registration is idempotent: re-registering the same (Table, FKColumn) for a
// resourceType is a no-op. The registry is a process-global with no removal path
// (registrations happen once at startup in the generated binary); dedupe keeps a
// repeated init() or a test that re-registers across sub-tests from accumulating
// duplicate cascades.
func RegisterCascadeJunction(resourceType string, j CascadeJunction) {
	cascadeMu.Lock()
	defer cascadeMu.Unlock()
	for _, existing := range cascadeJunctions[resourceType] {
		if existing == j {
			return
		}
	}
	cascadeJunctions[resourceType] = append(cascadeJunctions[resourceType], j)
}

// CascadeJunctionsFor returns the registered cascade junctions for resourceType.
func CascadeJunctionsFor(resourceType string) []CascadeJunction {
	cascadeMu.Lock()
	defer cascadeMu.Unlock()
	out := make([]CascadeJunction, len(cascadeJunctions[resourceType]))
	copy(out, cascadeJunctions[resourceType])
	return out
}

// ReconcilePartition hard-deletes local rows of resourceType in one partition
// (rows whose data JSON at genericScopeJSONPath equals scopeValue) whose primary
// key is NOT in seenIDs. It is the mark-and-sweep half of deletion mirroring;
// the caller must pass the COMPLETE, successfully-enumerated seen-ID set for the
// partition. Victims are computed from the generic resources table so that
// legacy rows lacking a typed projection are also cleaned. Cleans, per victim:
// the typed table row (firing its AFTER DELETE FTS triggers, if any), the
// generic resources_fts entry (manual, no triggers), the generic resources row,
// and each cascade junction. Returns the number of generic rows deleted.
func (s *Store) ReconcilePartition(resourceType, genericScopeJSONPath, scopeValue string, seenIDs []string, typedTable string, cascades []CascadeJunction) (int, error) {
	if genericScopeJSONPath == "" || scopeValue == "" {
		return 0, fmt.Errorf("reconcile %s: empty partition scope", resourceType)
	}
	return s.reconcileUnseen(resourceType, seenIDs, typedTable, cascades,
		`SELECT id FROM resources
		 WHERE resource_type = ?
		   AND (CASE WHEN json_valid(data) THEN json_extract(data, ?) END) = ?`,
		resourceType, genericScopeJSONPath, scopeValue)
}

// Whole-table reconciliation is safe only when the caller supplies the complete
// seen-ID set from a proven-complete walk.
func (s *Store) ReconcileAll(resourceType string, seenIDs []string, typedTable string, cascades []CascadeJunction) (int, error) {
	return s.reconcileUnseen(resourceType, seenIDs, typedTable, cascades,
		`SELECT id FROM resources WHERE resource_type = ?`, resourceType)
}

func (s *Store) reconcileUnseen(resourceType string, seenIDs []string, typedTable string, cascades []CascadeJunction, query string, args ...any) (int, error) {
	s.lockForWrite()
	defer s.unlockAfterWrite()

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Seen-set membership is tested in Go, not SQL. Parent-keyed dependent rows
	// carry a NUL-composite storage id ("<id>\x00<parent>", built by
	// resourceStorageID) while seenIDs holds the BARE API ids sync enumerated, so
	// each stored id must run through BareResourceID before the comparison. A SQL
	// seen-set is not viable here: SQLite string functions treat the embedded NUL
	// as a C-string terminator, so an instr/substr or `IN` test over a key
	// containing "\x00" silently truncates and mis-matches. BareResourceID is a
	// no-op for plain ids, so flat/non-composite partitions are unaffected.
	seen := make(map[string]struct{}, len(seenIDs))
	for _, id := range seenIDs {
		seen[id] = struct{}{}
	}

	// CASE guards against a malformed-JSON row aborting the victim scan:
	// a row we cannot parse is never a victim — it is skipped (never deleted).
	rows, err := tx.Query(query, args...)
	if err != nil {
		return 0, fmt.Errorf("reconcile %s: select victims: %w", resourceType, err)
	}
	var victims []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return 0, err
		}
		if _, ok := seen[BareResourceID(id)]; ok {
			continue // bare id was enumerated this run — keep the row
		}
		victims = append(victims, id)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	// Safety: typedTable and cascade Table/FKColumn are TRUSTED generator/registration
	// metadata (schema-derived or RegisterCascadeJunction), not user input — Sprintf
	// interpolation here is intentional and safe.
	for _, id := range victims {
		if typedTable != "" {
			if _, err := tx.Exec(fmt.Sprintf(`DELETE FROM "%s" WHERE id = ?`, typedTable), id); err != nil {
				return 0, fmt.Errorf("reconcile %s: typed delete: %w", resourceType, err)
			}
		}
		if _, err := tx.Exec(`DELETE FROM resources_fts WHERE rowid = ?`, ftsRowID(resourceType, id)); err != nil {
			return 0, fmt.Errorf("reconcile %s: fts delete: %w", resourceType, err)
		}
		if _, err := tx.Exec(`DELETE FROM resources WHERE resource_type = ? AND id = ?`, resourceType, id); err != nil {
			return 0, fmt.Errorf("reconcile %s: generic delete: %w", resourceType, err)
		}
		// Cascade junction FKs hold the BARE entity id, never the NUL-composite
		// storage key, so strip the suffix before matching (no-op for plain ids).
		for _, c := range cascades {
			if _, err := tx.Exec(fmt.Sprintf(`DELETE FROM "%s" WHERE "%s" = ?`, c.Table, c.FKColumn), BareResourceID(id)); err != nil {
				return 0, fmt.Errorf("reconcile %s: cascade %s: %w", resourceType, c.Table, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(victims), nil
}

// ResolveByName resolves a human-readable name to a UUID from synced data.
// If the input is already a UUID, it is returned as-is.
// matchFields are JSON field names to search against (e.g., "name", "key", "email").
//
// json_extract path components cannot be bound as SQL parameters, so each
// field is validated against validIdentifierRE before being spliced into
// the query.
func (s *Store) ResolveByName(resourceType string, input string, matchFields ...string) (string, error) {
	if IsUUID(input) {
		return input, nil
	}

	var matches []string
	for _, field := range matchFields {
		if !validIdentifierRE.MatchString(field) {
			continue
		}
		query := fmt.Sprintf(
			`SELECT id FROM resources WHERE resource_type = ? AND LOWER(json_extract(data, '$.%s')) = LOWER(?)`,
			field,
		)
		rows, err := s.db.Query(query, resourceType, input)
		if err != nil {
			return "", err
		}
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				// Deduplicate
				found := false
				for _, m := range matches {
					if m == id {
						found = true
						break
					}
				}
				if !found {
					matches = append(matches, id)
				}
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return "", err
		}
		_ = rows.Close()
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("%s %q not found in local store. Run 'sync' first, or use the UUID directly", resourceType, input)
	case 1:
		return matches[0], nil
	default:
		hint := matches[0]
		if len(matches) > 5 {
			hint = strings.Join(matches[:5], ", ") + "..."
		} else {
			hint = strings.Join(matches, ", ")
		}
		return "", fmt.Errorf("ambiguous: %q matches %d %s entries (%s). Use the exact UUID instead", input, len(matches), resourceType, hint)
	}
}
