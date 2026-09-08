// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package portfolio

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const portfolioSchemaVersion = 1

const portfolioSchema = `
CREATE TABLE IF NOT EXISTS portfolio_meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS snapshots (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    endpoint TEXT NOT NULL,
    api_version TEXT NOT NULL,
    discovery_revision TEXT NOT NULL,
    customer_id TEXT NOT NULL,
    login_customer_id TEXT NOT NULL DEFAULT '',
    language TEXT NOT NULL,
    network TEXT NOT NULL,
    currency_code TEXT NOT NULL DEFAULT '',
    currency_source TEXT NOT NULL DEFAULT '',
    geo_set_id TEXT NOT NULL,
    geo_target_constants TEXT NOT NULL,
    submitted_seeds TEXT NOT NULL,
    submitted_keywords TEXT NOT NULL,
    requested_start TEXT NOT NULL,
    requested_end TEXT NOT NULL,
    request_body BLOB,
    request_body_present INTEGER NOT NULL DEFAULT 0 CHECK (request_body_present IN (0,1)),
    request_body_sha256 TEXT NOT NULL DEFAULT '',
    source_variant TEXT NOT NULL DEFAULT '',
    metadata BLOB,
    fetched_at TEXT NOT NULL,
    status TEXT NOT NULL,
    complete INTEGER NOT NULL DEFAULT 0 CHECK (complete IN (0,1)),
    finalized INTEGER NOT NULL DEFAULT 0 CHECK (finalized IN (0,1)),
    finished_at TEXT,
    warning_flags TEXT NOT NULL DEFAULT '[]',
    failed_receipts TEXT NOT NULL DEFAULT '[]',
    missing_batches TEXT NOT NULL DEFAULT '[]'
);

CREATE INDEX IF NOT EXISTS snapshots_fetched_idx ON snapshots(fetched_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS snapshots_endpoint_idx ON snapshots(endpoint);

CREATE TABLE IF NOT EXISTS response_receipts (
    id TEXT PRIMARY KEY,
    snapshot_id TEXT NOT NULL REFERENCES snapshots(id),
    endpoint TEXT NOT NULL,
    page_token TEXT NOT NULL DEFAULT '',
    page_number INTEGER NOT NULL CHECK (page_number >= 0),
    batch_number INTEGER NOT NULL CHECK (batch_number >= 0),
    attempt INTEGER NOT NULL CHECK (attempt >= 0),
    request_body BLOB,
    request_body_present INTEGER NOT NULL DEFAULT 0 CHECK (request_body_present IN (0,1)),
    submitted_keywords TEXT NOT NULL,
    http_status INTEGER NOT NULL DEFAULT 0,
    google_request_id TEXT NOT NULL DEFAULT '',
    body BLOB,
    body_present INTEGER NOT NULL DEFAULT 0 CHECK (body_present IN (0,1)),
    body_sha256 TEXT NOT NULL,
    fetched_at TEXT NOT NULL,
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    normalized INTEGER NOT NULL DEFAULT 0 CHECK (normalized IN (0,1)),
    normalized_at TEXT,
    UNIQUE(snapshot_id, endpoint, page_number, batch_number, attempt)
);

CREATE INDEX IF NOT EXISTS receipts_snapshot_idx ON response_receipts(snapshot_id, page_number, batch_number, attempt);

CREATE TABLE IF NOT EXISTS account_metadata (
    id TEXT PRIMARY KEY,
    snapshot_id TEXT NOT NULL REFERENCES snapshots(id),
    receipt_id TEXT NOT NULL REFERENCES response_receipts(id),
    currency_code TEXT NOT NULL,
    source TEXT NOT NULL,
    recorded_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS account_metadata_snapshot_idx ON account_metadata(snapshot_id, recorded_at, id);

CREATE TABLE IF NOT EXISTS keyword_metrics (
    id TEXT PRIMARY KEY,
    snapshot_id TEXT NOT NULL REFERENCES snapshots(id),
    receipt_id TEXT NOT NULL REFERENCES response_receipts(id),
    endpoint TEXT NOT NULL,
    returned_text TEXT NOT NULL,
    submitted_text TEXT NOT NULL DEFAULT '',
    submitted_index INTEGER,
    variant_group TEXT NOT NULL DEFAULT '',
    linkage_status TEXT NOT NULL DEFAULT 'UNKNOWN',
    average_monthly_searches INTEGER,
    average_monthly_searches_state TEXT NOT NULL DEFAULT 'absent',
    competition TEXT NOT NULL DEFAULT '',
    competition_index INTEGER,
    low_bid_micros INTEGER,
    high_bid_micros INTEGER,
    average_cpc_micros INTEGER,
    average_cpc_state TEXT NOT NULL DEFAULT 'absent',
    annotations BLOB,
    raw_result BLOB NOT NULL,
    status TEXT NOT NULL DEFAULT 'ok',
    flags TEXT NOT NULL DEFAULT '[]',
    created_at TEXT NOT NULL,
    UNIQUE(receipt_id, id)
);

CREATE INDEX IF NOT EXISTS metrics_snapshot_idx ON keyword_metrics(snapshot_id, returned_text, id);
CREATE INDEX IF NOT EXISTS metrics_receipt_idx ON keyword_metrics(receipt_id);

CREATE TABLE IF NOT EXISTS monthly_volumes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    snapshot_id TEXT NOT NULL REFERENCES snapshots(id),
    receipt_id TEXT NOT NULL REFERENCES response_receipts(id),
    metric_id TEXT NOT NULL REFERENCES keyword_metrics(id),
    returned_text TEXT NOT NULL,
    month TEXT,
    raw_month TEXT NOT NULL DEFAULT '',
    raw_month_json BLOB,
    raw_year TEXT NOT NULL DEFAULT '',
    raw_year_json BLOB,
    monthly_searches INTEGER,
    value_state TEXT NOT NULL DEFAULT 'absent',
    eligibility TEXT NOT NULL DEFAULT 'eligible',
    flags TEXT NOT NULL DEFAULT '[]',
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS monthly_snapshot_idx ON monthly_volumes(snapshot_id, month, returned_text);
CREATE INDEX IF NOT EXISTS monthly_metric_idx ON monthly_volumes(metric_id);

CREATE TABLE IF NOT EXISTS variant_links (
    id TEXT PRIMARY KEY,
    snapshot_id TEXT NOT NULL REFERENCES snapshots(id),
    receipt_id TEXT NOT NULL REFERENCES response_receipts(id),
    metric_id TEXT NOT NULL REFERENCES keyword_metrics(id),
    submitted_index INTEGER,
    submitted_text TEXT NOT NULL DEFAULT '',
    returned_text TEXT NOT NULL DEFAULT '',
    relationship TEXT NOT NULL,
    variant_group TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS variants_snapshot_idx ON variant_links(snapshot_id, returned_text, id);

CREATE TABLE IF NOT EXISTS coverage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    snapshot_id TEXT NOT NULL REFERENCES snapshots(id),
    receipt_id TEXT NOT NULL REFERENCES response_receipts(id),
    endpoint TEXT NOT NULL,
    page_number INTEGER NOT NULL,
    batch_number INTEGER NOT NULL,
    attempt INTEGER NOT NULL,
    requested_start TEXT NOT NULL,
    requested_end TEXT NOT NULL,
    requested_terms TEXT NOT NULL,
    returned_terms TEXT NOT NULL,
    returned_months TEXT NOT NULL,
    missing_months TEXT NOT NULL,
    unavailable_months TEXT NOT NULL,
    raw_only_months TEXT NOT NULL,
    requested_count INTEGER NOT NULL,
    returned_count INTEGER NOT NULL,
    returned_month_count INTEGER NOT NULL,
    no_result INTEGER NOT NULL CHECK (no_result IN (0,1)),
    collection_status TEXT NOT NULL,
    complete INTEGER NOT NULL CHECK (complete IN (0,1)),
    next_page_token TEXT NOT NULL DEFAULT '',
    vendor_total_size INTEGER,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS coverage_snapshot_idx ON coverage(snapshot_id, page_number, batch_number, attempt);
`

