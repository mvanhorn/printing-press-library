package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Store is the local SQLite database for historical trending.
type Store struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS sync_state (
	id              INTEGER PRIMARY KEY AUTOINCREMENT,
	key             TEXT NOT NULL UNIQUE,
	value           TEXT NOT NULL,
	updated_at      INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS mining_snapshots (
	id              INTEGER PRIMARY KEY AUTOINCREMENT,
	ts              INTEGER NOT NULL,
	block_height    INTEGER NOT NULL,
	difficulty      REAL NOT NULL,
	hashrate_net    REAL NOT NULL,
	hashrate_local  REAL NOT NULL DEFAULT 0,
	peer_count      INTEGER NOT NULL DEFAULT 0,
	mempool_size    INTEGER NOT NULL DEFAULT 0,
	mempool_bytes   INTEGER NOT NULL DEFAULT 0,
	version         INTEGER NOT NULL DEFAULT 0,
	version_obs     INTEGER NOT NULL DEFAULT 0,
	errors_msg      TEXT NOT NULL DEFAULT '',
	wallet_balance  REAL NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_mining_ts ON mining_snapshots(ts);

CREATE TABLE IF NOT EXISTS block_events (
	id              INTEGER PRIMARY KEY AUTOINCREMENT,
	ts              INTEGER NOT NULL,
	block_height    INTEGER NOT NULL UNIQUE,
	block_hash      TEXT NOT NULL,
	difficulty      REAL NOT NULL DEFAULT 0,
	tx_count        INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_block_ts ON block_events(ts);
CREATE INDEX IF NOT EXISTS idx_block_height ON block_events(block_height);
`

func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "dogecoin-pp-cli", "db.sqlite")
}

// Open opens or creates the SQLite store at path.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("creating store dir: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("opening db: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("applying schema: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// MiningSnapshot represents one point-in-time snapshot.
type MiningSnapshot struct {
	TS           int64
	BlockHeight  int64
	Difficulty   float64
	HashrateNet  float64
	HashrateLocal float64
	PeerCount    int64
	MempoolSize  int64
	MempoolBytes int64
	Version      int64
	VersionObs   bool
	ErrorsMsg    string
	WalletBalance float64
}

// InsertSnapshot saves a mining snapshot.
func (s *Store) InsertSnapshot(ctx context.Context, snap MiningSnapshot) error {
	obs := 0
	if snap.VersionObs {
		obs = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO mining_snapshots
			(ts, block_height, difficulty, hashrate_net, hashrate_local, peer_count,
			 mempool_size, mempool_bytes, version, version_obs, errors_msg, wallet_balance)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		snap.TS, snap.BlockHeight, snap.Difficulty, snap.HashrateNet, snap.HashrateLocal,
		snap.PeerCount, snap.MempoolSize, snap.MempoolBytes, snap.Version, obs,
		snap.ErrorsMsg, snap.WalletBalance)
	return err
}

// LatestSnapshot returns the most recent mining snapshot.
func (s *Store) LatestSnapshot(ctx context.Context) (*MiningSnapshot, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT ts, block_height, difficulty, hashrate_net, hashrate_local, peer_count,
		       mempool_size, mempool_bytes, version, version_obs, errors_msg, wallet_balance
		FROM mining_snapshots ORDER BY ts DESC LIMIT 1`)
	return scanSnapshot(row)
}

// SnapshotsSince returns snapshots newer than cutoff (unix timestamp).
func (s *Store) SnapshotsSince(ctx context.Context, cutoff int64) ([]MiningSnapshot, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ts, block_height, difficulty, hashrate_net, hashrate_local, peer_count,
		       mempool_size, mempool_bytes, version, version_obs, errors_msg, wallet_balance
		FROM mining_snapshots WHERE ts >= ? ORDER BY ts ASC`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var snaps []MiningSnapshot
	for rows.Next() {
		snap, err := scanSnapshotRow(rows)
		if err != nil {
			return nil, err
		}
		snaps = append(snaps, *snap)
	}
	return snaps, rows.Err()
}

func scanSnapshot(row *sql.Row) (*MiningSnapshot, error) {
	var snap MiningSnapshot
	var obs int
	err := row.Scan(&snap.TS, &snap.BlockHeight, &snap.Difficulty, &snap.HashrateNet,
		&snap.HashrateLocal, &snap.PeerCount, &snap.MempoolSize, &snap.MempoolBytes,
		&snap.Version, &obs, &snap.ErrorsMsg, &snap.WalletBalance)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	snap.VersionObs = obs != 0
	return &snap, nil
}

func scanSnapshotRow(rows *sql.Rows) (*MiningSnapshot, error) {
	var snap MiningSnapshot
	var obs int
	err := rows.Scan(&snap.TS, &snap.BlockHeight, &snap.Difficulty, &snap.HashrateNet,
		&snap.HashrateLocal, &snap.PeerCount, &snap.MempoolSize, &snap.MempoolBytes,
		&snap.Version, &obs, &snap.ErrorsMsg, &snap.WalletBalance)
	if err != nil {
		return nil, err
	}
	snap.VersionObs = obs != 0
	return &snap, nil
}

// BlockEvent represents one block arrival.
type BlockEvent struct {
	TS          int64
	BlockHeight int64
	BlockHash   string
	Difficulty  float64
	TxCount     int64
}

// UpsertBlock inserts or ignores a block event (unique on block_height).
func (s *Store) UpsertBlock(ctx context.Context, ev BlockEvent) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO block_events (ts, block_height, block_hash, difficulty, tx_count)
		VALUES (?, ?, ?, ?, ?)`,
		ev.TS, ev.BlockHeight, ev.BlockHash, ev.Difficulty, ev.TxCount)
	return err
}

