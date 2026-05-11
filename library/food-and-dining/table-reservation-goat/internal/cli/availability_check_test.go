// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: location-native-redesign — U7 wiring tests for
// `availability check` (and the shared helpers covering multi-day).
// Pins:
//   - --location resolves to a typed GeoContext and decorates the row
//     with location_resolved (HIGH/MEDIUM/forced-LOW shapes).
//   - --metro is still parsed and routed through ResolveLocation; the
//     legacy implicit --batch-accept-ambiguous keeps ambiguous bare slugs
//     resolving to a forced-pick GeoContext rather than the envelope path.
//   - --metro fires the once-per-process stderr deprecation warning.
//   - omitting both flags preserves the no-decoration shape.
//   - --location bellevue without --batch-accept-ambiguous emits the
//     DisambiguationEnvelope JSON shape (needs_clarification + candidates),
//     not an earliestRow.
//   - empty venue argument returns a typed error.
//   - isNumericIDInput correctly classifies the numeric-ID exemption
//     boundary (bare digits and opentable:<digits> in, slugs and
//     tock:<digits> out).
//   - applyGeoToVenueRow soft-demotes (attaches LocationWarning) for
//     out-of-radius numeric IDs and decorates without warning for
//     in-radius numeric IDs / slug inputs.

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// runAvailabilityCheck drives the cobra command and returns
// (stdout, stderr, error). dryRun=true short-circuits the live
// provider calls so the test doesn't need a network or auth.Session —
// the dry-run path still flows through the location-resolution wiring,
// which is what we're pinning.
func runAvailabilityCheck(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	resetMetroDeprecationWarning()
	flags := &rootFlags{dryRun: true}
	cmd := newAvailabilityCheckCmd(flags)
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

// unmarshalEarliestRow parses captured stdout into an earliestRow.
// Fails the test on parse error — every non-envelope path must produce
// a valid earliestRow shape.
func unmarshalEarliestRow(t *testing.T, raw string) earliestRow {
	t.Helper()
	var row earliestRow
	if err := json.Unmarshal([]byte(raw), &row); err != nil {
		t.Fatalf("unmarshal earliestRow: %v\nraw: %s", err, raw)
	}
	return row
}

// TestAvailabilityCheck_LocationDecoration exercises the happy paths
// where --location or --metro resolves cleanly. All cases land on an
// earliestRow with location_resolved populated; the warning presence
// depends on whether the resolve had alternates.
func TestAvailabilityCheck_LocationDecoration(t *testing.T) {
	cases := []struct {
		name         string
		args         []string
		wantResolved string
		wantSource   Source
		wantWarning  bool   // location_warning expected (alternates present)
		wantStderr   string // substring; "" -> no stderr assertion
	}{
		{
			name:         "HIGH bare city Seattle (single registry match)",
			args:         []string{"canlis", "--location", "seattle"},
			wantResolved: "Seattle, WA",
			wantSource:   SourceExplicitFlag,
			wantWarning:  false,
		},
		{
			name:         "HIGH city+state Bellevue WA",
			args:         []string{"canlis", "--location", "bellevue, wa"},
			wantResolved: "Bellevue, WA",
			wantSource:   SourceExplicitFlag,
			wantWarning:  false,
		},
		{
			name:         "legacy --metro seattle behaves like --location seattle",
			args:         []string{"canlis", "--metro", "seattle"},
			wantResolved: "Seattle, WA",
			wantSource:   SourceExplicitFlag,
			wantWarning:  false,
			wantStderr:   "deprecated",
		},
		// U14: --metro bellevue is ambiguous; legacy implicit
		// --batch-accept-ambiguous is now suppressed and the envelope
		// path fires. See TestAvailabilityCheck_MetroAmbiguous below.
		{
			name:         "--location bellevue --batch-accept-ambiguous (forced pick)",
			args:         []string{"canlis", "--location", "bellevue", "--batch-accept-ambiguous"},
			wantResolved: "Bellevue, WA",
			wantSource:   SourceExplicitFlag,
			wantWarning:  true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, err := runAvailabilityCheck(t, tc.args...)
			if err != nil {
				t.Fatalf("Execute: unexpected error: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
			}
			row := unmarshalEarliestRow(t, stdout)
			if row.LocationResolved == nil {
				t.Fatalf("LocationResolved is nil; want resolved_to=%q\nstdout: %s", tc.wantResolved, stdout)
			}
			if row.LocationResolved.ResolvedTo != tc.wantResolved {
				t.Errorf("ResolvedTo = %q; want %q", row.LocationResolved.ResolvedTo, tc.wantResolved)
			}
			if row.LocationResolved.Source != tc.wantSource {
				t.Errorf("Source = %q; want %q", row.LocationResolved.Source, tc.wantSource)
			}
			if tc.wantWarning && row.LocationWarning == nil {
				t.Errorf("LocationWarning is nil; expected forced-pick warning")
			}
			if !tc.wantWarning && row.LocationWarning != nil {
				t.Errorf("LocationWarning unexpectedly set: %+v", row.LocationWarning)
			}
			if tc.wantStderr != "" && !strings.Contains(stderr, tc.wantStderr) {
				t.Errorf("stderr missing %q; got %q", tc.wantStderr, stderr)
			}
			if tc.wantStderr == "" && strings.Contains(stderr, "deprecated") {
				t.Errorf("stderr should not contain 'deprecated' for non-legacy path; got %q", stderr)
			}
		})
	}
}

// TestAvailabilityCheck_NoLocation pins the no-constraint shape:
// without --location or --metro, the row carries no location_resolved
// or location_warning field. omitempty leaves both absent from JSON.
func TestAvailabilityCheck_NoLocation(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"no flags", []string{"canlis"}},
		{"empty --location", []string{"canlis", "--location", ""}},
		{"whitespace-only --location", []string{"canlis", "--location", "   "}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, err := runAvailabilityCheck(t, tc.args...)
			if err != nil {
				t.Fatalf("Execute: unexpected error: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
			}
			if strings.Contains(stdout, `"location_resolved"`) {
				t.Errorf("no-location path should omit location_resolved; got %s", stdout)
			}
			if strings.Contains(stdout, `"location_warning"`) {
				t.Errorf("no-location path should omit location_warning; got %s", stdout)
			}
			row := unmarshalEarliestRow(t, stdout)
			if row.LocationResolved != nil {
				t.Errorf("LocationResolved should be nil; got %+v", row.LocationResolved)
			}
		})
	}
}