func open(ctx context.Context, path string, readOnly bool) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("%w: database path is empty", ErrInvalidInput)
	}
	if readOnly {
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return nil, ErrNotFound
			}
			return nil, fmt.Errorf("stat portfolio database: %w", err)
		}
	} else if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, fmt.Errorf("create portfolio directory: %w", err)
		}
	}

	dsn := sqliteDSN(path, readOnly)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open portfolio database: %w", err)
	}
	if path == ":memory:" {
		db.SetMaxOpenConns(1)
	} else {
		db.SetMaxOpenConns(4)
		db.SetMaxIdleConns(2)
	}
	s := &Store{db: db, path: path, readOnly: readOnly, writeMu: make(chan struct{}, 1)}
	s.writeMu <- struct{}{}
	if !readOnly {
		if err := s.migrate(ctx); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping portfolio database: %w", err)
	}
	return s, nil
}

func sqliteDSN(path string, readOnly bool) string {
	if path == ":memory:" {
		return "file:portfolio-memory?mode=memory&cache=shared&_pragma=foreign_keys(ON)"
	}
	// Escaping spaces and query delimiters keeps a local path a SQLite URI path.
	// The OS path remains the value reported by Store.Path.
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	if readOnly {
		return u.String() + "?mode=ro&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	}
	return u.String() + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)"
}

