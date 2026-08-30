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
		round_id TEXT NOT NULL DEFAULT '',
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
	// Rounds are enumerated per (station, board type) newest-first, so the
	// board type has to be part of the index or every history read scans.
	`CREATE INDEX IF NOT EXISTS idx_irail_obs_station ON irail_observations(station, board_type, observed_at)`,
	`CREATE INDEX IF NOT EXISTS idx_irail_obs_conn ON irail_observations(departure_connection)`,

	`CREATE TABLE IF NOT EXISTS irail_saved_routes (
		name TEXT PRIMARY KEY,
		from_station TEXT NOT NULL,
		to_station TEXT NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL
	)`,
}

// EnsureIrailSchema creates the hand-authored tables if they do not exist and
// upgrades stores written by an earlier build. Safe to call repeatedly and from
// every novel command.
func (s *Store) EnsureIrailSchema(ctx context.Context) error {
	for _, stmt := range irailSchema {
		if _, err := s.DB().ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("irail schema: %w", err)
		}
	}
	return s.migrateObservationRounds(ctx)
}

// migrateObservationRounds moves the observation table from an implicit round
// key to an explicit one.
//
// The original key was (station, board_type, vehicle, scheduled_at,
// observed_at) with observed_at at one-second resolution. Two captures of the
// same board that started within the same second produced identical keys, so
// INSERT OR IGNORE silently discarded the second one and the reader saw a
// single merged round instead of two samples. Rounds are now identified
// explicitly by round_id, which observe generates per capture.
//
// Every statement is idempotent, so this runs on each EnsureIrailSchema call.
func (s *Store) migrateObservationRounds(ctx context.Context) error {
	hasRound, err := s.columnExists(ctx, "irail_observations", "round_id")
	if err != nil {
		return err
	}
	if !hasRound {
		if _, err := s.DB().ExecContext(ctx,
			`ALTER TABLE irail_observations ADD COLUMN round_id TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("irail schema: add round_id: %w", err)
		}
	}
	// Rows written before round_id existed belong to whatever round the old key
	// implied, so the backfill reconstructs that grouping. Route captures also
	// fold in the destination: for board_type 'route' the direction column is
	// the target of the capture and is constant across the round, so including
	// it separates two routes out of one origin that the old key merged. For
	// departure and arrival rows direction is each train's own headsign and
	// varies within a round, so it must stay out of the key there.
	if _, err := s.DB().ExecContext(ctx,
		`UPDATE irail_observations
		    SET round_id = station || '|' || board_type || '|' || observed_at ||
		                   CASE WHEN board_type = 'route'
		                        THEN '|' || COALESCE(direction, '')
		                        ELSE '' END
		  WHERE round_id = ''`); err != nil {
		return fmt.Errorf("irail schema: backfill round_id: %w", err)
	}
	if _, err := s.DB().ExecContext(ctx,
		`DROP INDEX IF EXISTS idx_irail_obs_unique`); err != nil {
		return fmt.Errorf("irail schema: drop legacy observation index: %w", err)
	}
	// Guard the index creation. The backfilled key is at least as specific as
	// the old unique key, so a store written by this CLI cannot hold a row that
	// violates it. Rather than rely on that, drop any row that would violate
	// the constraint anyway: a failed CREATE UNIQUE INDEX would leave every
	// command unable to open the database, which is a far worse outcome than
	// discarding a duplicate the old INSERT OR IGNORE would never have written.
	if _, err := s.DB().ExecContext(ctx,
		`DELETE FROM irail_observations
		  WHERE id NOT IN (
		        SELECT MIN(id) FROM irail_observations
		         GROUP BY round_id, vehicle, scheduled_at)`); err != nil {
		return fmt.Errorf("irail schema: drop duplicate observations: %w", err)
	}
	// Created after the backfill: doing it earlier would make every legacy row
	// collide on the empty round_id.
	if _, err := s.DB().ExecContext(ctx,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_irail_obs_round_unique
			ON irail_observations(round_id, vehicle, scheduled_at)`); err != nil {
		return fmt.Errorf("irail schema: observation round index: %w", err)
	}
	return nil
}

// columnExists reports whether a table already has a column, so migrations can
// be skipped rather than erroring on a second run.
func (s *Store) columnExists(ctx context.Context, table, column string) (bool, error) {
	var n int
	err := s.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, column).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("irail schema: inspect %s.%s: %w", table, column, err)
	}
	return n > 0, nil
}

