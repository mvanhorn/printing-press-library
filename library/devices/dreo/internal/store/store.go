// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

// Package store is a thin SQLite-backed cache for Dreo device metadata,
// state snapshots, sensor timeseries, and named scenes. Uses modernc.org/sqlite
// (pure-Go) so the CLI ships without a CGo toolchain dependency.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Device mirrors the catalog rows in `devices`.
type Device struct {
	Sn        string          `json:"sn"`
	Name      string          `json:"name"`
	Model     string          `json:"model"`
	Room      string          `json:"room"`
	ProductID int             `json:"product_id"`
	Online    bool            `json:"online"`
	Raw       json.RawMessage `json:"raw,omitempty"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// Reading is one sensor sample.
type Reading struct {
	Sn     string    `json:"sn"`
	Ts     time.Time `json:"ts"`
	Metric string    `json:"metric"`
	Value  float64   `json:"value"`
}

// SceneInfo is a row of the scenes catalog.
type SceneInfo struct {
	Name    string    `json:"name"`
	SavedAt time.Time `json:"saved_at"`
	Devices int       `json:"devices"`
}

// Store wraps a *sql.DB with the Dreo CLI schema.
type Store struct {
	DB     *sql.DB
	useFTS bool
}

// DefaultPath returns the conventional store location.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "store.db"
	}
	return filepath.Join(home, ".cache", "dreo-pp-cli", "store.db")
}

// Open opens or creates the SQLite database at path. Parent dirs are
// created with mode 0o755 if missing. The schema is initialised on the
// first call; subsequent calls are idempotent.
func Open(path string) (*Store, error) {
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("store: mkdir: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	s := &Store{DB: db}
	if err := s.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the underlying *sql.DB handle.
func (s *Store) Close() error { return s.DB.Close() }

func (s *Store) initSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS devices (
			sn TEXT PRIMARY KEY,
			name TEXT,
			model TEXT,
			room TEXT,
			product_id INTEGER,
			online INTEGER,
			raw TEXT,
			updated_at INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS device_state (
			sn TEXT PRIMARY KEY,
			data TEXT,
			fetched_at INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS sensor_readings (
			sn TEXT,
			ts INTEGER,
			metric TEXT,
			value REAL,
			PRIMARY KEY (sn, ts, metric)
		)`,
		`CREATE TABLE IF NOT EXISTS scenes (
			name TEXT PRIMARY KEY,
			devices_json TEXT,
			saved_at INTEGER
		)`,
	}
	for _, q := range stmts {
		if _, err := s.DB.Exec(q); err != nil {
			return fmt.Errorf("store: schema: %w", err)
		}
	}
	// Try to create an FTS5 virtual table. If FTS5 isn't compiled in,
	// fall back silently to LIKE-based search.
	_, err := s.DB.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS devices_fts USING fts5(
		sn UNINDEXED, name, room, model,
		content='devices', content_rowid='rowid'
	)`)
	if err == nil {
		s.useFTS = true
	}
	return nil
}

// UpsertDevice writes/replaces a device row.
func (s *Store) UpsertDevice(ctx context.Context, d Device) error {
	online := 0
	if d.Online {
		online = 1
	}
	raw := string(d.Raw)
	updatedAt := d.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO devices(sn,name,model,room,product_id,online,raw,updated_at)
		VALUES(?,?,?,?,?,?,?,?)
		ON CONFLICT(sn) DO UPDATE SET
			name=excluded.name, model=excluded.model, room=excluded.room,
			product_id=excluded.product_id, online=excluded.online,
			raw=excluded.raw, updated_at=excluded.updated_at`,
		d.Sn, d.Name, d.Model, d.Room, d.ProductID, online, raw, updatedAt.Unix())
	if err != nil {
		return err
	}
	if s.useFTS {
		// rebuild this row's FTS entry
		_, _ = s.DB.ExecContext(ctx, `DELETE FROM devices_fts WHERE sn=?`, d.Sn)
		_, _ = s.DB.ExecContext(ctx, `INSERT INTO devices_fts(sn,name,room,model) VALUES(?,?,?,?)`,
			d.Sn, d.Name, d.Room, d.Model)
	}
	return nil
}

