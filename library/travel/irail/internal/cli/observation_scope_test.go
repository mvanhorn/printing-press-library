// Copyright 2026 Olivier and contributors. Licensed under Apache-2.0. See LICENSE.
// Regression tests for observation round identity and board-type scoping.
//
// Each test here corresponds to a defect where the observation history read
// back something that was never observed: two captures collapsing into one, or
// departure/arrival/route captures being compared and averaged together.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/irail/internal/store"
)

// seedStore opens a throwaway store with the irail tables in place.
func seedStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "irail.db")
	db, err := store.OpenWithContext(context.Background(), path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.EnsureIrailSchema(context.Background()); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	return db, path
}

// canonical is the station name the commands actually query with. Tests that
// drive a command through the cobra tree must seed the store with this, not
// with the alias a user types, or they pass by finding nothing at all.
func canonical(name string) string { return resolveStationName(name) }

// obs builds one observation row. Fields the test does not care about get
// stable defaults so each case only states what it is actually exercising.
func obs(roundID string, observedAt int64, station, boardType, vehicle string, delay int) store.Observation {
	return store.Observation{
		RoundID:        roundID,
		ObservedAt:     observedAt,
		Station:        station,
		BoardType:      boardType,
		Vehicle:        vehicle,
		VehicleShort:   vehicle,
		ScheduledAt:    1_800_000_000,
		DelaySeconds:   delay,
		PlatformNormal: true,
		Platform:       "3",
	}
}

// TestSameSecondCapturesStayDistinct pins the fix for observation rounds
// colliding on a one-second timestamp. Two captures of the same board that
// start within the same second are two samples, not one; the old key made the
// second one vanish through INSERT OR IGNORE.
func TestSameSecondCapturesStayDistinct(t *testing.T) {
	db, _ := seedStore(t)
	ctx := context.Background()
	const at = int64(1_800_000_100)

	first := []store.Observation{obs("round-a", at, "Brussels-Central", "departure", "BE.NMBS.IC2843", 0)}
	second := []store.Observation{obs("round-b", at, "Brussels-Central", "departure", "BE.NMBS.IC2843", 600)}

	for _, batch := range [][]store.Observation{first, second} {
		n, err := db.InsertObservations(ctx, batch)
		if err != nil {
			t.Fatalf("insert observations: %v", err)
		}
		if n != 1 {
			t.Fatalf("inserted %d rows, want 1 (same-second capture was dropped)", n)
		}
	}

	rounds, err := db.RecentObservationRounds(ctx, "Brussels-Central", "departure", "", 10)
	if err != nil {
		t.Fatalf("recent rounds: %v", err)
	}
	if len(rounds) != 2 {
		t.Fatalf("got %d rounds, want 2 distinct rounds sharing observed_at %d", len(rounds), at)
	}
}

// TestRoundIsIdempotentWithinOneCapture keeps the property that made the
// original key attractive: a vehicle seen twice inside a single capture is one
// sample, so a retried write cannot inflate the delay statistics.
func TestRoundIsIdempotentWithinOneCapture(t *testing.T) {
	db, _ := seedStore(t)
	ctx := context.Background()

	batch := []store.Observation{obs("round-a", 1_800_000_100, "Leuven", "departure", "BE.NMBS.IC1832", 0)}
	if _, err := db.InsertObservations(ctx, batch); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	n, err := db.InsertObservations(ctx, batch)
	if err != nil {
		t.Fatalf("replayed insert: %v", err)
	}
	if n != 0 {
		t.Fatalf("replaying one capture inserted %d rows, want 0", n)
	}
}

// TestRecentObservationRoundsScopeByBoardType pins that a station's departure,
// arrival and route captures are separate histories. Reading them together
// makes every train in one look like it appeared or vanished from the other.
func TestRecentObservationRoundsScopeByBoardType(t *testing.T) {
	db, _ := seedStore(t)
	ctx := context.Background()
	const at = int64(1_800_000_200)

	batches := []store.Observation{
		obs("dep-1", at, "Ghent-Sint-Pieters", "departure", "BE.NMBS.IC1", 0),
		obs("arr-1", at, "Ghent-Sint-Pieters", "arrival", "BE.NMBS.IC2", 0),
	}
	routeRow := obs("route-1", at, "Ghent-Sint-Pieters", "route", "BE.NMBS.IC3", 0)
	routeRow.Direction = "Brussels-Central"

	if _, err := db.InsertObservations(ctx, append(batches, routeRow)); err != nil {
		t.Fatalf("insert observations: %v", err)
	}

	for _, tc := range []struct{ boardType, direction, wantRound string }{
		{"departure", "", "dep-1"},
		{"arrival", "", "arr-1"},
		{"route", "Brussels-Central", "route-1"},
	} {
		rounds, err := db.RecentObservationRounds(ctx, "Ghent-Sint-Pieters", tc.boardType, tc.direction, 10)
		if err != nil {
			t.Fatalf("%s rounds: %v", tc.boardType, err)
		}
		if len(rounds) != 1 {
			t.Fatalf("%s: got %d rounds, want exactly 1 (scopes are leaking)", tc.boardType, len(rounds))
		}
		if rounds[0].ID != tc.wantRound {
			t.Fatalf("%s: got round %q, want %q", tc.boardType, rounds[0].ID, tc.wantRound)
		}
	}
}

