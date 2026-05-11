// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: location-native-redesign — U8 wiring tests for `watch add`.
// Pins:
//   - --location resolves through resolveLocationFlags and decorates
//     the watchRow with location_resolved at subscription start.
//   - --metro is still parsed; legacy implicit --batch-accept-ambiguous keeps
//     ambiguous bare slugs resolving to a forced-pick rather than the
//     envelope path. Fires the once-per-process stderr deprecation
//     warning.
//   - Warn-and-continue under ambiguity: the watch is created with a
//     location_warning rather than refused.
//   - Omitting both flags preserves the no-decoration shape.
//
// PATCH: U15 — Watch location persistence tests pin:
//   - `watch add --location <city>` persists the resolved GeoContext
//     to `watches.location_context` as JSON so pollOneWatch can prefer
//     it over slug-suffix inference at tick time.
//   - resolveWatchAnchor prefers the persisted GeoContext lat/lng over
//     slug-suffix inference, with the legacy NYC anchor as final fallback.
//   - Schema migration adds the column to pre-existing tables idempotently
//     (PRAGMA table_info probe + ALTER TABLE).

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// runWatchAdd drives `watch add` with dry-run=true so the test doesn't
// touch the local SQLite store. The location pipeline is the only
// behavior exercised.
func runWatchAdd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	resetMetroDeprecationWarning()
	setDynamicMetros(nil, 0)
	t.Cleanup(func() { setDynamicMetros(nil, 0) })
	flags := &rootFlags{dryRun: true}
	cmd := newWatchAddCmd(flags)
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	cmd.SetContext(context.Background())
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	err = cmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

// unmarshalWatchRow parses captured stdout into a watchRow.
func unmarshalWatchRow(t *testing.T, raw string) watchRow {
	t.Helper()
	var row watchRow
	if err := json.Unmarshal([]byte(raw), &row); err != nil {
		t.Fatalf("unmarshal watchRow: %v\nraw: %s", err, raw)
	}
	return row
}

// TestWatchAdd_LocationDecoration pins the U8 happy paths on the watch
// command surface: --location and --metro both resolve through
// resolveLocationFlags and decorate the row.
func TestWatchAdd_LocationDecoration(t *testing.T) {
	cases := []struct {
		name         string
		args         []string
		wantResolved string
		wantSource   Source
		wantStderr   string
	}{
		{
			name:         "HIGH --location seattle",
			args:         []string{"tock:alinea", "--location", "seattle"},
			wantResolved: "Seattle",
			wantSource:   SourceExplicitFlag,
		},
		{
			name:         "legacy --metro seattle emits deprecation",
			args:         []string{"tock:alinea", "--metro", "seattle"},
			wantResolved: "Seattle",
			wantSource:   SourceExplicitFlag,
			wantStderr:   "deprecated",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, err := runWatchAdd(t, tc.args...)
			if err != nil {
				t.Fatalf("Execute: unexpected error: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
			}
			row := unmarshalWatchRow(t, stdout)
			if row.LocationResolved == nil {
				t.Fatalf("LocationResolved is nil; want resolved_to starting %q\nstdout: %s", tc.wantResolved, stdout)
			}
			if !strings.HasPrefix(row.LocationResolved.ResolvedTo, tc.wantResolved) {
				t.Errorf("ResolvedTo = %q; want prefix %q", row.LocationResolved.ResolvedTo, tc.wantResolved)
			}
			if row.LocationResolved.Source != tc.wantSource {
				t.Errorf("Source = %q; want %q", row.LocationResolved.Source, tc.wantSource)
			}
			if tc.wantStderr != "" && !strings.Contains(stderr, tc.wantStderr) {
				t.Errorf("stderr missing %q; got %q", tc.wantStderr, stderr)
			}
			if tc.wantStderr == "" && strings.Contains(stderr, "deprecated") {
				t.Errorf("stderr should not contain 'deprecated'; got %q", stderr)
			}
		})
	}
}

