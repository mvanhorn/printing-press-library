package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	DB *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	// modernc.org/sqlite driver name is "sqlite".
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(on)")
	if err != nil {
		return nil, err
	}
	// WAL allows concurrent readers; writes serialize at the SQLite layer via busy_timeout.
	s := &Store{DB: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.DB.Close() }

const schema = `
CREATE TABLE IF NOT EXISTS protocols (
    id              TEXT,
    slug            TEXT PRIMARY KEY,
    name            TEXT,
    symbol          TEXT,
    chain           TEXT,
    category        TEXT,
    tvl             REAL,
    mcap            REAL,
    change_1h       REAL,
    change_1d       REAL,
    change_7d       REAL,
    chains          TEXT,
    url             TEXT,
    description     TEXT
);
CREATE INDEX IF NOT EXISTS idx_protocols_tvl ON protocols(tvl);
CREATE INDEX IF NOT EXISTS idx_protocols_category ON protocols(category);

CREATE TABLE IF NOT EXISTS protocol_chain_tvl (
    protocol_slug   TEXT,
    chain           TEXT,
    tvl             REAL,
    PRIMARY KEY (protocol_slug, chain)
);
CREATE INDEX IF NOT EXISTS idx_pct_chain ON protocol_chain_tvl(chain);
CREATE INDEX IF NOT EXISTS idx_pct_tvl ON protocol_chain_tvl(tvl);

CREATE TABLE IF NOT EXISTS chains (
    name            TEXT PRIMARY KEY,
    tvl             REAL,
    token_symbol    TEXT,
    cmc_id          TEXT,
    gecko_id        TEXT
);

CREATE TABLE IF NOT EXISTS chain_tvl_hist (
    chain           TEXT,
    date            TEXT,
    tvl             REAL,
    PRIMARY KEY (chain, date)
);

CREATE TABLE IF NOT EXISTS pools (
    pool_id         TEXT PRIMARY KEY,
    chain           TEXT,
    project         TEXT,
    symbol          TEXT,
    tvl_usd         REAL,
    apy             REAL,
    apy_base        REAL,
    apy_reward      REAL,
    il_risk         TEXT,
    stablecoin      INTEGER,
    exposure        TEXT,
    pool_meta       TEXT,
    underlying_tokens TEXT,
    reward_tokens   TEXT
);
CREATE INDEX IF NOT EXISTS idx_pools_chain ON pools(chain);
CREATE INDEX IF NOT EXISTS idx_pools_project ON pools(project);
CREATE INDEX IF NOT EXISTS idx_pools_apy ON pools(apy);
CREATE INDEX IF NOT EXISTS idx_pools_tvl ON pools(tvl_usd);

CREATE TABLE IF NOT EXISTS stablecoins (
    id              TEXT PRIMARY KEY,
    name            TEXT,
    symbol          TEXT,
    peg_type        TEXT,
    peg_mechanism   TEXT,
    circulating     REAL,
    price           REAL,
    mcap            REAL,
    chains          TEXT
);
CREATE INDEX IF NOT EXISTS idx_stablecoins_circ ON stablecoins(circulating);

CREATE TABLE IF NOT EXISTS stablecoin_chains (
    stablecoin_id   TEXT,
    chain           TEXT,
    circulating     REAL,
    PRIMARY KEY (stablecoin_id, chain)
);

CREATE TABLE IF NOT EXISTS dex_overview (
    protocol        TEXT PRIMARY KEY,
    display_name    TEXT,
    total_24h       REAL,
    total_7d        REAL,
    total_30d       REAL,
    change_1d       REAL,
    change_7d       REAL,
    change_30d      REAL,
    chains          TEXT
);
CREATE INDEX IF NOT EXISTS idx_dex_24h ON dex_overview(total_24h);

CREATE TABLE IF NOT EXISTS dex_chain_volume (
    protocol        TEXT,
    chain           TEXT,
    total_24h       REAL,
    total_7d        REAL,
    total_30d       REAL,
    PRIMARY KEY (protocol, chain)
);
CREATE INDEX IF NOT EXISTS idx_dxv_chain ON dex_chain_volume(chain);
CREATE INDEX IF NOT EXISTS idx_dxv_24h ON dex_chain_volume(total_24h);

CREATE TABLE IF NOT EXISTS fees_overview (
    protocol        TEXT PRIMARY KEY,
    display_name    TEXT,
    total_24h_fees  REAL,
    total_24h_rev   REAL,
    total_7d_fees   REAL,
    total_7d_rev    REAL,
    total_30d_fees  REAL,
    total_30d_rev   REAL,
    category        TEXT,
    chains          TEXT
);
CREATE INDEX IF NOT EXISTS idx_fees_rev ON fees_overview(total_24h_rev);
CREATE INDEX IF NOT EXISTS idx_fees_fees ON fees_overview(total_24h_fees);

CREATE TABLE IF NOT EXISTS options_overview (
    protocol        TEXT PRIMARY KEY,
    display_name    TEXT,
    total_24h       REAL,
    total_7d        REAL,
    chains          TEXT
);

CREATE TABLE IF NOT EXISTS open_interest (
    protocol        TEXT PRIMARY KEY,
    display_name    TEXT,
    total_oi        REAL,
    chains          TEXT
);

-- Historical tables (P2.1)
CREATE TABLE IF NOT EXISTS protocol_tvl_hist (
    protocol_slug   TEXT,
    date            TEXT,
    tvl             REAL,
    chain           TEXT DEFAULT '',
    PRIMARY KEY (protocol_slug, date, chain)
);
CREATE INDEX IF NOT EXISTS idx_pthist_date ON protocol_tvl_hist(date);

CREATE TABLE IF NOT EXISTS fees_hist (
    protocol        TEXT,
    date            TEXT,
    fees            REAL,
    revenue         REAL,
    PRIMARY KEY (protocol, date)
);

CREATE TABLE IF NOT EXISTS dex_hist (
    protocol        TEXT,
    date            TEXT,
    volume          REAL,
    PRIMARY KEY (protocol, date)
);

CREATE TABLE IF NOT EXISTS pool_hist (
    pool_id         TEXT,
    date            TEXT,
    tvl             REAL,
    apy             REAL,
    PRIMARY KEY (pool_id, date)
);

CREATE TABLE IF NOT EXISTS stablecoin_hist (
    stablecoin_id   TEXT,
    date            TEXT,
    circulating     REAL,
    chain           TEXT DEFAULT '',
    PRIMARY KEY (stablecoin_id, date, chain)
);

CREATE TABLE IF NOT EXISTS sync_meta (
    domain          TEXT PRIMARY KEY,
    last_sync       INTEGER,
    row_count       INTEGER,
    note            TEXT
);
`