// TestRecentObservationRoundsScopeRouteByDirection pins that two routes out of
// the same origin are separate histories. Without the direction scope, a
// Ghent->Brussels round would be diffed against a Ghent->Antwerp round.
func TestRecentObservationRoundsScopeRouteByDirection(t *testing.T) {
	db, _ := seedStore(t)
	ctx := context.Background()

	toBrussels := obs("route-bru", 1_800_000_300, "Ghent-Sint-Pieters", "route", "BE.NMBS.IC1", 0)
	toBrussels.Direction = "Brussels-Central"
	toAntwerp := obs("route-ant", 1_800_000_300, "Ghent-Sint-Pieters", "route", "BE.NMBS.IC9", 0)
	toAntwerp.Direction = "Antwerp-Central"

	if _, err := db.InsertObservations(ctx, []store.Observation{toBrussels, toAntwerp}); err != nil {
		t.Fatalf("insert observations: %v", err)
	}

	rounds, err := db.RecentObservationRounds(ctx, "Ghent-Sint-Pieters", "route", "Brussels-Central", 10)
	if err != nil {
		t.Fatalf("recent rounds: %v", err)
	}
	if len(rounds) != 1 || rounds[0].ID != "route-bru" {
		t.Fatalf("got %+v, want only the Brussels route round", rounds)
	}
}

// TestChangesDoesNotDiffAcrossBoardTypes is the end-to-end version of the
// scoping bug: a departure round and an arrival round recorded at different
// instants must not be reported as one board losing and gaining trains.
func TestChangesDoesNotDiffAcrossBoardTypes(t *testing.T) {
	db, path := seedStore(t)
	ctx := context.Background()

	station := canonical("Brussels-Central")
	rows := []store.Observation{
		obs("dep-1", 1_800_000_400, station, "departure", "BE.NMBS.IC2843", 0),
		obs("arr-1", 1_800_000_500, station, "arrival", "BE.NMBS.IC7777", 0),
	}
	if _, err := db.InsertObservations(ctx, rows); err != nil {
		t.Fatalf("insert observations: %v", err)
	}

	out := runChanges(t, "--station", "Brussels-Central", "--db", path, "--json")

	var view changesView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("decode changes output %q: %v", out, err)
	}
	if view.Station != station {
		t.Fatalf("station = %q, want %q (test seeded a name the command never queries)", view.Station, station)
	}
	if view.BoardType != "departure" {
		t.Fatalf("board_type = %q, want departure", view.BoardType)
	}
	if len(view.Changes) != 0 {
		t.Fatalf("got %d change(s) from one departure round plus an arrival round, want 0: %+v",
			len(view.Changes), view.Changes)
	}
	if view.Note == "" {
		t.Fatalf("expected a note explaining that one departure round is not enough to compare")
	}
}

// TestChangesDiffsWithinOneBoardType is the positive control for the test
// above: once two rounds of the same board type exist, real deltas still
// surface.
func TestChangesDiffsWithinOneBoardType(t *testing.T) {
	db, path := seedStore(t)
	ctx := context.Background()

	station := canonical("Brussels-Central")
	rows := []store.Observation{
		obs("dep-1", 1_800_000_400, station, "departure", "BE.NMBS.IC2843", 0),
		obs("dep-2", 1_800_000_500, station, "departure", "BE.NMBS.IC2843", 600),
		// An arrival round in between must not disturb the departure diff.
		obs("arr-1", 1_800_000_450, station, "arrival", "BE.NMBS.IC7777", 0),
	}
	if _, err := db.InsertObservations(ctx, rows); err != nil {
		t.Fatalf("insert observations: %v", err)
	}

	out := runChanges(t, "--station", "Brussels-Central", "--db", path, "--json")

	var view changesView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("decode changes output %q: %v", out, err)
	}
	if !hasKind(view.Changes, "delay-increased") {
		t.Fatalf("want a delay-increased row, got %+v", view.Changes)
	}
	for _, ch := range view.Changes {
		if ch.Vehicle == "BE.NMBS.IC7777" {
			t.Fatalf("arrival-board train leaked into the departure diff: %+v", ch)
		}
	}
}

