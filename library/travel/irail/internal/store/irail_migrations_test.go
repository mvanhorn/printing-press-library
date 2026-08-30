// Copyright 2026 Olivier and contributors. Licensed under Apache-2.0. See LICENSE.
// Upgrade tests for the observation store.
//
// These build a database in the pre-round_id shape, then run EnsureIrailSchema
// over it, because the failure mode being guarded against is an upgrade that
// leaves the CLI unable to open its own store.

package store

import (
	"context"
	"path/filepath"
	"testing"
)

// legacyObservationsDDL is the observation table exactly as the first build of
// this CLI created it: no round_id, and a unique key ending in a one-second
// observed_at.
const legacyObservationsDDL = `
CREATE TABLE irail_observations (
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
)`

const legacyObservationsIndex = `
CREATE UNIQUE INDEX idx_irail_obs_unique
	ON irail_observations(station, board_type, vehicle, scheduled_at, observed_at)`

// openLegacyStore returns a store whose observation table is in the old shape.
func openLegacyStore(t *testing.T, withLegacyIndex bool) *Store {
	t.Helper()
	db, err := OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.DB().Exec(legacyObservationsDDL); err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if withLegacyIndex {
		if _, err := db.DB().Exec(legacyObservationsIndex); err != nil {
			t.Fatalf("create legacy index: %v", err)
		}
	}
	return db
}

// insertLegacyRow writes directly through the legacy column set, bypassing
// InsertObservations, which now requires a round id.
func insertLegacyRow(t *testing.T, db *Store, observedAt int64, station, boardType, vehicle, direction string, scheduledAt int64) error {
	t.Helper()
	_, err := db.DB().Exec(
		`INSERT OR IGNORE INTO irail_observations
			(observed_at, station, board_type, vehicle, direction, scheduled_at)
		 VALUES (?,?,?,?,?,?)`,
		observedAt, station, boardType, vehicle, direction, scheduledAt)
	return err
}

