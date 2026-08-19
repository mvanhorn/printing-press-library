// Copyright 2026 Felix Banuchi and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/health/peloton/internal/store"
)

// TestPlanClassDetailSync_ExcludesFreestyleSentinelRideID guards round-13
// verification NEW E: Peloton's all-zeros sentinel ride id
// (pelotonNoClassRideID) marks a freestyle/non-class workout (Just Run,
// Outdoor Running, Just Ride), not a real class -- there is no detail
// endpoint for it. Without exclusion it fetches, fails, gets logged as a
// per_parent_fetch_failed anomaly on every single call forever, and --
// since this dependent's pending check is content-based, not
// presence-based -- can never be satisfied, so the pending set never
// drains to zero even once every real class is caught up.
func TestPlanClassDetailSync_ExcludesFreestyleSentinelRideID(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	seedPlanTestWorkoutsWithRides(t, db, map[string]string{
		"w1": "ride-a",
		"w2": pelotonNoClassRideID,
		"w3": pelotonNoClassRideID,
	})

	plan, err := planClassDetailSync(db, false, nil, nil, 0)
	if err != nil {
		t.Fatalf("planClassDetailSync: %v", err)
	}
	if len(plan.ids) != 1 || plan.ids[0] != "ride-a" {
		t.Fatalf("plan.ids = %v, want exactly [ride-a] (the sentinel must never appear)", plan.ids)
	}
}

// TestPlanClassDetailSync_AllFreestyleWorkoutsReportsParentTableEmpty guards
// the edge case where every synced workout is freestyle: after excluding
// the sentinel, the candidate set is empty, which must report
// parentTableEmpty (a legitimate "nothing to do here" state) rather than
// an error or a false "fully caught up" success.
func TestPlanClassDetailSync_AllFreestyleWorkoutsReportsParentTableEmpty(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	seedPlanTestWorkoutsWithRides(t, db, map[string]string{"w1": pelotonNoClassRideID})

	plan, err := planClassDetailSync(db, false, nil, nil, 0)
	if err != nil {
		t.Fatalf("planClassDetailSync: %v", err)
	}
	if !plan.parentTableEmpty {
		t.Fatal("expected parentTableEmpty=true when every workout is the freestyle sentinel")
	}
}

// TestPlanClassDetailSync_ScopesToDistinctRideIDsNotFullCatalog guards the
// core of round-11 verification NEW 1's fix: classes_detail's fan-out must
// be over the distinct classes an account holder actually took
// (workouts.ride_id), not every id in the "classes" family -- fanning out
// over a full catalog (tens of thousands of ids) would defeat the entire
// point of scoping this dependent.
func TestPlanClassDetailSync_ScopesToDistinctRideIDsNotFullCatalog(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Two workouts share ride-a; a third references ride-b. The classes
	// catalog (bulk list sync) additionally carries ride-c, which no
	// workout ever took.
	seedPlanTestWorkoutsWithRides(t, db, map[string]string{"w1": "ride-a", "w2": "ride-a", "w3": "ride-b"})
	for _, rideID := range []string{"ride-a", "ride-b", "ride-c"} {
		if _, err := db.RecordProviderFact("classes", rideID, json.RawMessage(`{"id":"`+rideID+`","title":"Catalog item"}`)); err != nil {
			t.Fatalf("seed classes/%s: %v", rideID, err)
		}
	}

	plan, err := planClassDetailSync(db, false, nil, nil, 0)
	if err != nil {
		t.Fatalf("planClassDetailSync: %v", err)
	}
	if plan.parentTableEmpty {
		t.Fatal("expected parentTableEmpty=false: workouts are synced")
	}
	got := append([]string(nil), plan.ids...)
	want := []string{"ride-a", "ride-b"}
	if len(got) != len(want) {
		t.Fatalf("plan.ids = %v, want exactly the taken classes %v (ride-c was never taken and must not appear)", got, want)
	}
	seen := map[string]bool{}
	for _, id := range got {
		seen[id] = true
	}
	for _, id := range want {
		if !seen[id] {
			t.Fatalf("plan.ids = %v, missing taken class %q", got, id)
		}
	}
	if seen["ride-c"] {
		t.Fatalf("plan.ids = %v, includes ride-c which no workout ever took", got)
	}
}