// TestPunctualityScopesByBoardType pins that delay statistics are never
// averaged across departure, arrival and route captures of the same station.
func TestPunctualityScopesByBoardType(t *testing.T) {
	db, path := seedStore(t)
	ctx := context.Background()

	// Same vehicle observed on both boards, with very different delays. If the
	// board types are merged, the departure average is pulled toward 300.
	now := time.Now().Unix()
	station := canonical("Leuven")
	rows := []store.Observation{
		obs("dep-1", now, station, "departure", "BE.NMBS.IC1832", 0),
		obs("arr-1", now, station, "arrival", "BE.NMBS.IC1832", 600),
	}
	if _, err := db.InsertObservations(ctx, rows); err != nil {
		t.Fatalf("insert observations: %v", err)
	}

	for _, tc := range []struct {
		boardType string
		wantDelay int
	}{
		{"departure", 0},
		{"arrival", 600},
	} {
		out := runPunctuality(t, "--station", "Leuven", "--board-type", tc.boardType, "--db", path, "--json")

		var view punctualityView
		if err := json.Unmarshal([]byte(out), &view); err != nil {
			t.Fatalf("%s: decode punctuality output %q: %v", tc.boardType, out, err)
		}
		if view.BoardType != tc.boardType {
			t.Fatalf("board_type = %q, want %q", view.BoardType, tc.boardType)
		}
		if view.Samples != 1 {
			t.Fatalf("%s: samples = %d, want 1 (board types are being merged)", tc.boardType, view.Samples)
		}
		if len(view.Trains) != 1 {
			t.Fatalf("%s: got %d trains, want 1", tc.boardType, len(view.Trains))
		}
		if got := view.Trains[0].AvgDelaySec; got != tc.wantDelay {
			t.Fatalf("%s: avg delay = %d, want %d", tc.boardType, got, tc.wantDelay)
		}
	}
}

// TestChangesRejectsRouteWithoutDestination keeps the route scope honest: a
// station can be the origin of several observed routes, so a route diff
// without a destination would compare unrelated journeys.
func TestChangesRejectsRouteWithoutDestination(t *testing.T) {
	_, path := seedStore(t)

	cmd := RootCmd()
	cmd.SetArgs([]string{"changes", "--station", "Ghent-Sint-Pieters", "--board-type", "route", "--db", path, "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("route diff without --to succeeded, want a usage error; output:\n%s", out.String())
	}
}

// TestPunctualityRejectsRouteWithoutDestination mirrors the changes guard: a
// station is usually the origin of several observed routes, so a route summary
// without --to would average unrelated journeys into one reliability figure.
func TestPunctualityRejectsRouteWithoutDestination(t *testing.T) {
	_, path := seedStore(t)

	cmd := RootCmd()
	cmd.SetArgs([]string{"punctuality", "--from", "Ghent-Sint-Pieters", "--board-type", "route", "--db", path, "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("route summary without --to succeeded, want a usage error; output:\n%s", out.String())
	}
}

// TestPunctualityScopesRouteByDestination is the positive control: once the
// destination is named, only that route's observations are summarised.
func TestPunctualityScopesRouteByDestination(t *testing.T) {
	db, path := seedStore(t)
	ctx := context.Background()

	origin := canonical("Ghent-Sint-Pieters")
	now := time.Now().Unix()

	toBrussels := obs("route-bru", now, origin, "route", "BE.NMBS.IC1", 0)
	toBrussels.Direction = canonical("Brussels-Central")
	toAntwerp := obs("route-ant", now, origin, "route", "BE.NMBS.IC9", 1200)
	toAntwerp.Direction = canonical("Antwerp-Central")

	if _, err := db.InsertObservations(ctx, []store.Observation{toBrussels, toAntwerp}); err != nil {
		t.Fatalf("insert observations: %v", err)
	}

	out := runPunctuality(t,
		"--from", "Ghent-Sint-Pieters", "--to", "Brussels-Central",
		"--board-type", "route", "--db", path, "--json")

	var view punctualityView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("decode punctuality output %q: %v", out, err)
	}
	if view.Samples != 1 {
		t.Fatalf("samples = %d, want 1 (destinations out of one origin are being merged)", view.Samples)
	}
	if len(view.Trains) != 1 || view.Trains[0].Vehicle != "BE.NMBS.IC1" {
		t.Fatalf("got %+v, want only the Brussels route train", view.Trains)
	}
	if got := view.Trains[0].AvgDelaySec; got != 0 {
		t.Fatalf("avg delay = %d, want 0 (the Antwerp route leaked in)", got)
	}
}

// TestPunctualityRejectsUnknownBoardType guards the flag against silent
// typos, which would otherwise return an empty history that looks like "no
// trains were ever late".
func TestPunctualityRejectsUnknownBoardType(t *testing.T) {
	_, path := seedStore(t)

	cmd := RootCmd()
	cmd.SetArgs([]string{"punctuality", "--station", "Leuven", "--board-type", "departures", "--db", path, "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("unknown --board-type succeeded, want a usage error; output:\n%s", out.String())
	}
}

func runChanges(t *testing.T, args ...string) string {
	t.Helper()
	return runIrail(t, append([]string{"changes"}, args...)...)
}

func runPunctuality(t *testing.T, args ...string) string {
	t.Helper()
	return runIrail(t, append([]string{"punctuality"}, args...)...)
}

// runIrail executes one command against the real cobra tree and returns
// stdout. Stderr is kept separate so hints do not corrupt the JSON.
func runIrail(t *testing.T, args ...string) string {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(args)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%v failed: %v\nstderr:\n%s", args, err, stderr.String())
	}
	return stdout.String()
}