func countObservations(t *testing.T, db *Store) int {
	t.Helper()
	var n int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM irail_observations`).Scan(&n); err != nil {
		t.Fatalf("count observations: %v", err)
	}
	return n
}

// TestLegacyIndexAlreadyRejectedSameSecondRouteDuplicates establishes why the
// upgrade is safe: under the old unique key, two same-second route captures out
// of one origin sharing a vehicle and scheduled time could never both be
// stored, because direction was not part of that key either. The row a naive
// backfill would collide on therefore cannot exist in a store this CLI wrote.
func TestLegacyIndexAlreadyRejectedSameSecondRouteDuplicates(t *testing.T) {
	db := openLegacyStore(t, true)
	const at, sched = int64(1_800_000_000), int64(1_800_000_500)

	if err := insertLegacyRow(t, db, at, "Ghent-Sint-Pieters", "route", "BE.NMBS.IC1", "Brussels-Central", sched); err != nil {
		t.Fatalf("first legacy insert: %v", err)
	}
	if err := insertLegacyRow(t, db, at, "Ghent-Sint-Pieters", "route", "BE.NMBS.IC1", "Antwerp-Central", sched); err != nil {
		t.Fatalf("second legacy insert: %v", err)
	}
	if got := countObservations(t, db); got != 1 {
		t.Fatalf("legacy store holds %d rows, want 1: the old key did not dedupe as assumed", got)
	}
}

// TestMigrateLegacyStoreSeparatesRouteDestinations covers the ordinary upgrade:
// legacy rows survive, get a round id, and two routes out of one origin become
// distinct rounds rather than one merged history.
func TestMigrateLegacyStoreSeparatesRouteDestinations(t *testing.T) {
	db := openLegacyStore(t, true)
	ctx := context.Background()
	const at = int64(1_800_000_000)

	// Same second, same origin, different destinations and different trains:
	// permitted by the legacy key, and genuinely two separate captures.
	if err := insertLegacyRow(t, db, at, "Ghent-Sint-Pieters", "route", "BE.NMBS.IC1", "Brussels-Central", 1_800_000_500); err != nil {
		t.Fatalf("legacy insert: %v", err)
	}
	if err := insertLegacyRow(t, db, at, "Ghent-Sint-Pieters", "route", "BE.NMBS.IC9", "Antwerp-Central", 1_800_000_600); err != nil {
		t.Fatalf("legacy insert: %v", err)
	}
	// A departure board whose rows carry per-train headsigns must stay one round.
	if err := insertLegacyRow(t, db, at, "Leuven", "departure", "BE.NMBS.IC3", "Oostende", 1_800_000_700); err != nil {
		t.Fatalf("legacy insert: %v", err)
	}
	if err := insertLegacyRow(t, db, at, "Leuven", "departure", "BE.NMBS.IC4", "Hasselt", 1_800_000_800); err != nil {
		t.Fatalf("legacy insert: %v", err)
	}

	if err := db.EnsureIrailSchema(ctx); err != nil {
		t.Fatalf("upgrade failed, leaving the store unusable: %v", err)
	}
	if got := countObservations(t, db); got != 4 {
		t.Fatalf("upgrade kept %d rows, want all 4", got)
	}

	routes, err := db.RecentObservationRounds(ctx, "Ghent-Sint-Pieters", "route", "Brussels-Central", 10)
	if err != nil {
		t.Fatalf("route rounds: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("got %d Brussels route round(s), want 1", len(routes))
	}

	// The two departure rows share one capture, so they must share one round.
	departures, err := db.RecentObservationRounds(ctx, "Leuven", "departure", "", 10)
	if err != nil {
		t.Fatalf("departure rounds: %v", err)
	}
	if len(departures) != 1 {
		t.Fatalf("got %d departure round(s), want 1: per-train headsigns split the round", len(departures))
	}
	rows, err := db.ObservationsInRound(ctx, departures[0].ID)
	if err != nil {
		t.Fatalf("observations in round: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("departure round holds %d rows, want 2", len(rows))
	}
}

// TestMigrateToleratesViolatingLegacyRows is the belt-and-braces case: if a
// store somehow holds rows that would violate the new unique key, the upgrade
// must still complete. A failed CREATE UNIQUE INDEX would leave every command
// unable to open the database.
func TestMigrateToleratesViolatingLegacyRows(t *testing.T) {
	// No legacy unique index, so the duplicate below is actually storable.
	db := openLegacyStore(t, false)
	ctx := context.Background()
	const at, sched = int64(1_800_000_000), int64(1_800_000_500)

	for i := 0; i < 2; i++ {
		if err := insertLegacyRow(t, db, at, "Ghent-Sint-Pieters", "route", "BE.NMBS.IC1", "Brussels-Central", sched); err != nil {
			t.Fatalf("legacy insert %d: %v", i, err)
		}
	}
	if got := countObservations(t, db); got != 2 {
		t.Fatalf("setup stored %d rows, want 2 violating rows", got)
	}

	if err := db.EnsureIrailSchema(ctx); err != nil {
		t.Fatalf("upgrade failed on duplicate rows, leaving the store unusable: %v", err)
	}
	if got := countObservations(t, db); got != 1 {
		t.Fatalf("upgrade kept %d rows, want the duplicate collapsed to 1", got)
	}
}

// TestEnsureIrailSchemaIsRepeatable pins that the upgrade can run on every
// command invocation, which is how it is actually called.
func TestEnsureIrailSchemaIsRepeatable(t *testing.T) {
	db := openLegacyStore(t, true)
	ctx := context.Background()

	if err := insertLegacyRow(t, db, 1_800_000_000, "Leuven", "departure", "BE.NMBS.IC3", "Oostende", 1_800_000_700); err != nil {
		t.Fatalf("legacy insert: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := db.EnsureIrailSchema(ctx); err != nil {
			t.Fatalf("EnsureIrailSchema run %d: %v", i+1, err)
		}
	}
	if got := countObservations(t, db); got != 1 {
		t.Fatalf("repeated upgrades left %d rows, want 1", got)
	}
}
