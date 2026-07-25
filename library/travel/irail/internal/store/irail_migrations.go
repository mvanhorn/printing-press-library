// Hand-authored store extension for irail-pp-cli.
//
// Lives beside the generated store.go rather than inside it so that
// `generate --force` preserves it. Novel commands call EnsureIrailSchema
// lazily before touching these tables.

package store

import (
	"context"
	"database/sql"
	"fmt"
)

// irailSchema holds the tables that back the observation history and saved
// routes. The live iRail API has no historical endpoint, so delay history only
// exists if this CLI records it.
var irailSchema = []string{
	`CREATE TABLE IF NOT EXISTS irail_observations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		observed_at INTEGER NOT NULL,
		station TEXT NOT NULL,
		board_type TEXT NOT NULL DEFAULT 'departure',
		vehicle TEXT NOT NULL,
		vehicle_short TEXT,
		direction TEXT,
		scheduled_at INTEGER NOT NULL,
		delay_seconds INTEGER NOT NULL DEFAULT 0,
		canceled INTEGER NOT NULL DEFAULT 0,
		left_station INTEGER NOT NULL DEFAULT 0,
		platform TEXT,
		platform_normal INTEGER NOT NULL DEFAULT 1,
		occupancy TEXT,
		departure_connection TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_irail_obs_vehicle ON irail_observations(vehicle, scheduled_at)`,
	`CREATE INDEX IF NOT EXISTS idx_irail_obs_station ON irail_observations(station, observed_at)`,
	`CREATE INDEX IF NOT EXISTS idx_irail_obs_conn ON irail_observations(departure_connection)`,
	// One row per (departure, observation instant). Re-running observe in the
	// same second is idempotent instead of double-counting a delay sample.
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_irail_obs_unique
		ON irail_observations(station, board_type, vehicle, scheduled_at, observed_at)`,

	`CREATE TABLE IF NOT EXISTS irail_saved_routes (
		name TEXT PRIMARY KEY,
		from_station TEXT NOT NULL,
		to_station TEXT NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL
	)`,
}

// EnsureIrailSchema creates the hand-authored tables if they do not exist.
// Safe to call repeatedly and from every novel command.
func (s *Store) EnsureIrailSchema(ctx context.Context) error {
	for _, stmt := range irailSchema {
		if _, err := s.DB().ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("irail schema: %w", err)
		}
	}
	return nil
}

// Observation is one recorded board row.
type Observation struct {
	ObservedAt          int64  `json:"observed_at"`
	Station             string `json:"station"`
	BoardType           string `json:"board_type"`
	Vehicle             string `json:"vehicle"`
	VehicleShort        string `json:"vehicle_short,omitempty"`
	Direction           string `json:"direction,omitempty"`
	ScheduledAt         int64  `json:"scheduled_at"`
	DelaySeconds        int    `json:"delay_seconds"`
	Canceled            bool   `json:"canceled"`
	Left                bool   `json:"left"`
	Platform            string `json:"platform,omitempty"`
	PlatformNormal      bool   `json:"platform_normal"`
	Occupancy           string `json:"occupancy,omitempty"`
	DepartureConnection string `json:"departure_connection,omitempty"`
}

// InsertObservations writes a batch of observations in one transaction.
// Duplicate rows (same station/board/vehicle/scheduled/observed instant) are
// ignored rather than erroring, so repeated observe runs stay idempotent.
// It returns the number of rows actually inserted.
func (s *Store) InsertObservations(ctx context.Context, obs []Observation) (int, error) {
	if len(obs) == 0 {
		return 0, nil
	}
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin observation tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO irail_observations (
			observed_at, station, board_type, vehicle, vehicle_short, direction,
			scheduled_at, delay_seconds, canceled, left_station, platform,
			platform_normal, occupancy, departure_connection
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return 0, fmt.Errorf("prepare observation insert: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	inserted := 0
	for _, o := range obs {
		res, err := stmt.ExecContext(ctx,
			o.ObservedAt, o.Station, o.BoardType, o.Vehicle, o.VehicleShort, o.Direction,
			o.ScheduledAt, o.DelaySeconds, boolToInt(o.Canceled), boolToInt(o.Left), o.Platform,
			boolToInt(o.PlatformNormal), o.Occupancy, o.DepartureConnection)
		if err != nil {
			return 0, fmt.Errorf("insert observation for %s: %w", o.Vehicle, err)
		}
		if n, err := res.RowsAffected(); err == nil {
			inserted += int(n)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit observations: %w", err)
	}
	return inserted, nil
}

// ObservationCount reports how many observations are stored, so commands that
// read history can tell an empty store from a genuinely quiet route.
func (s *Store) ObservationCount(ctx context.Context) (int, error) {
	var n int
	err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM irail_observations`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count observations: %w", err)
	}
	return n, nil
}

// SavedRoute is a named shortcut. ToStation is empty for station-only shortcuts.
type SavedRoute struct {
	Name        string `json:"name"`
	FromStation string `json:"from"`
	ToStation   string `json:"to,omitempty"`
	CreatedAt   int64  `json:"created_at"`
}

// SaveRoute inserts or replaces a named shortcut.
func (s *Store) SaveRoute(ctx context.Context, r SavedRoute) error {
	_, err := s.DB().ExecContext(ctx,
		`INSERT INTO irail_saved_routes (name, from_station, to_station, created_at)
		 VALUES (?,?,?,?)
		 ON CONFLICT(name) DO UPDATE SET
		   from_station=excluded.from_station,
		   to_station=excluded.to_station`,
		r.Name, r.FromStation, r.ToStation, r.CreatedAt)
	if err != nil {
		return fmt.Errorf("save route %q: %w", r.Name, err)
	}
	return nil
}

// ListSavedRoutes returns every shortcut, oldest first.
func (s *Store) ListSavedRoutes(ctx context.Context) ([]SavedRoute, error) {
	rows, err := s.DB().QueryContext(ctx,
		`SELECT name, from_station, COALESCE(to_station,''), created_at
		 FROM irail_saved_routes ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list saved routes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]SavedRoute, 0)
	for rows.Next() {
		var r SavedRoute
		var to sql.NullString
		if err := rows.Scan(&r.Name, &r.FromStation, &to, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan saved route: %w", err)
		}
		r.ToStation = to.String
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate saved routes: %w", err)
	}
	return out, nil
}

// GetSavedRoute resolves one shortcut by name.
func (s *Store) GetSavedRoute(ctx context.Context, name string) (SavedRoute, bool, error) {
	var r SavedRoute
	var to sql.NullString
	err := s.DB().QueryRowContext(ctx,
		`SELECT name, from_station, COALESCE(to_station,''), created_at
		 FROM irail_saved_routes WHERE name = ?`, name).
		Scan(&r.Name, &r.FromStation, &to, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return r, false, nil
	}
	if err != nil {
		return r, false, fmt.Errorf("get saved route %q: %w", name, err)
	}
	r.ToStation = to.String
	return r, true, nil
}

// DeleteSavedRoute removes a shortcut, reporting whether it existed.
func (s *Store) DeleteSavedRoute(ctx context.Context, name string) (bool, error) {
	res, err := s.DB().ExecContext(ctx, `DELETE FROM irail_saved_routes WHERE name = ?`, name)
	if err != nil {
		return false, fmt.Errorf("delete saved route %q: %w", name, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, nil
	}
	return n > 0, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
