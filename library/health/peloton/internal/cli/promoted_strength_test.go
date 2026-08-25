// Copyright 2026 Felix Banuchi and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/health/peloton/internal/cliutil"
)

// TestStrengthWarnsWhenMovementTrackerDataAbsent guards MINOR #11 from a
// live post-fix verification sweep: most workouts (anything that isn't a
// tracked strength class) have no movement_tracker_data at all, and this
// live single-fetch command had no signal for that case — a bike ride with
// no tracker data looked identical to a real fetch problem, with nothing
// distinguishing "there's nothing here" from "something went wrong".
// offline_strength (offline.go) already surfaces this as an explicit
// caveat; this test drives the real `strength` command end-to-end against
// a fake server and asserts the equivalent stderr warning fires.
func TestStrengthWarnsWhenMovementTrackerDataAbsent(t *testing.T) {
	home := t.TempDir()
	restore, err := cliutil.SetHomeOverride(home)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"w1","fitness_discipline":"running"}`))
	}))
	defer server.Close()
	t.Setenv("PELOTON_BASE_URL", server.URL)
	seedValidOAuthBundleForLiveFetchTests(t)

	root := newRootCmd(&rootFlags{})
	var out, stderr bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&stderr)
	root.SetArgs([]string{"strength", "w1", "--home", home, "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("strength: %v\nstderr: %s", err, stderr.String())
	}

	if !bytes.Contains(stderr.Bytes(), []byte("no movement tracker data")) {
		t.Fatalf("expected a caveat warning when movement_tracker_data is absent, got stderr: %q", stderr.String())
	}
}

// TestStrengthDoesNotWarnWhenMovementTrackerDataPresent guards against the
// new caveat check false-positiving on a workout that actually does carry
// movement tracker data (e.g. a real strength class).
func TestStrengthDoesNotWarnWhenMovementTrackerDataPresent(t *testing.T) {
	home := t.TempDir()
	restore, err := cliutil.SetHomeOverride(home)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"w1","fitness_discipline":"strength","movement_tracker_data":{"muscle_groups":["chest"]}}`))
	}))
	defer server.Close()
	t.Setenv("PELOTON_BASE_URL", server.URL)
	seedValidOAuthBundleForLiveFetchTests(t)

	root := newRootCmd(&rootFlags{})
	var out, stderr bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&stderr)
	root.SetArgs([]string{"strength", "w1", "--home", home, "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("strength: %v\nstderr: %s", err, stderr.String())
	}

	if bytes.Contains(stderr.Bytes(), []byte("no movement tracker data")) {
		t.Fatalf("unexpected caveat warning when movement_tracker_data is present: %q", stderr.String())
	}
}
