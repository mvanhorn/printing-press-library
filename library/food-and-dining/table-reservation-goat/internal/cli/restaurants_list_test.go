// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: location-native-redesign — U6 wiring tests for `restaurants list`.
// Pins:
//   - --location resolves to a typed GeoContext and decorates the response
//     with location_resolved (HIGH/MEDIUM/forced-LOW shapes).
//   - --metro is still parsed and routed through ResolveLocation; the
//     legacy implicit --accept-ambiguous keeps ambiguous bare slugs
//     resolving to a forced-pick GeoContext rather than the envelope path.
//   - --metro fires a one-time stderr deprecation warning.
//   - omitting both flags preserves the no-filter no-decoration shape.
//   - --location with out-of-range coords surfaces a typed parse error
//     (not a silent fallthrough).
//   - --location bellevue without --accept-ambiguous emits the
//     DisambiguationEnvelope JSON shape (needs_clarification + candidates),
//     not a goatResponse.

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// runRestaurantsList drives the cobra command through a string-args list
// and returns (stdout, stderr, error). dryRun=true short-circuits the
// real provider calls so the test doesn't need a live network or a
// mocked auth.Session — the dry-run path still flows through the
// location resolution wiring, which is what we're pinning.
func runRestaurantsList(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	resetMetroDeprecationWarning()
	flags := &rootFlags{dryRun: true}
	cmd := newRestaurantsListCmd(flags)
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	cmd.SetContext(context.Background())
	// Silence cobra's "Error: ..." reprint to stderr — we want the
	// stderr buffer to capture ONLY the warnings we emit ourselves.
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	err = cmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

// unmarshalGoatResponse parses captured stdout into a goatResponse.
// Fails the test on parse error — every non-envelope path must produce
// a valid goatResponse shape.
func unmarshalGoatResponse(t *testing.T, raw string) goatResponse {
	t.Helper()
	var resp goatResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal goatResponse: %v\nraw: %s", err, raw)
	}
	return resp
}

// unmarshalEnvelope parses captured stdout into a DisambiguationEnvelope.
// Used by the envelope-path test only.
func unmarshalEnvelope(t *testing.T, raw string) DisambiguationEnvelope {
	t.Helper()
	var env DisambiguationEnvelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("unmarshal envelope: %v\nraw: %s", err, raw)
	}
	return env
}