// Observation is one recorded board row.
//
// RoundID groups every row captured by a single observe call against a single
// target. It is what makes two captures distinguishable even when they share an
// observed_at second.
type Observation struct {
	RoundID             string `json:"round_id"`
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
// Rows are deduplicated within a round only: the same vehicle and scheduled
// time seen twice in one capture collapses, while two separate captures are
// always kept apart even if they share an observed_at second.
// It returns the number of rows actually inserted.
func (s *Store) InsertObservations(ctx context.Context, obs []Observation) (int, error) {
	if len(obs) == 0 {
		return 0, nil
	}
	for i := range obs {
		if obs[i].RoundID == "" {
			return 0, fmt.Errorf("insert observation for %s: round id is required", obs[i].Vehicle)
		}
	}
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin observation tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO irail_observations (
			round_id, observed_at, station, board_type, vehicle, vehicle_short, direction,
			scheduled_at, delay_seconds, canceled, left_station, platform,
			platform_normal, occupancy, departure_connection
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return 0, fmt.Errorf("prepare observation insert: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	inserted := 0
	for _, o := range obs {
		res, err := stmt.ExecContext(ctx,
			o.RoundID, o.ObservedAt, o.Station, o.BoardType, o.Vehicle, o.VehicleShort, o.Direction,
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

// ObservationRound identifies one capture of one target.
type ObservationRound struct {
	ID         string `json:"round_id"`
	ObservedAt int64  `json:"observed_at"`
}

// RecentObservationRounds returns the most recent capture rounds for one
// target, newest first.
//
// boardType is required. Departure, arrival and route captures of the same
// station are separate histories: comparing a departures round against an
// arrivals round reports every train as appearing or vanishing.
//
// direction scopes route captures to a single destination, because a station
// can be the origin of several observed routes. It must be empty for
// departure and arrival boards, where direction holds each train's own
// headsign rather than the target of the capture.
func (s *Store) RecentObservationRounds(ctx context.Context, station, boardType, direction string, limit int) ([]ObservationRound, error) {
	query := `SELECT round_id, MAX(observed_at) AS at
		    FROM irail_observations
		   WHERE station = ? AND board_type = ?`
	argv := []any{station, boardType}
	if direction != "" {
		query += ` AND direction = ?`
		argv = append(argv, direction)
	}
	query += ` GROUP BY round_id ORDER BY at DESC, round_id DESC LIMIT ?`
	argv = append(argv, limit)

	rows, err := s.DB().QueryContext(ctx, query, argv...)
	if err != nil {
		return nil, fmt.Errorf("querying observation rounds: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]ObservationRound, 0, limit)
	for rows.Next() {
		var r ObservationRound
		if err := rows.Scan(&r.ID, &r.ObservedAt); err != nil {
			return nil, fmt.Errorf("scanning observation round: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating observation rounds: %w", err)
	}
	return out, nil
}

// ObservationsInRound loads every row captured in one round. The round id
// already pins station, board type and — for routes — destination, so no
// further scoping is needed here.
func (s *Store) ObservationsInRound(ctx context.Context, roundID string) ([]Observation, error) {
	rows, err := s.DB().QueryContext(ctx,
		`SELECT round_id, observed_at, station, board_type, vehicle,
		        COALESCE(vehicle_short,''), COALESCE(direction,''), scheduled_at,
		        delay_seconds, canceled, left_station, COALESCE(platform,''),
		        platform_normal, COALESCE(occupancy,''), COALESCE(departure_connection,'')
		   FROM irail_observations WHERE round_id = ?`, roundID)
	if err != nil {
		return nil, fmt.Errorf("querying observations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]Observation, 0)
	for rows.Next() {
		var o Observation
		var short, dir, platform, occupancy, conn sql.NullString
		var scheduled, delay, canceled, left, normal sql.NullInt64
		if err := rows.Scan(&o.RoundID, &o.ObservedAt, &o.Station, &o.BoardType, &o.Vehicle,
			&short, &dir, &scheduled, &delay, &canceled, &left, &platform,
			&normal, &occupancy, &conn); err != nil {
			return nil, fmt.Errorf("scanning observation: %w", err)
		}
		o.VehicleShort = short.String
		o.Direction = dir.String
		o.ScheduledAt = scheduled.Int64
		o.DelaySeconds = int(delay.Int64)
		o.Canceled = canceled.Int64 == 1
		o.Left = left.Int64 == 1
		o.Platform = platform.String
		// A missing platform_normal means iRail did not send the flag; treat
		// that as the usual platform rather than as a change.
		o.PlatformNormal = !normal.Valid || normal.Int64 == 1
		o.Occupancy = occupancy.String
		o.DepartureConnection = conn.String
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating observations: %w", err)
	}
	return out, nil
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
