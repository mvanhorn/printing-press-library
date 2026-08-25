// Copyright 2026 Felix Banuchi and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/peloton/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/health/peloton/internal/store"
)

func seedValidOAuthBundleForLiveFetchTests(t *testing.T) {
	t.Helper()
	bundlePath, err := oauthBundlePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := saveOAuthBundle(pelotonTokenBundle{
		AccessToken:  "fixture-access",
		RefreshToken: "fixture-refresh",
		ExpiresAt:    time.Now().Add(time.Hour),
		SessionID:    "fixture-session",
	}); err != nil {
		t.Fatalf("seeding bundle at %s: %v", bundlePath, err)
	}
}

// TestWorkoutsShowLiveFetchPopulatesWorkoutDetails guards the fix for
// offline workout/intervals/strength all reading a "workout_details" family
// that, before this fix, nothing ever wrote to under that exact name —
// workouts_show.go's own generic write-through cached under "workouts"
// instead. Without cacheWorkoutDetail, `offline workout <id>` 404s on every
// workout unconditionally, regardless of any prior sync or live fetch.
func TestWorkoutsShowLiveFetchPopulatesWorkoutDetails(t *testing.T) {
	home := t.TempDir()
	restore, err := cliutil.SetHomeOverride(home)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"w1","ride_id":"ride-a","movement_tracker_data":[{"name":"squat"}]}`))
	}))
	defer server.Close()
	t.Setenv("PELOTON_BASE_URL", server.URL)
	seedValidOAuthBundleForLiveFetchTests(t)

	root := newRootCmd(&rootFlags{})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"workouts", "show", "w1", "--home", home, "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("workouts show: %v\noutput: %s", err, out.String())
	}

	db, err := store.OpenReadOnly(defaultDBPath("peloton-pp-cli"))
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer db.Close()

	fact, err := db.GetProviderFact("workout_details", "w1")
	if err != nil {
		t.Fatalf("live fetch did not populate offline workout_details for w1: %v", err)
	}
	if len(fact.Payload) == 0 {
		t.Fatal("stored workout_details fact has empty payload")
	}
}

// TestStrengthLiveFetchPopulatesWorkoutDetails mirrors the workouts-show
// test above for the `strength` command, which hits the identical
// GET /api/workout/{workout_id} endpoint under a different resourceType
// ("strength") but must populate the same shared "workout_details" family
// offline reads expect.
func TestStrengthLiveFetchPopulatesWorkoutDetails(t *testing.T) {
	home := t.TempDir()
	restore, err := cliutil.SetHomeOverride(home)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"w2","movement_tracker_data":[{"name":"lunge"}]}`))
	}))
	defer server.Close()
	t.Setenv("PELOTON_BASE_URL", server.URL)
	seedValidOAuthBundleForLiveFetchTests(t)

	root := newRootCmd(&rootFlags{})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"strength", "w2", "--home", home, "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("strength: %v\noutput: %s", err, out.String())
	}

	db, err := store.OpenReadOnly(defaultDBPath("peloton-pp-cli"))
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer db.Close()

	fact, err := db.GetProviderFact("workout_details", "w2")
	if err != nil {
		t.Fatalf("live fetch did not populate offline workout_details for w2: %v", err)
	}
	if len(fact.Payload) == 0 {
		t.Fatal("stored workout_details fact has empty payload")
	}
}