// TestPlanClassDetailSync_PendingIsContentBasedNotPresenceBased guards the
// distinction from planDependentSync: a class already has a "classes"
// provider_payloads row from the flat catalog sync, so a plain presence
// check would see it as done forever. Only classDetailField (segments)
// presence should mark it done.
func TestPlanClassDetailSync_PendingIsContentBasedNotPresenceBased(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	seedPlanTestWorkoutsWithRides(t, db, map[string]string{"w1": "ride-a", "w2": "ride-b"})
	// ride-a: list-form only (has a provider_payloads row, but no segments).
	if _, err := db.RecordProviderFact("classes", "ride-a", json.RawMessage(`{"id":"ride-a","title":"List form"}`)); err != nil {
		t.Fatalf("seed ride-a: %v", err)
	}
	// ride-b: already detail-fetched (has segments).
	if _, err := db.RecordProviderFact("classes", "ride-b", json.RawMessage(`{"ride":{"id":"ride-b"},"segments":[{"role":"warmup"}]}`)); err != nil {
		t.Fatalf("seed ride-b: %v", err)
	}

	plan, err := planClassDetailSync(db, false, nil, nil, 0)
	if err != nil {
		t.Fatalf("planClassDetailSync: %v", err)
	}
	if len(plan.ids) != 1 || plan.ids[0] != "ride-a" {
		t.Fatalf("plan.ids = %v, want exactly [ride-a] (ride-b already has segments)", plan.ids)
	}
}

// TestPlanClassDetailSync_MaxParentsCaps guards --max-parents bounding the
// same way it does for performance/workout_details.
func TestPlanClassDetailSync_MaxParentsCaps(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	seedPlanTestWorkoutsWithRides(t, db, map[string]string{"w1": "ride-a", "w2": "ride-b", "w3": "ride-c"})

	plan, err := planClassDetailSync(db, false, nil, nil, 2)
	if err != nil {
		t.Fatalf("planClassDetailSync: %v", err)
	}
	if len(plan.ids) != 2 {
		t.Fatalf("plan.ids = %v, want 2 (capped)", plan.ids)
	}
	if !plan.capped {
		t.Fatal("expected capped=true")
	}
	if plan.totalPending != 3 {
		t.Fatalf("totalPending = %d, want 3", plan.totalPending)
	}
}