// TestRestaurantsList_LocationDecoration exercises the happy paths
// where --location or --metro resolves cleanly to a single Place or to
// a forced-pick (legacy --metro + bare ambiguous name). All cases land
// on a goatResponse with location_resolved populated; the warning
// presence depends on whether the resolve had alternates.
func TestRestaurantsList_LocationDecoration(t *testing.T) {
	cases := []struct {
		name          string
		args          []string
		wantResolved  string
		wantSource    Source
		minConfidence float64
		wantWarning   bool   // location_warning expected (alternates present)
		wantStderr    string // substring; "" -> no stderr assertion
	}{
		{
			name:          "HIGH city+state Bellevue WA",
			args:          []string{"--query", "sushi", "--location", "bellevue, wa"},
			wantResolved:  "Bellevue, WA",
			wantSource:    SourceExplicitFlag,
			minConfidence: 0.3,
			wantWarning:   false, // state filter collapses to one candidate
		},
		{
			name:          "HIGH bare city Seattle (single registry match)",
			args:          []string{"--query", "sushi bellevue", "--location", "seattle"},
			wantResolved:  "Seattle, WA",
			wantSource:    SourceExplicitFlag,
			minConfidence: 0.4,
			wantWarning:   false,
		},
		{
			name:          "legacy --metro seattle behaves like --location seattle",
			args:          []string{"--query", "sushi", "--metro", "seattle"},
			wantResolved:  "Seattle, WA",
			wantSource:    SourceExplicitFlag,
			minConfidence: 0.4,
			wantWarning:   false,
			wantStderr:    "deprecated",
		},
		{
			name:          "legacy --metro bellevue implies --accept-ambiguous (forced pick)",
			args:          []string{"--query", "sushi", "--metro", "bellevue"},
			wantResolved:  "Bellevue, WA", // top-ranked by population
			wantSource:    SourceExplicitFlag,
			minConfidence: 0.0, // forced-LOW pick has low confidence
			wantWarning:   true,
			wantStderr:    "deprecated",
		},
		{
			name:          "--location bellevue --accept-ambiguous (forced pick)",
			args:          []string{"--query", "sushi", "--location", "bellevue", "--accept-ambiguous"},
			wantResolved:  "Bellevue, WA",
			wantSource:    SourceExplicitFlag,
			minConfidence: 0.0,
			wantWarning:   true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, err := runRestaurantsList(t, tc.args...)
			if err != nil {
				t.Fatalf("Execute: unexpected error: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
			}
			resp := unmarshalGoatResponse(t, stdout)
			if resp.LocationResolved == nil {
				t.Fatalf("LocationResolved is nil; want resolved_to=%q\nstdout: %s", tc.wantResolved, stdout)
			}
			if resp.LocationResolved.ResolvedTo != tc.wantResolved {
				t.Errorf("ResolvedTo = %q; want %q", resp.LocationResolved.ResolvedTo, tc.wantResolved)
			}
			if resp.LocationResolved.Source != tc.wantSource {
				t.Errorf("Source = %q; want %q", resp.LocationResolved.Source, tc.wantSource)
			}
			if resp.LocationResolved.Confidence < tc.minConfidence {
				t.Errorf("Confidence = %v; want >= %v", resp.LocationResolved.Confidence, tc.minConfidence)
			}
			if tc.wantWarning && resp.LocationWarning == nil {
				t.Errorf("LocationWarning is nil; expected forced-pick warning")
			}
			if !tc.wantWarning && resp.LocationWarning != nil {
				t.Errorf("LocationWarning unexpectedly set: %+v", resp.LocationWarning)
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

// TestRestaurantsList_NoLocation pins the no-constraint shape: without
// --location and --metro, the response carries NO location_resolved or
// location_warning field (omitempty leaves both absent from JSON). This
// preserves the pre-U6 output shape for callers who never opted into
// location filtering.
func TestRestaurantsList_NoLocation(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"no flags", []string{"--query", "sushi"}},
		{"empty --location", []string{"--query", "sushi", "--location", ""}},
		{"whitespace-only --location", []string{"--query", "sushi", "--location", "   "}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, err := runRestaurantsList(t, tc.args...)
			if err != nil {
				t.Fatalf("Execute: unexpected error: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
			}
			// JSON field check via raw substring — omitempty must omit
			// the field name entirely, not emit `"location_resolved":null`.
			if strings.Contains(stdout, `"location_resolved"`) {
				t.Errorf("no-location path should omit location_resolved; got %s", stdout)
			}
			if strings.Contains(stdout, `"location_warning"`) {
				t.Errorf("no-location path should omit location_warning; got %s", stdout)
			}
			// The decoration helper must be safe under nil gc; structural
			// sanity: a goatResponse still unmarshals.
			resp := unmarshalGoatResponse(t, stdout)
			if resp.LocationResolved != nil {
				t.Errorf("LocationResolved should be nil; got %+v", resp.LocationResolved)
			}
		})
	}
}

// TestRestaurantsList_AmbiguousEmitsEnvelope pins R14 F3: a bare
// ambiguous --location without --accept-ambiguous emits the
// DisambiguationEnvelope JSON shape (not a goatResponse). The envelope
// carries needs_clarification=true plus the three Bellevue candidates.
func TestRestaurantsList_AmbiguousEmitsEnvelope(t *testing.T) {
	stdout, stderr, err := runRestaurantsList(t, "--query", "sushi", "--location", "bellevue")
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	// Whitespace-tolerant check before unmarshal — printJSONFiltered
	// pretty-prints, so a compact-substring assertion would miss the
	// space after the colon. The unmarshal-and-field-check below pins
	// the actual contract.
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
	// Sanity: the envelope must not also carry a results array — that
	// would mean we serialized a goatResponse with envelope fields
	// merged in, which is the wrong shape.
	if strings.Contains(stdout, `"sources_queried"`) {
		t.Errorf("envelope path should NOT include goatResponse fields; got %s", stdout)
	}
}

// TestRestaurantsList_LocationParseError pins the typed-error path: a
// --location value that parses as coords but with out-of-range numbers
// surfaces the parse error to the caller, NOT a silent fallthrough to
// LocKindCity (which would treat "100.5,200.3" as a bare city name and
// hit location_unknown — a different error class).
func TestRestaurantsList_LocationParseError(t *testing.T) {
	_, _, err := runRestaurantsList(t, "--query", "sushi", "--location", "100.5,200.3")
	if err == nil {
		t.Fatalf("expected parse error for out-of-range coords; got nil")
	}
	if !strings.Contains(err.Error(), "latitude") && !strings.Contains(err.Error(), "longitude") {
		t.Errorf("error should mention latitude/longitude range; got %q", err.Error())
	}
}

// TestRestaurantsList_MetroDeprecationFiresOnce pins the once-per-
// process semantic of the --metro warning. The first invocation emits
// "deprecated"; a second invocation in the same process (no reset)
// stays silent. The runRestaurantsList helper resets the gate before
// each call to ensure cross-test isolation, so this test asserts the
// silence path by NOT calling the reset between the two runs.
func TestRestaurantsList_MetroDeprecationFiresOnce(t *testing.T) {
	resetMetroDeprecationWarning()
	flags := &rootFlags{dryRun: true}

	run := func() string {
		cmd := newRestaurantsListCmd(flags)
		var outBuf, errBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetErr(&errBuf)
		cmd.SetArgs([]string{"--query", "sushi", "--metro", "seattle"})
		cmd.SetContext(context.Background())
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute: %v\nstderr: %s", err, errBuf.String())
		}
		return errBuf.String()
	}

	first := run()
	if !strings.Contains(first, "deprecated") {
		t.Errorf("first --metro call should emit deprecation warning; got %q", first)
	}
	second := run()
	if strings.Contains(second, "deprecated") {
		t.Errorf("second --metro call should be silent; got %q", second)
	}
}

// TestInferTierFromGeoContext pins the heuristic that decorateForList
// uses to map a returned GeoContext back to a tier for the
// DecorateWithLocationContext call. The split is documented inline in
// inferTierFromGeoContext; this test pins the behavior so a tweak to
// the cutoff doesn't accidentally flip a fixture.
func TestInferTierFromGeoContext(t *testing.T) {
	cases := []struct {
		name              string
		gc                *GeoContext
		acceptedAmbiguous bool
		want              TierEnum
	}{
		{
			name: "nil gc -> unknown",
			gc:   nil,
			want: TierUnknown,
		},
		{
			name: "no alternates -> high",
			gc:   &GeoContext{Confidence: 0.6, Alternates: nil},
			want: TierHigh,
		},
		{
			name: "alternates, no bypass -> medium (envelope would have fired for low)",
			gc: &GeoContext{
				Confidence: 0.5,
				Alternates: []Candidate{{Name: "Bellevue, NE"}},
			},
			acceptedAmbiguous: false,
			want:              TierMedium,
		},
		{
			name: "alternates, bypass, high confidence -> medium",
			gc: &GeoContext{
				Confidence: 0.5,
				Alternates: []Candidate{{Name: "Bellevue, NE"}},
			},
			acceptedAmbiguous: true,
			want:              TierMedium,
		},
		{
			name: "alternates, bypass, low confidence -> low (forced pick)",
			gc: &GeoContext{
				Confidence: 0.2,
				Alternates: []Candidate{
					{Name: "Bellevue, NE"},
					{Name: "Bellevue, KY"},
				},
			},
			acceptedAmbiguous: true,
			want:              TierLow,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := inferTierFromGeoContext(tc.gc, tc.acceptedAmbiguous); got != tc.want {
				t.Errorf("inferTierFromGeoContext = %v; want %v", got, tc.want)
			}
		})
	}
}