// ListDevices returns every cached device, newest-updated first.
func (s *Store) ListDevices(ctx context.Context) ([]Device, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT sn,name,model,room,product_id,online,raw,updated_at
		FROM devices ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDevices(rows)
}

// SearchDevices runs a free-text query against name/room/model/sn.
func (s *Store) SearchDevices(ctx context.Context, q string) ([]Device, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return s.ListDevices(ctx)
	}
	if s.useFTS {
		// Build a tolerant FTS query: AND of prefix tokens.
		tokens := strings.Fields(q)
		for i, t := range tokens {
			tokens[i] = sanitizeFTSToken(t) + "*"
		}
		ftsQuery := strings.Join(tokens, " ")
		rows, err := s.DB.QueryContext(ctx, `SELECT d.sn,d.name,d.model,d.room,d.product_id,d.online,d.raw,d.updated_at
			FROM devices_fts f JOIN devices d ON d.sn=f.sn
			WHERE devices_fts MATCH ? ORDER BY rank`, ftsQuery)
		if err == nil {
			defer rows.Close()
			return scanDevices(rows)
		}
		// fall through to LIKE on error
	}
	like := "%" + strings.ToLower(q) + "%"
	rows, err := s.DB.QueryContext(ctx, `SELECT sn,name,model,room,product_id,online,raw,updated_at
		FROM devices
		WHERE lower(sn) LIKE ? OR lower(name) LIKE ? OR lower(room) LIKE ? OR lower(model) LIKE ?
		ORDER BY name`, like, like, like, like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDevices(rows)
}

// GetDevice looks up a device by exact sn first, then by name (case-insensitive).
func (s *Store) GetDevice(ctx context.Context, snOrName string) (*Device, error) {
	q := strings.TrimSpace(snOrName)
	if q == "" {
		return nil, errors.New("store.GetDevice: empty key")
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT sn,name,model,room,product_id,online,raw,updated_at
		FROM devices WHERE sn=? LIMIT 1`, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	devs, _ := scanDevices(rows)
	if len(devs) > 0 {
		return &devs[0], nil
	}
	// Name lookup (case-insensitive exact, then prefix)
	rows2, err := s.DB.QueryContext(ctx, `SELECT sn,name,model,room,product_id,online,raw,updated_at
		FROM devices WHERE lower(name)=? LIMIT 1`, strings.ToLower(q))
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	devs2, _ := scanDevices(rows2)
	if len(devs2) > 0 {
		return &devs2[0], nil
	}
	return nil, fmt.Errorf("store: device %q not found", snOrName)
}

// UpsertDeviceState writes the latest state JSON snapshot for a device.
func (s *Store) UpsertDeviceState(ctx context.Context, sn string, data json.RawMessage, fetchedAt time.Time) error {
	if fetchedAt.IsZero() {
		fetchedAt = time.Now()
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO device_state(sn,data,fetched_at) VALUES(?,?,?)
		ON CONFLICT(sn) DO UPDATE SET data=excluded.data, fetched_at=excluded.fetched_at`,
		sn, string(data), fetchedAt.Unix())
	return err
}