// BlocksSince returns block events newer than cutoff.
func (s *Store) BlocksSince(ctx context.Context, cutoff int64) ([]BlockEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ts, block_height, block_hash, difficulty, tx_count
		FROM block_events WHERE ts >= ? ORDER BY block_height ASC`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var evs []BlockEvent
	for rows.Next() {
		var ev BlockEvent
		if err := rows.Scan(&ev.TS, &ev.BlockHeight, &ev.BlockHash, &ev.Difficulty, &ev.TxCount); err != nil {
			return nil, err
		}
		evs = append(evs, ev)
	}
	return evs, rows.Err()
}

// HighestBlock returns the highest block height stored, or 0 if empty.
func (s *Store) HighestBlock(ctx context.Context) (int64, error) {
	var h sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT MAX(block_height) FROM block_events`).Scan(&h)
	if err != nil {
		return 0, err
	}
	return h.Int64, nil
}

// ParseSince converts durations like "30d", "7d", "24h" to a unix cutoff timestamp.
func ParseSince(since string) (int64, error) {
	if len(since) < 2 {
		return 0, fmt.Errorf("invalid --since value %q: use format like 30d, 7d, 24h", since)
	}
	unit := since[len(since)-1]
	numStr := since[:len(since)-1]
	var n int
	if _, err := fmt.Sscanf(numStr, "%d", &n); err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid --since value %q: use format like 30d, 7d, 24h", since)
	}
	var dur time.Duration
	switch unit {
	case 'd':
		dur = time.Duration(n) * 24 * time.Hour
	case 'h':
		dur = time.Duration(n) * time.Hour
	case 'm':
		dur = time.Duration(n) * time.Minute
	default:
		return 0, fmt.Errorf("invalid --since unit %q: use d (days), h (hours), or m (minutes)", string(unit))
	}
	return time.Now().Add(-dur).Unix(), nil
}

// SyncState tracks cursor-based sync progress for incremental updates.
type SyncState struct {
	Key       string
	Value     string
	UpdatedAt int64
}

// GetSyncState retrieves a sync state value by key.
func (s *Store) GetSyncState(ctx context.Context, key string) (*SyncState, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT key, value, updated_at FROM sync_state WHERE key = ?`, key)
	var st SyncState
	if err := row.Scan(&st.Key, &st.Value, &st.UpdatedAt); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &st, nil
}

// SaveSyncState upserts a sync state value.
func (s *Store) SaveSyncState(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sync_state (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		key, value, time.Now().Unix())
	return err
}