// TestWatchAdd_NoLocation pins the no-constraint shape: omitting both
// --location and --metro produces a watchRow with no location_resolved
// or location_warning fields — preserves the pre-U8 JSON shape.
func TestWatchAdd_NoLocation(t *testing.T) {
	stdout, stderr, err := runWatchAdd(t, "tock:alinea")
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	if strings.Contains(stdout, `"location_resolved"`) {
		t.Errorf("no-location path should omit location_resolved; got %s", stdout)
	}
	if strings.Contains(stdout, `"location_warning"`) {
		t.Errorf("no-location path should omit location_warning; got %s", stdout)
	}
}

// TestWatchAdd_AmbiguousWarnAndContinue pins the warn-and-continue
// contract: a bare ambiguous --location with --batch-accept-ambiguous
// produces a watchRow carrying both location_resolved AND
// location_warning, plus a stderr "location_warning:" line. The watch
// is created in the same call — never refused.
func TestWatchAdd_AmbiguousWarnAndContinue(t *testing.T) {
	stdout, stderr, err := runWatchAdd(t, "tock:alinea", "--location", "bellevue", "--batch-accept-ambiguous")
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	row := unmarshalWatchRow(t, stdout)
	if row.LocationResolved == nil {
		t.Fatalf("LocationResolved is nil under warn-and-continue; want forced-pick row\nstdout: %s", stdout)
	}
	if row.LocationWarning == nil {
		t.Errorf("LocationWarning is nil; warn-and-continue must annotate the row")
	}
	if !strings.Contains(stderr, "location_warning") {
		t.Errorf("stderr missing 'location_warning' line; got %q", stderr)
	}
	if row.State != "active" {
		t.Errorf("State = %q; want 'active' (warn-and-continue, not refused)", row.State)
	}
}

// TestWatchAdd_AmbiguousLocationEmitsEnvelope pins the envelope path:
// without --batch-accept-ambiguous, a bare ambiguous --location emits
// the DisambiguationEnvelope rather than persisting a watch. The
// caller must disambiguate before the watch is meaningful.
func TestWatchAdd_AmbiguousLocationEmitsEnvelope(t *testing.T) {
	stdout, stderr, err := runWatchAdd(t, "tock:alinea", "--location", "bellevue")
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "needs_clarification") {
		t.Fatalf("envelope output missing needs_clarification; got %s", stdout)
	}
	env := unmarshalEnvelope(t, stdout)
	if !env.NeedsClarification {
		t.Errorf("NeedsClarification = false; want true")
	}
	if env.ErrorKind != ErrorKindLocationAmbiguous {
		t.Errorf("ErrorKind = %q; want %q", env.ErrorKind, ErrorKindLocationAmbiguous)
	}
}