// GetDeviceState returns the cached state for a device.
func (s *Store) GetDeviceState(ctx context.Context, sn string) (json.RawMessage, time.Time, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT data, fetched_at FROM device_state WHERE sn=?`, sn)
	var data string
	var ts int64
	if err := row.Scan(&data, &ts); err != nil {
		return nil, time.Time{}, err
	}
	return json.RawMessage(data), time.Unix(ts, 0), nil
}

// AppendSensorReading inserts a (sn, ts, metric, value) row.
// Duplicate (sn, ts, metric) triples are silently ignored.
func (s *Store) AppendSensorReading(ctx context.Context, sn string, ts time.Time, metric string, value float64) error {
	if ts.IsZero() {
		ts = time.Now()
	}
	_, err := s.DB.ExecContext(ctx, `INSERT OR IGNORE INTO sensor_readings(sn,ts,metric,value)
		VALUES(?,?,?,?)`, sn, ts.Unix(), metric, value)
	return err
}

// QuerySensorReadings returns readings matching filters; limit==0 -> no cap.
func (s *Store) QuerySensorReadings(ctx context.Context, sn string, since, until time.Time, metric string, limit int) ([]Reading, error) {
	conds := []string{}
	args := []any{}
	if sn != "" {
		conds = append(conds, "sn=?")
		args = append(args, sn)
	}
	if !since.IsZero() {
		conds = append(conds, "ts>=?")
		args = append(args, since.Unix())
	}
	if !until.IsZero() {
		conds = append(conds, "ts<=?")
		args = append(args, until.Unix())
	}
	if metric != "" {
		conds = append(conds, "metric=?")
		args = append(args, metric)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	limitClause := ""
	if limit > 0 {
		limitClause = fmt.Sprintf(" LIMIT %d", limit)
	}
	q := "SELECT sn,ts,metric,value FROM sensor_readings " + where + " ORDER BY ts DESC" + limitClause
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Reading
	for rows.Next() {
		var r Reading
		var ts int64
		if err := rows.Scan(&r.Sn, &ts, &r.Metric, &r.Value); err != nil {
			return nil, err
		}
		r.Ts = time.Unix(ts, 0)
		out = append(out, r)
	}
	return out, rows.Err()
}

// SaveScene persists a named snapshot. devices_json maps sn -> field map.
func (s *Store) SaveScene(ctx context.Context, name string, snapshots map[string]map[string]any) error {
	if name == "" {
		return errors.New("store.SaveScene: name required")
	}
	raw, err := json.Marshal(snapshots)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO scenes(name,devices_json,saved_at) VALUES(?,?,?)
		ON CONFLICT(name) DO UPDATE SET devices_json=excluded.devices_json, saved_at=excluded.saved_at`,
		name, string(raw), time.Now().Unix())
	return err
}

// LoadScene returns the saved field map for each device in the scene.
func (s *Store) LoadScene(ctx context.Context, name string) (map[string]map[string]any, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT devices_json FROM scenes WHERE name=?`, name)
	var raw string
	if err := row.Scan(&raw); err != nil {
		return nil, err
	}
	var out map[string]map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("store: scene %q corrupt: %w", name, err)
	}
	return out, nil
}

// ListScenes returns scene metadata, newest first.
func (s *Store) ListScenes(ctx context.Context) ([]SceneInfo, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT name, saved_at, devices_json FROM scenes ORDER BY saved_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SceneInfo
	for rows.Next() {
		var name, raw string
		var ts int64
		if err := rows.Scan(&name, &ts, &raw); err != nil {
			return nil, err
		}
		count := 0
		var m map[string]any
		if json.Unmarshal([]byte(raw), &m) == nil {
			count = len(m)
		}
		out = append(out, SceneInfo{Name: name, SavedAt: time.Unix(ts, 0), Devices: count})
	}
	return out, rows.Err()
}

func scanDevices(rows *sql.Rows) ([]Device, error) {
	var out []Device
	for rows.Next() {
		var d Device
		var online int
		var raw sql.NullString
		var updatedAt int64
		if err := rows.Scan(&d.Sn, &d.Name, &d.Model, &d.Room, &d.ProductID, &online, &raw, &updatedAt); err != nil {
			return nil, err
		}
		d.Online = online != 0
		if raw.Valid {
			d.Raw = json.RawMessage(raw.String)
		}
		d.UpdatedAt = time.Unix(updatedAt, 0)
		out = append(out, d)
	}
	return out, rows.Err()
}

// sanitizeFTSToken escapes characters that would break FTS5 query syntax.
func sanitizeFTSToken(t string) string {
	t = strings.Map(func(r rune) rune {
		switch r {
		case '"', '\'', '\\', '(', ')', '*':
			return -1
		}
		return r
	}, t)
	return t
}