func (s *Store) migrate(ctx context.Context) error {
	if s.readOnly {
		return fmt.Errorf("%w: read-only portfolio", ErrInvalidInput)
	}
	if err := s.withWriteLock(ctx, func() error {
		if _, err := s.db.ExecContext(ctx, portfolioSchema); err != nil {
			return fmt.Errorf("create portfolio schema: %w", err)
		}
		var version string
		err := s.db.QueryRowContext(ctx, `SELECT value FROM portfolio_meta WHERE key = 'schema_version'`).Scan(&version)
		if errors.Is(err, sql.ErrNoRows) {
			_, err = s.db.ExecContext(ctx, `INSERT INTO portfolio_meta(key, value) VALUES('schema_version', ?)`, fmt.Sprint(portfolioSchemaVersion))
		} else if err == nil && version != fmt.Sprint(portfolioSchemaVersion) {
			return fmt.Errorf("unsupported portfolio schema version %q (want %d)", version, portfolioSchemaVersion)
		}
		if err != nil {
			return fmt.Errorf("stamp portfolio schema: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("migrate portfolio database: %w", err)
	}
	if s.path != ":memory:" {
		_ = os.Chmod(s.path, 0o600)
		_ = os.Chmod(s.path+"-wal", 0o600)
		_ = os.Chmod(s.path+"-shm", 0o600)
	}
	return nil
}

func (s *Store) withWriteLock(ctx context.Context, fn func() error) error {
	if s.readOnly {
		return fmt.Errorf("%w: portfolio is read-only", ErrInvalidInput)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.writeMu:
	}
	defer func() { s.writeMu <- struct{}{} }()
	return fn()
}

// Close releases the SQLite handle. Closing a read-only handle is safe.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// DB returns the underlying handle for read-only reporting and DuckDB-proof
// helpers. Callers must not close it or issue writes through it.
func (s *Store) DB() *sql.DB { return s.db }

// Path returns the configured on-disk path.
func (s *Store) Path() string { return s.path }

// SchemaVersion returns the portfolio schema version.
func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	var value string
	if err := s.db.QueryRowContext(ctx, `SELECT value FROM portfolio_meta WHERE key = 'schema_version'`).Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, fmt.Errorf("read portfolio schema version: %w", err)
	}
	var version int
	if _, err := fmt.Sscanf(value, "%d", &version); err != nil {
		return 0, fmt.Errorf("parse portfolio schema version: %w", err)
	}
	return version, nil
}

func utcText(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func currentTime(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t.UTC()
}