// TestAvailabilityCheck_AmbiguousEmitsEnvelope pins R14 F3 on this
// command surface: a bare ambiguous --location without
// --batch-accept-ambiguous emits the DisambiguationEnvelope JSON
// shape (not an earliestRow).
// The envelope carries needs_clarification=true plus the three Bellevue
// candidates.
func TestAvailabilityCheck_AmbiguousEmitsEnvelope(t *testing.T) {
	stdout, stderr, err := runAvailabilityCheck(t, "canlis", "--location", "bellevue")
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "needs_clarification") {
		t.Fatalf("envelope output missing needs_clarification field; got %s", stdout)
	}
	env := unmarshalEnvelope(t, stdout)
	if !env.NeedsClarification {
		t.Errorf("NeedsClarification = false; want true")
	}
	if env.ErrorKind != ErrorKindLocationAmbiguous {
		t.Errorf("ErrorKind = %q; want %q", env.ErrorKind, ErrorKindLocationAmbiguous)
	}
	if got := len(env.Candidates); got < 3 {
		t.Errorf("Candidates len = %d; want >= 3 (three Bellevues)", got)
	}
	// Sanity: envelope must not carry earliestRow fields.
	if strings.Contains(stdout, `"venue"`) {
		t.Errorf("envelope path should NOT include earliestRow fields; got %s", stdout)
	}
}

// TestAvailabilityCheck_EmptyVenueArg pins the typed-argument-error
// path: an empty/whitespace-only venue argument surfaces a clear error
// rather than running a doomed resolver call.
func TestAvailabilityCheck_EmptyVenueArg(t *testing.T) {
	_, _, err := runAvailabilityCheck(t, "")
	if err == nil {
		t.Fatalf("expected error for empty venue; got nil")
	}
	if !strings.Contains(err.Error(), "invalid venue") {
		t.Errorf("error should mention invalid venue; got %q", err.Error())
	}
}

// TestAvailabilityCheck_LocationParseError pins the typed-error path:
// an --location value that parses as coords but with out-of-range
// numbers surfaces the parse error to the caller.
func TestAvailabilityCheck_LocationParseError(t *testing.T) {
	_, _, err := runAvailabilityCheck(t, "canlis", "--location", "100.5,200.3")
	if err == nil {
		t.Fatalf("expected parse error for out-of-range coords; got nil")
	}
	if !strings.Contains(err.Error(), "latitude") && !strings.Contains(err.Error(), "longitude") {
		t.Errorf("error should mention latitude/longitude range; got %q", err.Error())
	}
}

