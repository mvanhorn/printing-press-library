// Copyright 2026 Felix Banuchi and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/health/peloton/internal/client"
	"github.com/mvanhorn/printing-press-library/library/health/peloton/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/health/peloton/internal/store"
)

func TestDefaultSyncResourcesIncludesWorkoutsAndClasses(t *testing.T) {
	got := defaultSyncResources()
	want := []string{"workouts", "classes"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("defaultSyncResources() = %v, want %v (bare `sync` must not emit no_bulk_list_endpoints when these are registered)", got, want)
	}
}

func TestExpandSyncResourcesWithDependentsCascadesFromWorkouts(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"workouts alone cascades to both dependents", []string{"workouts"}, []string{"workouts", "performance", "workout_details"}},
		{"workouts+classes cascades to all three dependents", []string{"workouts", "classes"}, []string{"workouts", "classes", "performance", "workout_details", "classes_detail"}},
		{"already-explicit performance is not duplicated", []string{"workouts", "performance"}, []string{"workouts", "performance", "workout_details"}},
		{"already-explicit workout_details is not duplicated", []string{"workouts", "workout_details"}, []string{"workouts", "workout_details", "performance"}},
		{"both explicit dependents are not duplicated", []string{"workouts", "performance", "workout_details"}, []string{"workouts", "performance", "workout_details"}},
		{"performance alone (no workouts) is left as-is", []string{"performance"}, []string{"performance"}},
		{"workout_details alone (no workouts) is left as-is", []string{"workout_details"}, []string{"workout_details"}},
		{"classes alone cascades to classes_detail", []string{"classes"}, []string{"classes", "classes_detail"}},
		{"already-explicit classes_detail is not duplicated", []string{"classes", "classes_detail"}, []string{"classes", "classes_detail"}},
		{"classes_detail alone (no classes) is left as-is", []string{"classes_detail"}, []string{"classes_detail"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := expandSyncResourcesWithDependents(append([]string(nil), tc.in...))
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("expandSyncResourcesWithDependents(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestNormalizeSyncResourceAliasesMapsStrengthToWorkoutDetails guards the
// user-facing "strength" alias: users think in terms of `offline strength`,
// but that command reads the same "workout_details" family that `offline
// workout`/`intervals` also read — there's no separate strength-only sync
// target. Also guards that mixing the alias with the canonical name doesn't
// produce a duplicate dependent sync.
func TestNormalizeSyncResourceAliasesMapsStrengthToWorkoutDetails(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"strength alone maps to workout_details", []string{"strength"}, []string{"workout_details"}},
		{"strength alongside workouts maps", []string{"workouts", "strength"}, []string{"workouts", "workout_details"}},
		{"strength and workout_details together dedupe", []string{"strength", "workout_details"}, []string{"workout_details"}},
		{"non-alias resources pass through unchanged", []string{"workouts", "classes"}, []string{"workouts", "classes"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeSyncResourceAliases(append([]string(nil), tc.in...))
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("normalizeSyncResourceAliases(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestSyncCommandNotesStrengthAliasSubstitution guards NEW ISSUE D from a
// live post-fix verification sweep: `sync --resources strength` silently
// reported sync_start/sync_complete events under "workout_details" with no
// indication the caller-named "strength" resource was substituted, and
// `sync --resources workout_details,strength` reported resources:1 with no
// hint that the two collapsed into one. The full `sync` command must emit
// an explicit sync_note event whenever "strength" appears in the
// caller-supplied --resources list, before any substitution/dedup happens.
func TestSyncCommandNotesStrengthAliasSubstitution(t *testing.T) {
	home := t.TempDir()
	restore, err := cliutil.SetHomeOverride(home)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	seedValidOAuthBundleForLiveFetchTests(t)

	root := newRootCmd(&rootFlags{})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"sync", "--resources", "strength", "--dry-run", "--home", home})
	// No workouts are synced in this fresh store, so the workout_details
	// dependent legitimately warns ("sync workouts first") and the overall
	// command reports a non-zero exit -- that's the documented, expected
	// contract for a dependent run with no parent data (see
	// TestSyncWorkoutDetailsDependentWithoutSyncedWorkoutsWarns). This test
	// only cares whether the alias substitution note was emitted, which
	// happens before that dependent phase even runs.
	_ = root.Execute()
	if !strings.Contains(out.String(), `"event":"sync_note"`) || !strings.Contains(out.String(), `"from":"strength"`) || !strings.Contains(out.String(), `"to":"workout_details"`) {
		t.Fatalf("expected a sync_note event announcing the strength->workout_details substitution, got: %s", out.String())
	}
}

// pathAwareSyncClient returns a canned response keyed by the exact request
// path, for tests that need per-workout performance_graph responses to
// differ by workout id (fixtureSyncClient always returns the same payload
// regardless of path, which can't exercise per-parent fan-out).
type pathAwareSyncClient struct {
	byPath map[string]json.RawMessage
}

func (c *pathAwareSyncClient) Get(_ context.Context, path string, _ map[string]string) (json.RawMessage, error) {
	if data, ok := c.byPath[path]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("no fixture response for path %q", path)
}

func (c *pathAwareSyncClient) RateLimit() float64 { return 0 }

// mustPlanDependentSync resolves the default (unbounded, non-full,
// unscoped) dependent sync plan against db for dependentResource, for
// tests that only care about the fan-out/storage behavior and not the
// planning logic itself (see sync_dependent_plan_test.go for planning
// tests).
func mustPlanDependentSync(t *testing.T, db *store.Store, dependentResource string) dependentSyncPlan {
	t.Helper()
	plan, err := planDependentSync(db, "workouts", dependentResource, false, nil, nil, 0, false)
	if err != nil {
		t.Fatalf("planDependentSync(%s): %v", dependentResource, err)
	}
	return plan
}

// TestSyncPerformanceDependentFansOutOverSyncedWorkouts guards the feature:
// performance_graph is per-workout only (no bulk list endpoint), so syncing
// it must enumerate every workout id already in the local store and fetch
// one performance_graph per id, storing each keyed by that workout id so
// `offline performance <id>` (family "performance") can read it back.
func TestSyncPerformanceDependentFansOutOverSyncedWorkouts(t *testing.T) {
	home := t.TempDir()
	dbPath := filepath.Join(home, "data", "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	workouts := &fixtureSyncClient{items: []json.RawMessage{
		json.RawMessage(`{"id":"w1","ride_id":"ride-a","start_time":"2026-01-01T10:00:00Z"}`),
		json.RawMessage(`{"id":"w2","ride_id":"ride-a","start_time":"2026-01-08T10:00:00Z"}`),
	}}
	if res := syncResource(context.Background(), workouts, db, "workouts", "", false, 0, false, false, nil, io.Discard); res.Err != nil {
		t.Fatalf("syncing workouts fixture: %v", res.Err)
	}

	perf := &pathAwareSyncClient{byPath: map[string]json.RawMessage{
		"/api/workout/w1/performance_graph": json.RawMessage(`{"seconds_since_pedaling_start":[0,1],"metrics":[{"display_name":"Output","values":[100,110]}]}`),
		"/api/workout/w2/performance_graph": json.RawMessage(`{"seconds_since_pedaling_start":[0,1],"metrics":[{"display_name":"Output","values":[90,95]}]}`),
	}}

	res := syncPerformanceDependent(context.Background(), perf, db, mustPlanDependentSync(t, db, "performance"), 4, nil, io.Discard)
	if res.Err != nil {
		t.Fatalf("syncPerformanceDependent: %v", res.Err)
	}
	if res.Count != 2 {
		t.Fatalf("performance synced count = %d, want 2", res.Count)
	}

	fact, err := db.GetProviderFact("performance", "w1")
	if err != nil {
		t.Fatalf("GetProviderFact(performance, w1): %v", err)
	}
	if !strings.Contains(string(fact.Payload), `"Output"`) {
		t.Fatalf("unexpected performance payload for w1: %s", fact.Payload)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := executeOffline(t, home, "offline", "performance", "w1")
	if err != nil {
		t.Fatalf("offline performance w1: %v", err)
	}
	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("data=%#v", got["data"])
	}
	if _, ok := data["metrics"]; !ok {
		t.Fatalf("offline performance w1 missing metrics: %#v", data)
	}
}

// TestSyncPerformanceDependentWithoutSyncedWorkoutsWarns guards the
// documented "run a dependent without re-syncing its parent" contract: with
// no workouts in the local store, the dependent sync must not silently no-op
// as a success — it should report an explicit warning identifying why.
func TestSyncPerformanceDependentWithoutSyncedWorkoutsWarns(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data", "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	res := syncPerformanceDependent(context.Background(), &pathAwareSyncClient{byPath: map[string]json.RawMessage{}}, db, mustPlanDependentSync(t, db, "performance"), 4, nil, io.Discard)
	if res.Err != nil {
		t.Fatalf("expected a warning, not an error: %v", res.Err)
	}
	if res.Warn == nil {
		t.Fatal("expected a warning when no workouts are synced yet")
	}
}

// TestSyncWorkoutDetailsDependentFansOutOverSyncedWorkouts guards the fix
// for BLOCKING #2: workout_details had no sync path at all, so `offline
// workout`/`intervals`/`repeat`/`strength` 404'd unconditionally regardless
// of any prior sync. Like performance, workout detail is per-workout only
// (GET /api/workout/{workout_id}, no bulk list endpoint), so syncing it must
// enumerate every workout id already in the local store and fetch one
// detail payload per id, storing each keyed by that workout id under the
// "workout_details" family the offline commands actually read.
func TestSyncWorkoutDetailsDependentFansOutOverSyncedWorkouts(t *testing.T) {
	home := t.TempDir()
	dbPath := filepath.Join(home, "data", "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	workouts := &fixtureSyncClient{items: []json.RawMessage{
		json.RawMessage(`{"id":"w1","ride_id":"ride-a","start_time":"2026-01-01T10:00:00Z"}`),
		json.RawMessage(`{"id":"w2","ride_id":"ride-a","start_time":"2026-01-08T10:00:00Z"}`),
	}}
	if res := syncResource(context.Background(), workouts, db, "workouts", "", false, 0, false, false, nil, io.Discard); res.Err != nil {
		t.Fatalf("syncing workouts fixture: %v", res.Err)
	}

	details := &pathAwareSyncClient{byPath: map[string]json.RawMessage{
		"/api/workout/w1": json.RawMessage(`{"id":"w1","movement_tracker_data":{"muscle_groups":[]}}`),
		"/api/workout/w2": json.RawMessage(`{"id":"w2","movement_tracker_data":{"muscle_groups":[]}}`),
	}}

	res := syncWorkoutDetailsDependent(context.Background(), details, db, mustPlanDependentSync(t, db, "workout_details"), 4, nil, io.Discard)
	if res.Err != nil {
		t.Fatalf("syncWorkoutDetailsDependent: %v", res.Err)
	}
	if res.Count != 2 {
		t.Fatalf("workout_details synced count = %d, want 2", res.Count)
	}

	fact, err := db.GetProviderFact("workout_details", "w1")
	if err != nil {
		t.Fatalf("GetProviderFact(workout_details, w1): %v", err)
	}
	if !strings.Contains(string(fact.Payload), `"movement_tracker_data"`) {
		t.Fatalf("unexpected workout_details payload for w1: %s", fact.Payload)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := executeOffline(t, home, "offline", "workout", "w1")
	if err != nil {
		t.Fatalf("offline workout w1: %v", err)
	}
	if _, ok := got["data"]; !ok {
		t.Fatalf("offline workout w1 missing data: %#v", got)
	}
}

// TestSyncWorkoutDetailsDependentWithoutSyncedWorkoutsWarns mirrors
// TestSyncPerformanceDependentWithoutSyncedWorkoutsWarns for the new
// workout_details dependent.
func TestSyncWorkoutDetailsDependentWithoutSyncedWorkoutsWarns(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data", "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	res := syncWorkoutDetailsDependent(context.Background(), &pathAwareSyncClient{byPath: map[string]json.RawMessage{}}, db, mustPlanDependentSync(t, db, "workout_details"), 4, nil, io.Discard)
	if res.Err != nil {
		t.Fatalf("expected a warning, not an error: %v", res.Err)
	}
	if res.Warn == nil {
		t.Fatal("expected a warning when no workouts are synced yet")
	}
}

// alwaysDataSyncClient returns the same payload for every path, regardless
// of the requested workout id — used to simulate --dry-run (every request
// gets the same {"dry_run":true} sentinel) and total credential failure
// (every request fails identically).
type alwaysDataSyncClient struct {
	data json.RawMessage
	err  error
}

func (c *alwaysDataSyncClient) Get(context.Context, string, map[string]string) (json.RawMessage, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.data, nil
}

func (c *alwaysDataSyncClient) RateLimit() float64 { return 0 }

// TestSyncPerformanceDependentDryRunWritesNothing guards against the
// dry-run sentinel client.dryRun() returns for every request under
// --dry-run being mistaken for real performance data and written to the
// store: it must be detected and skipped, not upserted as if it were a
// genuine performance_graph response.
func TestSyncPerformanceDependentDryRunWritesNothing(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data", "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	workouts := &fixtureSyncClient{items: []json.RawMessage{
		json.RawMessage(`{"id":"w1","ride_id":"ride-a","start_time":"2026-01-01T10:00:00Z"}`),
	}}
	if res := syncResource(context.Background(), workouts, db, "workouts", "", false, 0, false, false, nil, io.Discard); res.Err != nil {
		t.Fatalf("syncing workouts fixture: %v", res.Err)
	}

	dryRunClient := &alwaysDataSyncClient{data: json.RawMessage(`{"dry_run": true}`)}
	res := syncPerformanceDependent(context.Background(), dryRunClient, db, mustPlanDependentSync(t, db, "performance"), 4, nil, io.Discard)
	if res.Err != nil || res.Warn != nil {
		t.Fatalf("dry-run result should be clean, got err=%v warn=%v", res.Err, res.Warn)
	}
	if res.Count != 0 {
		t.Fatalf("dry-run Count = %d, want 0", res.Count)
	}
	if _, err := db.GetProviderFact("performance", "w1"); err == nil {
		t.Fatal("dry-run sentinel was written to the store as if it were real performance data")
	}
}

// TestSyncPerformanceDependentSurfacesPlaceholderCredentialError guards
// consistency with the flat resource loop: a placeholder-credential
// condition affects every request identically (a local config problem, not
// a per-workout API gap) and must surface as a hard error the same way it
// does for flat resources, not get silently downgraded to a generic
// "fetched 0/N" warning that a caller could mistake for "these workouts
// just have no performance data".
func TestSyncPerformanceDependentSurfacesPlaceholderCredentialError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data", "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	workouts := &fixtureSyncClient{items: []json.RawMessage{
		json.RawMessage(`{"id":"w1","ride_id":"ride-a","start_time":"2026-01-01T10:00:00Z"}`),
	}}
	if res := syncResource(context.Background(), workouts, db, "workouts", "", false, 0, false, false, nil, io.Discard); res.Err != nil {
		t.Fatalf("syncing workouts fixture: %v", res.Err)
	}

	failingClient := &alwaysDataSyncClient{err: client.ErrPlaceholderCredential}
	res := syncPerformanceDependent(context.Background(), failingClient, db, mustPlanDependentSync(t, db, "performance"), 4, nil, io.Discard)
	if res.Err == nil {
		t.Fatal("expected a hard error for a placeholder-credential failure, got none")
	}
	if !errors.Is(res.Err, client.ErrPlaceholderCredential) {
		t.Fatalf("res.Err = %v, want it to wrap client.ErrPlaceholderCredential", res.Err)
	}
}

// paramCapturingSyncClient records the params map passed to each Get call,
// keyed by path, for tests that need to assert on outgoing query params
// rather than just responses.
type paramCapturingSyncClient struct {
	data      json.RawMessage
	mu        sync.Mutex
	gotParams map[string]map[string]string
}

func (c *paramCapturingSyncClient) Get(_ context.Context, path string, params map[string]string) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.gotParams == nil {
		c.gotParams = map[string]map[string]string{}
	}
	c.gotParams[path] = params
	return c.data, nil
}

func (c *paramCapturingSyncClient) RateLimit() float64 { return 0 }

// TestSyncPerformanceDependentAppliesGlobalAndResourceParams guards
// --global-param and --resource-param actually reaching the per-workout
// performance_graph request. --global-param's own flag help promises
// injection "into every sync request including dependent path-scoped
// calls", and syncUserParams.applyTo's isDependent parameter exists
// specifically for this case -- but the dependent fan-out originally
// called c.Get with a bare nil params map, silently dropping both flags
// for the one resource that's actually a dependent.
// TestSyncPerformanceDependentDefaultsEveryNToOne guards against the sync
// path silently inheriting Peloton's own API default for every_n (~50x
// fewer samples than the single-fetch `workouts performance` command, which
// defaults its --every-n flag to 1). The dependent fan-out never goes
// through cobra flag machinery, so without an explicit default here, every
// performance_graph record written by sync was downsampled relative to a
// manual single fetch.
func TestSyncPerformanceDependentDefaultsEveryNToOne(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data", "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	workouts := &fixtureSyncClient{items: []json.RawMessage{
		json.RawMessage(`{"id":"w1","ride_id":"ride-a","start_time":"2026-01-01T10:00:00Z"}`),
	}}
	if res := syncResource(context.Background(), workouts, db, "workouts", "", false, 0, false, false, nil, io.Discard); res.Err != nil {
		t.Fatalf("syncing workouts fixture: %v", res.Err)
	}

	capturing := &paramCapturingSyncClient{data: json.RawMessage(`{"metrics":[]}`)}
	res := syncPerformanceDependent(context.Background(), capturing, db, mustPlanDependentSync(t, db, "performance"), 1, nil, io.Discard)
	if res.Err != nil {
		t.Fatalf("syncPerformanceDependent: %v", res.Err)
	}

	got := capturing.gotParams["/api/workout/w1/performance_graph"]
	if got["every_n"] != "1" {
		t.Fatalf("expected default every_n=1, got params %v", got)
	}
}

func TestSyncPerformanceDependentAppliesGlobalAndResourceParams(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data", "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	workouts := &fixtureSyncClient{items: []json.RawMessage{
		json.RawMessage(`{"id":"w1","ride_id":"ride-a","start_time":"2026-01-01T10:00:00Z"}`),
	}}
	if res := syncResource(context.Background(), workouts, db, "workouts", "", false, 0, false, false, nil, io.Discard); res.Err != nil {
		t.Fatalf("syncing workouts fixture: %v", res.Err)
	}

	userParams, err := parseSyncUserParams(nil, []string{"performance:every_n=5"}, []string{"tenant=acme"})
	if err != nil {
		t.Fatal(err)
	}
	capturing := &paramCapturingSyncClient{data: json.RawMessage(`{"metrics":[]}`)}
	res := syncPerformanceDependent(context.Background(), capturing, db, mustPlanDependentSync(t, db, "performance"), 1, userParams, io.Discard)
	if res.Err != nil {
		t.Fatalf("syncPerformanceDependent: %v", res.Err)
	}

	got := capturing.gotParams["/api/workout/w1/performance_graph"]
	if got["every_n"] != "5" {
		t.Fatalf("--resource-param performance:every_n=5 not applied: got params %v", got)
	}
	if got["tenant"] != "acme" {
		t.Fatalf("--global-param tenant=acme not applied: got params %v", got)
	}
}