// TestSyncClassDetailDependent_EnrichesExistingClassRecordInPlace guards
// the write path: a detail fetch must land in the SAME "classes" family
// record the flat catalog sync already created (so offline_classes_structure/
// offline_intervals need no changes to start seeing segments), using the
// same nested ride.id write-through resolution a live `classes show` call
// uses, not a new family or a caller-supplied id that bypasses it.
func TestSyncClassDetailDependent_EnrichesExistingClassRecordInPlace(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	seedPlanTestWorkoutsWithRides(t, db, map[string]string{"w1": "ride-a"})
	if _, err := db.RecordProviderFact("classes", "ride-a", json.RawMessage(`{"id":"ride-a","title":"List form","duration":1800}`)); err != nil {
		t.Fatalf("seed ride-a: %v", err)
	}

	detailClient := &pathAwareSyncClient{byPath: map[string]json.RawMessage{
		"/api/ride/ride-a/details": json.RawMessage(`{"ride":{"id":"ride-a","title":"List form"},"segments":[{"role":"warmup","metric":"cadence"}],"target_metrics_data":{"cadence":[55,65]}}`),
	}}

	plan, err := planClassDetailSync(db, false, nil, nil, 0)
	if err != nil {
		t.Fatalf("planClassDetailSync: %v", err)
	}
	if len(plan.ids) != 1 || plan.ids[0] != "ride-a" {
		t.Fatalf("plan.ids = %v, want [ride-a]", plan.ids)
	}

	res := syncClassDetailDependent(context.Background(), detailClient, db, plan, 4, io.Discard)
	if res.Err != nil {
		t.Fatalf("syncClassDetailDependent: %v", res.Err)
	}
	if res.Count != 1 {
		t.Fatalf("Count = %d, want 1", res.Count)
	}

	fact, err := db.GetProviderFact("classes", "ride-a")
	if err != nil {
		t.Fatalf("GetProviderFact(classes, ride-a): %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(fact.Payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if _, ok := payload["segments"]; !ok {
		t.Fatalf("stored classes/ride-a fact has no segments after detail sync: %s", fact.Payload)
	}

	// The pending set must now be empty: the record just gained segments.
	replan, err := planClassDetailSync(db, false, nil, nil, 0)
	if err != nil {
		t.Fatalf("planClassDetailSync (replan): %v", err)
	}
	if len(replan.ids) != 0 {
		t.Fatalf("replan.ids = %v, want empty (ride-a should no longer be pending)", replan.ids)
	}
}

// TestSyncClassDetailDependent_AnomalyEventIncludesSampleError guards round-12
// verification NEW D: a per_parent_fetch_failed anomaly reported only counts,
// with no way to tell from the event alone whether a consistent failure rate
// is a specific class type, region lock, or transient blip. The event must
// now carry a failed count and a sample of the actual error message.
func TestSyncClassDetailDependent_AnomalyEventIncludesSampleError(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	seedPlanTestWorkoutsWithRides(t, db, map[string]string{"w1": "ride-a", "w2": "ride-b"})

	// Only ride-a has a fixture response; ride-b's fetch fails with
	// pathAwareSyncClient's own "no fixture response" error.
	detailClient := &pathAwareSyncClient{byPath: map[string]json.RawMessage{
		"/api/ride/ride-a/details": json.RawMessage(`{"ride":{"id":"ride-a"},"segments":[{"role":"warmup"}]}`),
	}}

	plan, err := planClassDetailSync(db, false, nil, nil, 0)
	if err != nil {
		t.Fatalf("planClassDetailSync: %v", err)
	}
	if len(plan.ids) != 2 {
		t.Fatalf("plan.ids = %v, want 2", plan.ids)
	}

	var events bytes.Buffer
	res := syncClassDetailDependent(context.Background(), detailClient, db, plan, 1, &events)
	if res.Err != nil {
		t.Fatalf("syncClassDetailDependent: %v (a per-item failure must stay a soft anomaly, not a hard error)", res.Err)
	}
	if res.Count != 1 {
		t.Fatalf("Count = %d, want 1 (only ride-a succeeded)", res.Count)
	}

	out := events.String()
	if !strings.Contains(out, `"reason":"per_parent_fetch_failed"`) {
		t.Fatalf("expected a per_parent_fetch_failed anomaly event, got: %s", out)
	}
	if !strings.Contains(out, `"failed":1`) {
		t.Fatalf("expected failed:1 in the anomaly event, got: %s", out)
	}
	if !strings.Contains(out, "no fixture response") {
		t.Fatalf("expected the anomaly event to sample the actual error message, got: %s", out)
	}
}

// seedPlanTestWorkoutsWithRides is seedPlanTestWorkouts with a caller-chosen
// ride_id per workout id, for tests that need more than one distinct ride
// across the seeded workouts.
func seedPlanTestWorkoutsWithRides(t *testing.T, db *store.Store, workoutIDToRideID map[string]string) {
	t.Helper()
	items := make([]json.RawMessage, 0, len(workoutIDToRideID))
	for workoutID, rideID := range workoutIDToRideID {
		items = append(items, json.RawMessage(`{"id":"`+workoutID+`","ride_id":"`+rideID+`","start_time":"2026-01-01T10:00:00Z"}`))
	}
	if res := syncResource(context.Background(), &fixtureSyncClient{items: items}, db, "workouts", "", false, 0, false, false, nil, io.Discard); res.Err != nil {
		t.Fatalf("seeding workouts fixture: %v", res.Err)
	}
}