func (s *Store) migrate() error {
	_, err := s.DB.Exec(schema)
	return err
}

func (s *Store) Tx(fn func(*sql.Tx) error) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) SetSyncMeta(domain string, rowCount int, note string) error {
	_, err := s.DB.Exec(
		`INSERT INTO sync_meta(domain, last_sync, row_count, note)
		 VALUES(?, ?, ?, ?)
		 ON CONFLICT(domain) DO UPDATE SET last_sync=excluded.last_sync, row_count=excluded.row_count, note=excluded.note`,
		domain, time.Now().Unix(), rowCount, note,
	)
	return err
}

type SyncMeta struct {
	Domain   string
	LastSync time.Time
	RowCount int
	Note     string
}

func (s *Store) GetSyncMeta(domain string) (*SyncMeta, error) {
	row := s.DB.QueryRow(`SELECT domain, last_sync, row_count, COALESCE(note,'') FROM sync_meta WHERE domain = ?`, domain)
	m := &SyncMeta{}
	var ts int64
	if err := row.Scan(&m.Domain, &ts, &m.RowCount, &m.Note); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	m.LastSync = time.Unix(ts, 0)
	return m, nil
}

func (s *Store) AllSyncMeta() ([]SyncMeta, error) {
	rows, err := s.DB.Query(`SELECT domain, last_sync, row_count, COALESCE(note,'') FROM sync_meta ORDER BY domain`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SyncMeta
	for rows.Next() {
		m := SyncMeta{}
		var ts int64
		if err := rows.Scan(&m.Domain, &ts, &m.RowCount, &m.Note); err != nil {
			return nil, err
		}
		m.LastSync = time.Unix(ts, 0)
		out = append(out, m)
	}
	return out, rows.Err()
}

// StaleBefore reports whether the named domain's last sync is older than dur ago,
// or has never been synced at all.
func (s *Store) StaleBefore(domain string, dur time.Duration) (bool, error) {
	m, err := s.GetSyncMeta(domain)
	if err != nil {
		return false, err
	}
	if m == nil {
		return true, nil
	}
	return time.Since(m.LastSync) > dur, nil
}

// Helper for command output.
func (s *Store) Exec(query string, args ...any) (sql.Result, error) {
	return s.DB.Exec(query, args...)
}

func (s *Store) Query(query string, args ...any) (*sql.Rows, error) {
	return s.DB.Query(query, args...)
}

func (s *Store) QueryRow(query string, args ...any) *sql.Row {
	return s.DB.QueryRow(query, args...)
}

func formatErr(stage string, err error) error {
	return fmt.Errorf("%s: %w", stage, err)
}

var _ = formatErr // reserved for future helpers