// runWatchAddPersist drives `watch add` with dry-run=false so the row
// is actually written to the local SQLite store. Uses withTempCacheDir
// to redirect HOME into a per-test temp directory.
func runWatchAddPersist(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	resetMetroDeprecationWarning()
	setDynamicMetros(nil, 0)
	t.Cleanup(func() { setDynamicMetros(nil, 0) })
	flags := &rootFlags{}
	cmd := newWatchAddCmd(flags)
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	cmd.SetContext(context.Background())
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	err = cmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

// TestWatchAdd_PersistsLocationContext pins U15: when `watch add` runs
// with --location, the resolved GeoContext is marshaled to JSON and
// stored in the new `location_context` column so pollOneWatch can
// anchor on the persisted lat/lng instead of slug-suffix inference.
func TestWatchAdd_PersistsLocationContext(t *testing.T) {
	withTempCacheDir(t)
	stdout, stderr, err := runWatchAddPersist(t, "tock:alinea", "--location", "portland, me")
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	row := unmarshalWatchRow(t, stdout)
	if row.LocationResolved == nil || !strings.HasPrefix(row.LocationResolved.ResolvedTo, "Portland, ME") {
		t.Fatalf("LocationResolved missing or wrong; got %+v\nstdout: %s", row.LocationResolved, stdout)
	}

	// Open the DB directly and inspect the row.
	dbPath := defaultDBPath("table-reservation-goat-pp-cli")
	db, oerr := sql.Open("sqlite", dbPath)
	if oerr != nil {
		t.Fatalf("open db: %v", oerr)
	}
	defer db.Close()

	var locationCtx sql.NullString
	if qerr := db.QueryRow(`SELECT location_context FROM watches WHERE id = ?`, row.ID).Scan(&locationCtx); qerr != nil {
		t.Fatalf("query location_context: %v", qerr)
	}
	if !locationCtx.Valid || locationCtx.String == "" {
		t.Fatalf("location_context is NULL/empty; want non-empty JSON\nrow ID: %s", row.ID)
	}
	var gc GeoContext
	if uerr := json.Unmarshal([]byte(locationCtx.String), &gc); uerr != nil {
		t.Fatalf("unmarshal location_context: %v\nraw: %s", uerr, locationCtx.String)
	}
	if !strings.HasPrefix(gc.ResolvedTo, "Portland, ME") {
		t.Errorf("persisted GeoContext.ResolvedTo = %q; want prefix %q", gc.ResolvedTo, "Portland, ME")
	}
	if gc.Centroid[0] == 0 || gc.Centroid[1] == 0 {
		t.Errorf("persisted GeoContext.Centroid is zero; got %v", gc.Centroid)
	}
}

// TestWatchAdd_NoLocationStoresNullContext pins back-compat: when
// --location is omitted, location_context stays NULL — same JSON shape
// pre-U15 callers expect (no location_resolved/warning fields).
func TestWatchAdd_NoLocationStoresNullContext(t *testing.T) {
	withTempCacheDir(t)
	stdout, stderr, err := runWatchAddPersist(t, "tock:alinea")
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	row := unmarshalWatchRow(t, stdout)
	if row.LocationResolved != nil {
		t.Errorf("LocationResolved should be nil without --location; got %+v", row.LocationResolved)
	}

	dbPath := defaultDBPath("table-reservation-goat-pp-cli")
	db, oerr := sql.Open("sqlite", dbPath)
	if oerr != nil {
		t.Fatalf("open db: %v", oerr)
	}
	defer db.Close()

	var locationCtx sql.NullString
	if qerr := db.QueryRow(`SELECT location_context FROM watches WHERE id = ?`, row.ID).Scan(&locationCtx); qerr != nil {
		t.Fatalf("query location_context: %v", qerr)
	}
	if locationCtx.Valid && locationCtx.String != "" {
		t.Errorf("location_context = %q; want NULL/empty for no-location path", locationCtx.String)
	}
}

// TestResolveWatchAnchor_PrefersPersistedLocation pins U15: when a
// persisted GeoContext JSON is present, resolveWatchAnchor returns its
// centroid even when the slug carries no city suffix. This is the
// load-bearing contract — a watch on `joey` (no suffix) created with
// `--location 'bellevue, wa'` must anchor on Bellevue WA, not NYC.
func TestResolveWatchAnchor_PrefersPersistedLocation(t *testing.T) {
	bellevue := GeoContext{
		Origin:     "bellevue, wa",
		ResolvedTo: "Bellevue, WA",
		Centroid:   [2]float64{47.6101, -122.2015},
		RadiusKm:   15,
		Score:      0.9,
		Source:     SourceExplicitFlag,
	}
	raw, err := json.Marshal(bellevue)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	lat, lng := resolveWatchAnchor("joey", string(raw))
	if lat != 47.6101 || lng != -122.2015 {
		t.Errorf("resolveWatchAnchor = (%v, %v); want Bellevue WA (47.6101, -122.2015)", lat, lng)
	}
	// Sanity: with empty JSON and the same suffix-less slug, NYC default
	// fires — proves the persisted path is the cause of the Bellevue anchor.
	latDefault, lngDefault := resolveWatchAnchor("joey", "")
	if latDefault != 40.7128 || lngDefault != -74.0060 {
		t.Errorf("resolveWatchAnchor with empty ctx = (%v, %v); want NYC default (40.7128, -74.0060)", latDefault, lngDefault)
	}
}

// TestResolveWatchAnchor_FallsBackToSlugSuffixOnNullContext pins
// backward compat for pre-migration rows: when location_context is
// empty AND the slug carries a known city suffix, the slug-suffix
// inference still fires (same path the pre-U15 pollOneWatch took).
func TestResolveWatchAnchor_FallsBackToSlugSuffixOnNullContext(t *testing.T) {
	// joey-bellevue → slug-suffix inference → Bellevue WA.
	lat, lng := resolveWatchAnchor("joey-bellevue", "")
	// We don't know the exact registry coords, but they must NOT be
	// NYC's. The pollOneWatch fallback for unknown suffixes is NYC, so
	// if this trip returns NYC, the slug-suffix path didn't fire.
	if lat == 40.7128 && lng == -74.0060 {
		t.Errorf("resolveWatchAnchor(joey-bellevue, '') = NYC; want slug-suffix inference (non-NYC coords)")
	}
	// Verify it's roughly Bellevue WA / Seattle metro (lat ~47, lng ~-122).
	if lat < 46 || lat > 48 || lng > -120 || lng < -124 {
		t.Errorf("resolveWatchAnchor(joey-bellevue, '') = (%v, %v); want Bellevue/Seattle metro (~47, ~-122)", lat, lng)
	}
}

// TestResolveWatchAnchor_MalformedJSONFallsBack pins error handling:
// a corrupted location_context blob falls back to slug-suffix and
// eventually NYC rather than erroring or panicking. Defensive: a
// pre-migration row or a future-format row should never break a tick.
func TestResolveWatchAnchor_MalformedJSONFallsBack(t *testing.T) {
	lat, lng := resolveWatchAnchor("joey", "{not valid json")
	// joey alone (no suffix) → no slug-suffix match → NYC.
	if lat != 40.7128 || lng != -74.0060 {
		t.Errorf("resolveWatchAnchor(joey, malformed) = (%v, %v); want NYC fallback", lat, lng)
	}
}

// TestWatchSchema_MigrationAddsColumn pins U15 idempotent migration:
// when openWatchStore runs against a pre-existing watches table that
// lacks location_context, ALTER TABLE adds the column. Calling
// openWatchStore again is a no-op (idempotent).
func TestWatchSchema_MigrationAddsColumn(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", tmp)

	dbPath := defaultDBPath("table-reservation-goat-pp-cli")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create the v1 schema (pre-U15) by hand: no location_context column.
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	const v1Schema = `
		CREATE TABLE watches (
			id TEXT PRIMARY KEY,
			venue TEXT NOT NULL,
			network TEXT NOT NULL,
			slug TEXT NOT NULL,
			party_size INTEGER NOT NULL,
			window_spec TEXT,
			notify TEXT,
			state TEXT NOT NULL DEFAULT 'active',
			created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			last_polled_at DATETIME,
			last_match_at DATETIME,
			match_count INTEGER NOT NULL DEFAULT 0
		);
	`
	if _, err := db.Exec(v1Schema); err != nil {
		t.Fatalf("v1 schema create: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO watches (id, venue, network, slug, party_size, state) VALUES ('wat_legacy', 'tock:alinea', 'tock', 'alinea', 2, 'active')`); err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}
	db.Close()

	// Run openWatchStore to trigger the migration.
	flags := &rootFlags{}
	migDB, err := openWatchStore(flags)
	if err != nil {
		t.Fatalf("openWatchStore: %v", err)
	}
	if !columnExists(t, migDB, "watches", "location_context") {
		t.Fatalf("location_context column missing after migration")
	}
	// Legacy row should survive and have NULL location_context.
	var ctx sql.NullString
	if err := migDB.QueryRow(`SELECT location_context FROM watches WHERE id = 'wat_legacy'`).Scan(&ctx); err != nil {
		t.Fatalf("query legacy row: %v", err)
	}
	if ctx.Valid && ctx.String != "" {
		t.Errorf("legacy row location_context = %q; want NULL", ctx.String)
	}
	migDB.Close()

	// Second open should be idempotent — no error.
	migDB2, err := openWatchStore(flags)
	if err != nil {
		t.Fatalf("openWatchStore (second call): %v", err)
	}
	migDB2.Close()
}

// runWatchList drives `watch list` against the local SQLite store
// pointed at by HOME/XDG_CACHE_HOME. Returns captured stdout/stderr
// plus the Execute error. Pairs with seedWatchRow to set up rows.
func runWatchList(t *testing.T) (stdout, stderr string, err error) {
	t.Helper()
	flags := &rootFlags{}
	cmd := newWatchListCmd(flags)
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(nil)
	cmd.SetContext(context.Background())
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	err = cmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

// seedWatchRow inserts a watches row with the given GeoContext (or
// no location_context when gc is nil). Ensures the schema first via
// openWatchStore so the location_context column exists.
func seedWatchRow(t *testing.T, id, venue, network, slug string, gc *GeoContext) {
	t.Helper()
	flags := &rootFlags{}
	db, err := openWatchStore(flags)
	if err != nil {
		t.Fatalf("openWatchStore: %v", err)
	}
	defer db.Close()
	var locationCtx sql.NullString
	if gc != nil {
		raw, mErr := json.Marshal(gc)
		if mErr != nil {
			t.Fatalf("marshal gc: %v", mErr)
		}
		locationCtx = sql.NullString{String: string(raw), Valid: true}
	}
	if _, err := db.Exec(
		`INSERT INTO watches (id, venue, network, slug, party_size, state, location_context)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, venue, network, slug, 2, "active", locationCtx,
	); err != nil {
		t.Fatalf("seed watch row: %v", err)
	}
}

// findWatchRow returns the watchRow with the given ID from a list
// payload, or fails t when missing.
func findWatchRow(t *testing.T, rows []watchRow, id string) watchRow {
	t.Helper()
	for _, r := range rows {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("watchRow with ID %q not in list (got %d rows)", id, len(rows))
	return watchRow{}
}

// TestWatchList_RehydratesLowTierGeoContext pins the U19 fix: a watch
// persisted with a forced-pick low-tier GeoContext (e.g., bare
// "bellevue" + --batch-accept-ambiguous) must round-trip through
// `watch list` with location_resolved populated. Pre-U19 the listing
// path called decorateForList(gc, acceptedAmbiguous=false), which
// returned (nil, nil) for TierLow because that signature is the
// envelope path; the persisted decision was silently dropped.
//
// On rehydration, location_warning stays nil — it was a one-shot
// stderr notice at subscription time, not a persisted contract.
func TestWatchList_RehydratesLowTierGeoContext(t *testing.T) {
	withTempCacheDir(t)
	gc := &GeoContext{
		Origin:     "bellevue",
		ResolvedTo: "Bellevue, WA",
		Centroid:   [2]float64{47.6101, -122.2015},
		RadiusKm:   15,
		Score:      0.42,
		Tier:       ResolutionTierLow,
		Source:     SourceExplicitFlag,
		Alternates: []Candidate{
			{Name: "Bellevue, NE", State: "NE"},
			{Name: "Bellevue, KY", State: "KY"},
		},
	}
	seedWatchRow(t, "wat_low0000000000", "tock:joey", "tock", "joey", gc)

	stdout, stderr, err := runWatchList(t)
	if err != nil {
		t.Fatalf("watch list: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	var rows []watchRow
	if jerr := json.Unmarshal([]byte(stdout), &rows); jerr != nil {
		t.Fatalf("unmarshal list: %v\nstdout: %s", jerr, stdout)
	}
	r := findWatchRow(t, rows, "wat_low0000000000")
	if r.LocationResolved == nil {
		t.Fatalf("LocationResolved is nil; low-tier rehydration must surface the persisted decision\nstdout: %s", stdout)
	}
	if r.LocationResolved.ResolvedTo != "Bellevue, WA" {
		t.Errorf("ResolvedTo = %q; want %q", r.LocationResolved.ResolvedTo, "Bellevue, WA")
	}
	if r.LocationResolved.Tier != ResolutionTierLow {
		t.Errorf("Tier = %q; want %q", r.LocationResolved.Tier, ResolutionTierLow)
	}
	if r.LocationWarning != nil {
		t.Errorf("LocationWarning = %+v; want nil (one-shot at watch-add time, not persisted)", r.LocationWarning)
	}
}

// TestWatchList_RehydratesHighTierGeoContext regression-guards the
// high-tier path: an explicit `--location 'bellevue, wa'` persists a
// HIGH-tier GeoContext, and `watch list` must surface it as
// location_resolved with Tier=high and no warning.
func TestWatchList_RehydratesHighTierGeoContext(t *testing.T) {
	withTempCacheDir(t)
	gc := &GeoContext{
		Origin:     "bellevue, wa",
		ResolvedTo: "Bellevue, WA",
		Centroid:   [2]float64{47.6101, -122.2015},
		RadiusKm:   15,
		Score:      0.9,
		Tier:       ResolutionTierHigh,
		Source:     SourceExplicitFlag,
	}
	seedWatchRow(t, "wat_high000000000", "tock:joey", "tock", "joey", gc)

	stdout, stderr, err := runWatchList(t)
	if err != nil {
		t.Fatalf("watch list: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	var rows []watchRow
	if jerr := json.Unmarshal([]byte(stdout), &rows); jerr != nil {
		t.Fatalf("unmarshal list: %v\nstdout: %s", jerr, stdout)
	}
	r := findWatchRow(t, rows, "wat_high000000000")
	if r.LocationResolved == nil {
		t.Fatalf("LocationResolved is nil; high-tier rehydration must surface the persisted decision\nstdout: %s", stdout)
	}
	if r.LocationResolved.Tier != ResolutionTierHigh {
		t.Errorf("Tier = %q; want %q", r.LocationResolved.Tier, ResolutionTierHigh)
	}
	if r.LocationWarning != nil {
		t.Errorf("LocationWarning = %+v; want nil for HIGH tier", r.LocationWarning)
	}
}

// TestWatchList_NoLocationContextLeavesFieldsNil regression-guards the
// pre-U8 no-decoration shape: rows with NULL location_context surface
// with neither location_resolved nor location_warning. Avoids
// regressing the pre-migration / no-flag back-compat shape.
func TestWatchList_NoLocationContextLeavesFieldsNil(t *testing.T) {
	withTempCacheDir(t)
	seedWatchRow(t, "wat_null000000000", "tock:alinea", "tock", "alinea", nil)

	stdout, stderr, err := runWatchList(t)
	if err != nil {
		t.Fatalf("watch list: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	var rows []watchRow
	if jerr := json.Unmarshal([]byte(stdout), &rows); jerr != nil {
		t.Fatalf("unmarshal list: %v\nstdout: %s", jerr, stdout)
	}
	r := findWatchRow(t, rows, "wat_null000000000")
	if r.LocationResolved != nil {
		t.Errorf("LocationResolved = %+v; want nil for NULL location_context", r.LocationResolved)
	}
	if r.LocationWarning != nil {
		t.Errorf("LocationWarning = %+v; want nil for NULL location_context", r.LocationWarning)
	}
}

// columnExists is a test helper that probes sqlite's PRAGMA table_info
// to check whether a column exists. Mirrors the production migration
// probe but uses the test's open *sql.DB handle.
func columnExists(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info("` + table + `")`)
	if err != nil {
		t.Fatalf("PRAGMA table_info(%s): %v", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan table_info: %v", err)
		}
		if name == column {
			return true
		}
	}
	return false
}
