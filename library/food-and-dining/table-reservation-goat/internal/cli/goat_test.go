// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// runGoat drives the goat cobra command with dry-run=true so the
// location pipeline is the only behavior exercised. Resets the dynamic
// metro registry to the curated-only fallback so tests don't depend on
// a stale on-disk Tock SSR cache (the goat command calls
// hydrateMetrosFromTock at the top of RunE, which can swap in dynamic
// places whose State is empty — those would shift ResolveLocation's
// top-ranked candidate name out from under the fixture assertions).
// The t.Cleanup hook restores the dynamic-cleared state after each run
// so the goat command's hydrate side effect doesn't leak into the
// ResolveLocation tests downstream (which assume curated-only counts).
func runGoat(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	resetMetroDeprecationWarning()
	setDynamicMetros(nil, 0)
	t.Cleanup(func() { setDynamicMetros(nil, 0) })
	flags := &rootFlags{dryRun: true}
	cmd := newGoatCmd(flags)
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

// TestGoat_LocationDecoration pins the U8 happy paths: --location and
// --metro both resolve through resolveLocationFlags and decorate the
// goatResponse. The legacy --metro path implies --batch-accept-ambiguous
// so ambiguous bare slugs continue to land on a forced-pick rather than
// the envelope.
func TestGoat_LocationDecoration(t *testing.T) {
	cases := []struct {
		name         string
		args         []string
		wantResolved string
		wantSource   Source
		wantWarning  bool
		wantStderr   string
	}{
		{
			// Tock SSR hydration may push a State-less "Seattle" Place
			// to the top of the by-name lookup (dynamic entries
			// outrank curated when both share a slug). Assert a prefix
			// match on "Seattle" rather than a strict ", WA" suffix.
			name:         "HIGH bare city --location seattle",
			args:         []string{"sushi", "--location", "seattle"},
			wantResolved: "Seattle",
			wantSource:   SourceExplicitFlag,
			wantWarning:  false,
		},
		{
			name:         "legacy --metro seattle emits deprecation",
			args:         []string{"sushi", "--metro", "seattle"},
			wantResolved: "Seattle",
			wantSource:   SourceExplicitFlag,
			wantWarning:  false,
			wantStderr:   "deprecated",
		},
		// U14: --metro bellevue is ambiguous (multiple Bellevues by
		// name, no single canonical via Lookup); legacy implicit
		// --batch-accept-ambiguous is suppressed and the envelope path
		// fires. The TestGoat_AmbiguousLocationEmitsEnvelope test
		// below covers the envelope shape on this command surface.
		{
			name:         "--location bellevue --batch-accept-ambiguous forced-pick",
			args:         []string{"sushi", "--location", "bellevue", "--batch-accept-ambiguous"},
			wantResolved: "Bellevue",
			wantSource:   SourceExplicitFlag,
			wantWarning:  true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, err := runGoat(t, tc.args...)
			if err != nil {
				t.Fatalf("Execute: unexpected error: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
			}
			resp := unmarshalGoatResponse(t, stdout)
			if resp.LocationResolved == nil {
				t.Fatalf("LocationResolved is nil; want resolved_to=%q\nstdout: %s", tc.wantResolved, stdout)
			}
			if !strings.HasPrefix(resp.LocationResolved.ResolvedTo, tc.wantResolved) {
				t.Errorf("ResolvedTo = %q; want prefix %q", resp.LocationResolved.ResolvedTo, tc.wantResolved)
			}
			if resp.LocationResolved.Source != tc.wantSource {
				t.Errorf("Source = %q; want %q", resp.LocationResolved.Source, tc.wantSource)
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
				t.Errorf("stderr should not contain 'deprecated'; got %q", stderr)
			}
		})
	}
}

// TestGoat_AmbiguousLocationEmitsEnvelope pins the envelope path on the
// goat command surface: a bare ambiguous --location without
// --batch-accept-ambiguous emits the DisambiguationEnvelope shape.
func TestGoat_AmbiguousLocationEmitsEnvelope(t *testing.T) {
	stdout, stderr, err := runGoat(t, "sushi", "--location", "bellevue")
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
	if got := len(env.Candidates); got < 3 {
		t.Errorf("Candidates len = %d; want >= 3 (three Bellevues)", got)
	}
	if strings.Contains(stdout, `"sources_queried"`) {
		t.Errorf("envelope path should NOT include goatResponse fields; got %s", stdout)
	}
}

// TestGoat_NoLocation pins the no-constraint shape: omitting both
// --location and --metro produces a response with no location_resolved
// field — preserves the pre-U8 JSON shape.
func TestGoat_NoLocation(t *testing.T) {
	stdout, stderr, err := runGoat(t, "sushi")
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

func TestMetroCityName(t *testing.T) {
	cases := map[string]string{
		"seattle":  "Seattle",
		"chicago":  "Chicago",
		"new-york": "New York City",
		"nyc":      "New York City",
		// U17: "manhattan" and "dc" used to alias to their parent metros;
		// they now resolve to dedicated tighter Place entries
		// ("manhattan" -> Name "Manhattan", "dc" -> Name "Washington").
		"manhattan":     "Manhattan",
		"san-francisco": "San Francisco",
		"sf":            "San Francisco",
		"los-angeles":   "Los Angeles",
		"la":            "Los Angeles",
		"washington-dc": "Washington DC",
		"dc":            "Washington",
		"new-orleans":   "New Orleans",
		"nola":          "New Orleans",
		"  Seattle  ":   "Seattle", // whitespace + case-insensitive
		"unknown-metro": "",        // unknown returns empty for caller fallback
		"":              "",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			if got := metroCityName(in); got != want {
				t.Errorf("metroCityName(%q) = %q; want %q", in, got, want)
			}
		})
	}
}

func TestMetroCityName_AllKnownSlugsResolve(t *testing.T) {
	// Every slug in knownMetros() must resolve to a non-empty display name —
	// otherwise the city-search URL would carry an empty `?city=` value.
	for _, slug := range knownMetros() {
		t.Run(slug, func(t *testing.T) {
			if got := metroCityName(slug); got == "" {
				t.Errorf("metroCityName(%q) returned empty; every knownMetros slug must map", slug)
			}
		})
	}
}

func TestFirstToken(t *testing.T) {
	cases := map[string]string{
		"":                 "",
		"canlis":           "canlis",
		"tasting menu":     "tasting",
		"tasting\tmenu":    "tasting",
		"  leading spaces": "",
		"single":           "single",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			if got := firstToken(in); got != want {
				t.Errorf("firstToken(%q) = %q; want %q", in, got, want)
			}
		})
	}
}
