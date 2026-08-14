// Copyright 2026 Felix Banuchi and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
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
		{"workouts alone cascades to performance", []string{"workouts"}, []string{"workouts", "performance"}},
		{"workouts+classes cascades to performance", []string{"workouts", "classes"}, []string{"workouts", "classes", "performance"}},
		{"already-explicit performance is not duplicated", []string{"workouts", "performance"}, []string{"workouts", "performance"}},
		{"performance alone (no workouts) is left as-is", []string{"performance"}, []string{"performance"}},
		{"classes alone does not cascade", []string{"classes"}, []string{"classes"}},
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

	res := syncPerformanceDependent(context.Background(), perf, db, 4, nil, io.Discard)
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

	res := syncPerformanceDependent(context.Background(), &pathAwareSyncClient{byPath: map[string]json.RawMessage{}}, db, 4, nil, io.Discard)
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
	res := syncPerformanceDependent(context.Background(), dryRunClient, db, 4, nil, io.Discard)
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
	res := syncPerformanceDependent(context.Background(), failingClient, db, 4, nil, io.Discard)
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
	res := syncPerformanceDependent(context.Background(), capturing, db, 1, userParams, io.Discard)
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