// TestIsNumericIDInput pins the boundary classifier for the numeric-ID
// exemption. The downstream applyGeoToVenueRow branches on this helper
// to decide hard-reject (slug input) vs soft-demote (numeric-ID input)
// for out-of-radius rows.
func TestIsNumericIDInput(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"bare digits", "3688", true},
		{"opentable:<digits>", "opentable:3688", true},
		{"opentable:<long-digits>", "opentable:1183597", true},
		{"opentable:<small-digits>", "opentable:42", true},
		{"slug", "canlis", false},
		{"slug with hyphen", "joey-bellevue", false},
		{"opentable:slug", "opentable:le-bernardin", false},
		{"mixed alphanumeric", "abc123", false},
		{"digits with hyphen", "13-coins", false},
		{"tock:digits (Tock has no numeric-ID convention)", "tock:3688", false},
		{"tock:slug", "tock:alinea", false},
		{"empty", "", false},
		{"only-prefix", "opentable:", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNumericIDInput(tc.in); got != tc.want {
				t.Errorf("isNumericIDInput(%q) = %v; want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestApplyGeoToVenueRow_NumericIDExemption pins the soft-demote-with-
// warning shape for the numeric-ID exemption boundary. The decision
// table:
//
//   - nil gc                        -> no decoration, no warning.
//   - slug input + non-nil gc       -> decorate; no out-of-radius
//     warning (slug inputs already had their hard-reject enforced
//     inside resolveOTSlugGeoAware).
//   - numeric-ID input + in-radius  -> decorate, no warning.
//   - numeric-ID input + out-of-radius -> decorate + LocationWarning.
//   - numeric-ID input + missing lat/lng -> decorate, no warning (can't
//     make a geo judgement on missing data).
func TestApplyGeoToVenueRow_NumericIDExemption(t *testing.T) {
	// Seattle centroid, ~50km radius.
	seattleGC := &GeoContext{
		Origin:     "seattle",
		ResolvedTo: "Seattle, WA",
		Centroid:   [2]float64{47.6062, -122.3321},
		RadiusKm:   50.0,
		Score:      0.6,
		Tier:       ResolutionTierHigh,
		Source:     SourceExplicitFlag,
	}
	// In-radius point: Bellevue WA (~13km from Seattle).
	bellevueLat, bellevueLng := 47.6101, -122.2015
	// Out-of-radius point: Tampa FL (very far from Seattle).
	tampaLat, tampaLng := 27.9506, -82.4572

	cases := []struct {
		name             string
		venue            string
		gc               *GeoContext
		rowLat, rowLng   float64
		wantResolved     bool
		wantWarning      bool
		wantWarnContains string
	}{
		{
			name:         "nil gc — no decoration",
			venue:        "3688",
			gc:           nil,
			rowLat:       bellevueLat,
			rowLng:       bellevueLng,
			wantResolved: false,
			wantWarning:  false,
		},
		{
			name:         "slug input in-radius — decorate, no warning",
			venue:        "canlis",
			gc:           seattleGC,
			rowLat:       bellevueLat,
			rowLng:       bellevueLng,
			wantResolved: true,
			wantWarning:  false,
		},
		{
			name:         "slug input out-of-radius — decorate, no warning (slug hard-reject is upstream)",
			venue:        "canlis",
			gc:           seattleGC,
			rowLat:       tampaLat,
			rowLng:       tampaLng,
			wantResolved: true,
			wantWarning:  false,
		},
		{
			name:         "numeric ID in-radius — decorate, no warning",
			venue:        "3688",
			gc:           seattleGC,
			rowLat:       bellevueLat,
			rowLng:       bellevueLng,
			wantResolved: true,
			wantWarning:  false,
		},
		{
			name:             "numeric ID out-of-radius — decorate + warning",
			venue:            "3688",
			gc:               seattleGC,
			rowLat:           tampaLat,
			rowLng:           tampaLng,
			wantResolved:     true,
			wantWarning:      true,
			wantWarnContains: "outside your stated location",
		},
		{
			name:             "opentable:<id> prefix out-of-radius — decorate + warning",
			venue:            "opentable:3688",
			gc:               seattleGC,
			rowLat:           tampaLat,
			rowLng:           tampaLng,
			wantResolved:     true,
			wantWarning:      true,
			wantWarnContains: "outside your stated location",
		},
		{
			name:         "numeric ID with no lat/lng — decorate, skip warning (can't judge)",
			venue:        "3688",
			gc:           seattleGC,
			rowLat:       0,
			rowLng:       0,
			wantResolved: true,
			wantWarning:  false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := earliestRow{
				Venue:     tc.venue,
				Latitude:  tc.rowLat,
				Longitude: tc.rowLng,
			}
			out := applyGeoToVenueRow(row, tc.gc, false, tc.venue)
			if (out.LocationResolved != nil) != tc.wantResolved {
				t.Errorf("LocationResolved present = %v; want %v (row=%+v)", out.LocationResolved != nil, tc.wantResolved, out)
			}
			if (out.LocationWarning != nil) != tc.wantWarning {
				t.Errorf("LocationWarning present = %v; want %v (row=%+v)", out.LocationWarning != nil, tc.wantWarning, out)
			}
			if tc.wantWarnContains != "" && out.LocationWarning != nil {
				if !strings.Contains(out.LocationWarning.Reason, tc.wantWarnContains) {
					t.Errorf("warning reason missing %q; got %q", tc.wantWarnContains, out.LocationWarning.Reason)
				}
			}
		})
	}
}

// runAvailabilityMultiDay drives the multi-day cobra command for
// the location-wiring tests. Mirrors runAvailabilityCheck — dry-run
// short-circuits the live network so the location pipeline is the
// only behavior exercised.
func runAvailabilityMultiDay(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	resetMetroDeprecationWarning()
	flags := &rootFlags{dryRun: true}
	cmd := newAvailabilityMultiDayCmd(flags)
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

// unmarshalMultiDayResponse parses captured stdout into a
// multiDayResponse. Used only by the multi-day tests.
func unmarshalMultiDayResponse(t *testing.T, raw string) multiDayResponse {
	t.Helper()
	var resp multiDayResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal multiDayResponse: %v\nraw: %s", err, raw)
	}
	return resp
}

// TestAvailabilityMultiDay_LocationDecoration pins the location-wiring
// happy path on the multi-day command: --location seattle decorates
// the response at the top level (not per-day) and --metro emits the
// deprecation warning on first use.
func TestAvailabilityMultiDay_LocationDecoration(t *testing.T) {
	cases := []struct {
		name         string
		args         []string
		wantResolved string
		wantStderr   string
	}{
		{
			name:         "HIGH bare city Seattle",
			args:         []string{"canlis", "--start-date", "2026-05-15", "--days", "2", "--location", "seattle"},
			wantResolved: "Seattle, WA",
		},
		{
			name:         "legacy --metro seattle emits deprecation",
			args:         []string{"canlis", "--start-date", "2026-05-15", "--days", "2", "--metro", "seattle"},
			wantResolved: "Seattle, WA",
			wantStderr:   "deprecated",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, err := runAvailabilityMultiDay(t, tc.args...)
			if err != nil {
				t.Fatalf("Execute: unexpected error: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
			}
			resp := unmarshalMultiDayResponse(t, stdout)
			if resp.LocationResolved == nil {
				t.Fatalf("LocationResolved is nil at top level; want resolved_to=%q\nstdout: %s", tc.wantResolved, stdout)
			}
			if resp.LocationResolved.ResolvedTo != tc.wantResolved {
				t.Errorf("ResolvedTo = %q; want %q", resp.LocationResolved.ResolvedTo, tc.wantResolved)
			}
			// Per-day rows must NOT carry their own location_resolved —
			// the location applies once at resolve time, not per-day.
			for i, day := range resp.Results {
				if day.Result.LocationResolved != nil {
					t.Errorf("Results[%d].Result.LocationResolved should be nil (top-level only); got %+v", i, day.Result.LocationResolved)
				}
			}
			if tc.wantStderr != "" && !strings.Contains(stderr, tc.wantStderr) {
				t.Errorf("stderr missing %q; got %q", tc.wantStderr, stderr)
			}
		})
	}
}

// TestAvailabilityMultiDay_NoLocation pins the no-constraint shape:
// without --location or --metro, the response carries no
// location_resolved field at any level.
func TestAvailabilityMultiDay_NoLocation(t *testing.T) {
	stdout, stderr, err := runAvailabilityMultiDay(t, "canlis", "--start-date", "2026-05-15", "--days", "2")
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
