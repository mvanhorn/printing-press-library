// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: location-native-redesign — U8 wiring tests for `watch add`.
// Pins:
//   - --location resolves through resolveLocationFlags and decorates
//     the watchRow with location_resolved at subscription start.
//   - --metro is still parsed; legacy implicit --accept-ambiguous keeps
//     ambiguous bare slugs resolving to a forced-pick rather than the
//     envelope path. Fires the once-per-process stderr deprecation
//     warning.
//   - Warn-and-continue under ambiguity: the watch is created with a
//     location_warning rather than refused.
//   - Omitting both flags preserves the no-decoration shape.

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
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
// contract: a bare ambiguous --location with --accept-ambiguous
// produces a watchRow carrying both location_resolved AND
// location_warning, plus a stderr "location_warning:" line. The watch
// is created in the same call — never refused.
func TestWatchAdd_AmbiguousWarnAndContinue(t *testing.T) {
	stdout, stderr, err := runWatchAdd(t, "tock:alinea", "--location", "bellevue", "--accept-ambiguous")
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
// without --accept-ambiguous, a bare ambiguous --location emits the
// DisambiguationEnvelope rather than persisting a watch. The caller
// must disambiguate before the watch is meaningful.
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
