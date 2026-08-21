// Copyright 2026 Felix Banuchi and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/peloton/internal/store"
)

func seedPlanTestWorkouts(t *testing.T, db *store.Store, ids ...string) {
	t.Helper()
	items := make([]json.RawMessage, 0, len(ids))
	for _, id := range ids {
		items = append(items, json.RawMessage(`{"id":"`+id+`","ride_id":"ride-a","start_time":"2026-01-01T10:00:00Z"}`))
	}
	if res := syncResource(context.Background(), &fixtureSyncClient{items: items}, db, "workouts", "", false, 0, false, false, nil, io.Discard); res.Err != nil {
		t.Fatalf("seeding workouts fixture: %v", res.Err)
	}
}

// setProviderFactFetchedAt directly rewrites a stored fact's fetched_at, for
// tests that need deterministic control over "when was this touched"
// (ParentIDsTouchedSince/dependentScopeSince) without depending on real
// wall-clock gaps between writes.
func setProviderFactFetchedAt(t *testing.T, db *store.Store, family, id string, at time.Time) {
	t.Helper()
	if _, err := db.DB().Exec(`UPDATE provider_payloads SET fetched_at=? WHERE family=? AND provider_id=?`, at, family, id); err != nil {
		t.Fatalf("setProviderFactFetchedAt(%s,%s): %v", family, id, err)
	}
}

// TestPlanDependentSync_ParentTableEmptyReported guards the "no parent data
// at all" case: planDependentSync must surface parentTableEmpty distinctly
// from "fully caught up" so callers report the right message ("sync
// workouts first" vs. a quiet success).
func TestPlanDependentSync_ParentTableEmptyReported(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	plan, err := planDependentSync(db, "workouts", "performance", false, nil, nil, 0, false)
	if err != nil {
		t.Fatalf("planDependentSync: %v", err)
	}
	if !plan.parentTableEmpty {
		t.Fatal("expected parentTableEmpty=true when no workouts are synced")
	}
	if len(plan.ids) != 0 {
		t.Fatalf("ids = %v, want empty", plan.ids)
	}
}

// TestPlanDependentSync_DefaultSkipsAlreadySyncedParents guards the
// resumability rule central to NEW ISSUE C: by default, a parent that
// already has a dependentResource record is skipped, so a dependent sync
// naturally resumes across repeated calls without any separate cursor --
// an id with existing data doesn't need reprocessing.
func TestPlanDependentSync_DefaultSkipsAlreadySyncedParents(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedPlanTestWorkouts(t, db, "w1", "w2", "w3")
	if err := db.UpsertWithFacts("performance", "w1", json.RawMessage(`{"metrics":[]}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertWithFacts("performance", "w2", json.RawMessage(`{"metrics":[]}`)); err != nil {
		t.Fatal(err)
	}

	plan, err := planDependentSync(db, "workouts", "performance", false, nil, nil, 0, false)
	if err != nil {
		t.Fatalf("planDependentSync: %v", err)
	}
	if plan.parentTableEmpty {
		t.Fatal("parentTableEmpty=true, want false (workouts are synced)")
	}
	if len(plan.ids) != 1 || plan.ids[0] != "w3" {
		t.Fatalf("ids = %v, want exactly [w3] (w1/w2 already have performance records)", plan.ids)
	}
}

// TestPlanDependentSync_FullIgnoresAlreadySyncedParents guards --full's
// "ignore previous checkpoint, redo everything" contract applying to
// dependents too: skip-if-present must NOT apply under full=true, since
// the whole point of --full here is backfilling a fix (e.g. BLOCKING #1's
// every_n=1 correction) across already-synced, already-present records.
func TestPlanDependentSync_FullIgnoresAlreadySyncedParents(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedPlanTestWorkouts(t, db, "w1", "w2", "w3")
	if err := db.UpsertWithFacts("performance", "w1", json.RawMessage(`{"metrics":[]}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertWithFacts("performance", "w2", json.RawMessage(`{"metrics":[]}`)); err != nil {
		t.Fatal(err)
	}

	plan, err := planDependentSync(db, "workouts", "performance", true, nil, nil, 0, false)
	if err != nil {
		t.Fatalf("planDependentSync: %v", err)
	}
	if len(plan.ids) != 3 {
		t.Fatalf("ids = %v, want all 3 workouts under --full regardless of existing records", plan.ids)
	}
}

// commitFullOffset simulates runDependentFanOut's post-success persistence
// step: planDependentSync only computes where the --full offset WOULD land
// (dependentSyncPlan.fullOffsetKey/fullOffsetTarget) -- it no longer saves
// it itself, since doing so before the corresponding fetch work ran was the
// bug a live PR review caught (a partial-batch failure left the offset
// advanced past ids that were never actually fetched). Tests that call
// planDependentSync directly, without going through the real fetch loop,
// must call this between calls to model "the batch succeeded." Advances by
// the full batch length, i.e. simulates every id in plan.ids succeeding --
// runDependentFanOut itself advances by however many LEADING ids actually
// succeeded (see dependentSyncPlan.fullOffsetBase); this helper's "advance
// by everything" only matches that when nothing failed.
func commitFullOffset(t *testing.T, db *store.Store, plan dependentSyncPlan) {
	t.Helper()
	if plan.fullOffsetKey == "" {
		return
	}
	if err := db.SaveSyncState(plan.fullOffsetKey, "", plan.fullOffsetBase+plan.fullSweepCount); err != nil {
		t.Fatalf("commitFullOffset: %v", err)
	}
}

// TestPlanDependentSync_FullResumesAcrossCallsAndWrapsAfterACompletePass
// guards NEW ISSUE C's core ask: a --full backfill of a large backlog must
// drain across repeated bounded calls (via a persisted offset) rather than
// restarting from the top every time, and once a full pass completes, the
// NEXT --full call must start a fresh pass (matching --full's meaning as
// "redo," including on repeat use) instead of becoming a permanent no-op.
func TestPlanDependentSync_FullResumesAcrossCallsAndWrapsAfterACompletePass(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedPlanTestWorkouts(t, db, "w1", "w2", "w3", "w4", "w5")

	// First call: capped at 2, should get the first 2 ids (sorted order)
	// and persist an offset of 2.
	plan1, err := planDependentSync(db, "workouts", "performance", true, nil, nil, 2, false)
	if err != nil {
		t.Fatalf("planDependentSync call 1: %v", err)
	}
	if len(plan1.ids) != 2 || !plan1.capped {
		t.Fatalf("call 1 ids=%v capped=%v, want 2 ids capped=true", plan1.ids, plan1.capped)
	}
	if plan1.totalPending != 5 {
		t.Fatalf("call 1 totalPending=%d, want 5 (full backlog before capping)", plan1.totalPending)
	}
	commitFullOffset(t, db, plan1)

	// Second call: should continue from offset 2, not restart from the top.
	plan2, err := planDependentSync(db, "workouts", "performance", true, nil, nil, 2, false)
	if err != nil {
		t.Fatalf("planDependentSync call 2: %v", err)
	}
	if len(plan2.ids) != 2 {
		t.Fatalf("call 2 ids=%v, want 2 more (resuming from offset 2)", plan2.ids)
	}
	for _, id := range plan2.ids {
		for _, prior := range plan1.ids {
			if id == prior {
				t.Fatalf("call 2 reprocessed %q, which call 1 already claimed -- offset did not advance", id)
			}
		}
	}
	commitFullOffset(t, db, plan2)

	// Third call: only 1 remains (5 - 2 - 2), not capped.
	plan3, err := planDependentSync(db, "workouts", "performance", true, nil, nil, 2, false)
	if err != nil {
		t.Fatalf("planDependentSync call 3: %v", err)
	}
	if len(plan3.ids) != 1 || plan3.capped {
		t.Fatalf("call 3 ids=%v capped=%v, want exactly 1 id, not capped", plan3.ids, plan3.capped)
	}
	commitFullOffset(t, db, plan3)

	// Fourth call: the pass is complete (offset >= len(parentIDs)); a
	// fresh --full call must wrap around and start over, not return empty
	// forever.
	plan4, err := planDependentSync(db, "workouts", "performance", true, nil, nil, 2, false)
	if err != nil {
		t.Fatalf("planDependentSync call 4: %v", err)
	}
	if len(plan4.ids) != 2 {
		t.Fatalf("call 4 (fresh pass after completion) ids=%v, want 2 (wrapped back to the start)", plan4.ids)
	}
}

// TestFullSyncOffsetDoesNotAdvancePastAPartialBatchFailure is the true
// end-to-end regression for the bug a live PR review caught: the previous
// implementation persisted the --full resume offset inside planDependentSync,
// before a single request in the batch had actually been made. A failure
// partway through the batch still left the offset advanced past every
// planned id, so the failed id was skipped on every subsequent --full call
// until the entire pass wrapped back to 0 -- on a large backlog, that could
// be a very long time. Drives the real runDependentFanOut path (not just
// planDependentSync in isolation, which can't observe fetch failures) with
// one workout id that fails to fetch, and asserts the next plan still
// includes it rather than skipping past it.
func TestFullSyncOffsetDoesNotAdvancePastAPartialBatchFailure(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedPlanTestWorkouts(t, db, "w1", "w2")

	// w1 fails (pathAwareSyncClient errors on any path not in its map);
	// w2 succeeds.
	client := &pathAwareSyncClient{byPath: map[string]json.RawMessage{
		"/api/workout/w2/performance_graph": json.RawMessage(`{"seconds_since_pedaling_start":[0],"metrics":[]}`),
	}}

	plan, err := planDependentSync(db, "workouts", "performance", true, nil, nil, 2, false)
	if err != nil {
		t.Fatalf("planDependentSync: %v", err)
	}
	if len(plan.ids) != 2 {
		t.Fatalf("plan.ids = %v, want both w1 and w2", plan.ids)
	}

	res := syncPerformanceDependent(context.Background(), client, db, plan, 1, nil, io.Discard)
	if res.Err != nil {
		t.Fatalf("syncPerformanceDependent: %v (a per-item failure must stay a soft anomaly)", res.Err)
	}
	if res.Count != 1 {
		t.Fatalf("Count = %d, want 1 (only w2 succeeded)", res.Count)
	}

	// The bug: the next --full call must still see w1 as pending, not
	// skip past it because the offset advanced regardless of the failure.
	replan, err := planDependentSync(db, "workouts", "performance", true, nil, nil, 2, false)
	if err != nil {
		t.Fatalf("planDependentSync (replan): %v", err)
	}
	found := false
	for _, id := range replan.ids {
		if id == "w1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("replan.ids = %v, missing w1 -- the failed id was skipped because the offset advanced despite the failure", replan.ids)
	}
}

// TestFullSyncCheckpointSaveErrorFailsTheCommand guards the leftover
// Greptile finding on the atomic SaveSyncStates path: fan-out can
// succeed (or report a soft per-item anomaly) while the continuation
// transaction fails. The command must return that error instead of
// sync_complete / a successful count, and the cursor/backlog/turn must
// stay at their pre-call values so the next invocation retries the same
// window rather than pretending progress persisted.
func TestFullSyncCheckpointSaveErrorFailsTheCommand(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedPlanTestWorkouts(t, db, "w1", "w2")

	// Fail only the --full continuation rows. Workout seeding and
	// dependent upserts use other tables / resource_type keys, so
	// fan-out still completes; this is the existing db.DB() Exec
	// seam, not a new fake store.
	const failCheckpoint = `
CREATE TRIGGER fail_full_sync_checkpoint_insert
BEFORE INSERT ON sync_state
WHEN NEW.resource_type IN ('performance:full_progress','performance:full_failed','performance:full_turn')
BEGIN
	SELECT RAISE(ABORT, 'injected checkpoint failure');
END;
CREATE TRIGGER fail_full_sync_checkpoint_update
BEFORE UPDATE ON sync_state
WHEN NEW.resource_type IN ('performance:full_progress','performance:full_failed','performance:full_turn')
BEGIN
	SELECT RAISE(ABORT, 'injected checkpoint failure');
END;`
	if _, err := db.DB().Exec(failCheckpoint); err != nil {
		t.Fatalf("install checkpoint-failure triggers: %v", err)
	}

	client := &pathAwareSyncClient{byPath: map[string]json.RawMessage{
		"/api/workout/w1/performance_graph": json.RawMessage(`{"seconds_since_pedaling_start":[0],"metrics":[]}`),
		"/api/workout/w2/performance_graph": json.RawMessage(`{"seconds_since_pedaling_start":[0],"metrics":[]}`),
	}}
	plan, err := planDependentSync(db, "workouts", "performance", true, nil, nil, 2, false)
	if err != nil {
		t.Fatalf("planDependentSync: %v", err)
	}
	if len(plan.ids) != 2 {
		t.Fatalf("plan.ids = %v, want both w1 and w2", plan.ids)
	}

	res := syncPerformanceDependent(context.Background(), client, db, plan, 1, nil, io.Discard)
	if res.Err == nil {
		t.Fatal("syncPerformanceDependent Err = nil, want the SaveSyncStates failure surfaced")
	}
	if !strings.Contains(res.Err.Error(), "continuation checkpoint") {
		t.Fatalf("Err = %v, want it to name the continuation checkpoint", res.Err)
	}

	_, _, progress, err := db.GetSyncState("performance:full_progress")
	if err != nil {
		t.Fatalf("GetSyncState(full_progress): %v", err)
	}
	if progress != 0 {
		t.Fatalf("full_progress count = %d, want 0 (checkpoint must stay stale)", progress)
	}
	failedCursor, _, failedCount, err := db.GetSyncState("performance:full_failed")
	if err != nil {
		t.Fatalf("GetSyncState(full_failed): %v", err)
	}
	if failedCursor != "" || failedCount != 0 {
		t.Fatalf("full_failed = (%q, %d), want empty stale backlog", failedCursor, failedCount)
	}

	replan, err := planDependentSync(db, "workouts", "performance", true, nil, nil, 2, false)
	if err != nil {
		t.Fatalf("planDependentSync (replan): %v", err)
	}
	if len(replan.ids) != 2 || replan.ids[0] != plan.ids[0] || replan.ids[1] != plan.ids[1] {
		t.Fatalf("replan.ids = %v, want the same window %v after a failed checkpoint", replan.ids, plan.ids)
	}
}

// TestFullSyncOffsetSweepNeverWedgesOnAChronicallyFailingFirstID guards a
// third round of the same live PR review: gating the sweep cursor's
// advancement on "did the batch succeed" (whole-batch, or even just its
// leading run) means an id that fails EVERY time it's attempted -- not just
// once -- can permanently pin the sweep at its own position, blocking every
// id behind it in the pass forever, specifically when that id sorts FIRST
// in its window (the leading-run variant of the fix still wedged on this).
// The sweep cursor must advance unconditionally by however many fresh ids
// it attempted, regardless of any of their outcomes -- a chronically
// failing id blocks only itself (via the separate backlog retry mechanism,
// see TestFullSyncOffsetFailedIDGetsBacklogPriorityAcrossCalls), never the
// rest of the pass. Uses 5 ids with a --max-parents 2 cap so the sweep
// can't trivially wrap on the very first call, to actually exercise
// forward progress rather than accidentally passing via a same-call wrap.
func TestFullSyncOffsetSweepNeverWedgesOnAChronicallyFailingFirstID(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedPlanTestWorkouts(t, db, "w1", "w2", "w3", "w4", "w5")

	// w1 (sorts first) always fails; every other id always succeeds.
	client := &pathAwareSyncClient{byPath: map[string]json.RawMessage{
		"/api/workout/w2/performance_graph": json.RawMessage(`{"seconds_since_pedaling_start":[0],"metrics":[]}`),
		"/api/workout/w3/performance_graph": json.RawMessage(`{"seconds_since_pedaling_start":[0],"metrics":[]}`),
		"/api/workout/w4/performance_graph": json.RawMessage(`{"seconds_since_pedaling_start":[0],"metrics":[]}`),
		"/api/workout/w5/performance_graph": json.RawMessage(`{"seconds_since_pedaling_start":[0],"metrics":[]}`),
	}}

	// Call 1: sweep window [w1, w2] (offset 0, cap 2). w1 fails.
	plan1, err := planDependentSync(db, "workouts", "performance", true, nil, nil, 2, false)
	if err != nil {
		t.Fatalf("planDependentSync call 1: %v", err)
	}
	if got := plan1.ids; len(got) != 2 || got[0] != "w1" || got[1] != "w2" {
		t.Fatalf("call 1 plan.ids = %v, want [w1 w2]", got)
	}
	res1 := syncPerformanceDependent(context.Background(), client, db, plan1, 1, nil, io.Discard)
	if res1.Err != nil {
		t.Fatalf("call 1 syncPerformanceDependent: %v", res1.Err)
	}

	// The core assertion: call 2's sweep must have moved past w1/w2 to
	// [w3, w4] despite w1's failure -- not be stuck re-offering [w1, w2].
	// w1 is still expected to appear too, but via the backlog (priority),
	// not because the sweep failed to advance.
	plan2, err := planDependentSync(db, "workouts", "performance", true, nil, nil, 2, false)
	if err != nil {
		t.Fatalf("planDependentSync call 2: %v", err)
	}
	sweptForward := false
	for _, id := range plan2.ids {
		if id == "w3" || id == "w4" {
			sweptForward = true
		}
	}
	if !sweptForward {
		t.Fatalf("call 2 plan.ids = %v, want it to include w3 and/or w4 -- the sweep is wedged on w1's chronic failure", plan2.ids)
	}

	res2 := syncPerformanceDependent(context.Background(), client, db, plan2, 1, nil, io.Discard)
	if res2.Err != nil {
		t.Fatalf("call 2 syncPerformanceDependent: %v", res2.Err)
	}

	// Calls 3 and 4: confirm the sweep keeps moving rather than wedging on
	// any later call either. Each call spends one of its two --max-parents
	// slots retrying the chronically-failing w1 from the backlog, so the
	// sweep only advances by one fresh id per call here -- w5 (the last
	// id) is reached by call 4, not necessarily call 3.
	plan3, err := planDependentSync(db, "workouts", "performance", true, nil, nil, 2, false)
	if err != nil {
		t.Fatalf("planDependentSync call 3: %v", err)
	}
	res3 := syncPerformanceDependent(context.Background(), client, db, plan3, 1, nil, io.Discard)
	if res3.Err != nil {
		t.Fatalf("call 3 syncPerformanceDependent: %v", res3.Err)
	}

	plan4, err := planDependentSync(db, "workouts", "performance", true, nil, nil, 2, false)
	if err != nil {
		t.Fatalf("planDependentSync call 4: %v", err)
	}
	reachedW5 := false
	for _, id := range plan4.ids {
		if id == "w5" {
			reachedW5 = true
		}
	}
	if !reachedW5 {
		t.Fatalf("call 4 plan.ids = %v, want it to include w5 -- the sweep stopped making progress", plan4.ids)
	}
}

// TestFullSyncOffsetFailedIDGetsBacklogPriorityAcrossCalls guards the other
// half of the two-tier design: the sweep cursor advancing unconditionally
// (previous test) only avoids wedging the pass if a failed id is still
// remembered and retried somewhere -- otherwise it would just be silently
// dropped once the sweep passes it. A failed id must reappear, with
// priority, on every subsequent call until it succeeds, independent of
// where the sweep cursor has moved on to.
func TestFullSyncOffsetFailedIDGetsBacklogPriorityAcrossCalls(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedPlanTestWorkouts(t, db, "w1", "w2", "w3")

	// w2 (the middle id in sort order) always fails; w1 and w3 succeed.
	client := &pathAwareSyncClient{byPath: map[string]json.RawMessage{
		"/api/workout/w1/performance_graph": json.RawMessage(`{"seconds_since_pedaling_start":[0],"metrics":[]}`),
		"/api/workout/w3/performance_graph": json.RawMessage(`{"seconds_since_pedaling_start":[0],"metrics":[]}`),
	}}

	plan, err := planDependentSync(db, "workouts", "performance", true, nil, nil, 3, false)
	if err != nil {
		t.Fatalf("planDependentSync: %v", err)
	}
	res := syncPerformanceDependent(context.Background(), client, db, plan, 1, nil, io.Discard)
	if res.Err != nil {
		t.Fatalf("syncPerformanceDependent: %v", res.Err)
	}
	if res.Count != 2 {
		t.Fatalf("Count = %d, want 2 (w1 and w3 succeeded)", res.Count)
	}

	// w2 must appear next call (the backlog), and specifically FIRST
	// (priority retry), regardless of the sweep cursor having wrapped
	// past its original position.
	replan, err := planDependentSync(db, "workouts", "performance", true, nil, nil, 3, false)
	if err != nil {
		t.Fatalf("planDependentSync (replan): %v", err)
	}
	if len(replan.ids) == 0 || replan.ids[0] != "w2" {
		t.Fatalf("replan.ids = %v, want w2 first (backlog priority retry)", replan.ids)
	}
}

// TestFullSyncOffsetLargeBacklogDoesNotStarveSweep guards a live PR review
// finding on the two-tier design itself: with backlog-first candidate
// ordering, a backlog at or above --max-parents in size would otherwise
// consume the ENTIRE cap on every single call, permanently leaving the
// sweep with zero budget (fullSweepCount stuck at 0 forever) -- the exact
// same "something blocks the sweep forever" class of bug the whole
// two-tier design exists to prevent, just moved from "one persistently
// failing id" to "a backlog at least as large as the cap." The sweep must
// be guaranteed a minimum share of the cap (at least half, rounding up)
// regardless of how large the backlog grows.
func TestFullSyncOffsetLargeBacklogDoesNotStarveSweep(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedPlanTestWorkouts(t, db, "w1", "w2", "w3", "w4", "w5", "w6")

	// Seed a backlog of 3 ids -- already larger than the --max-parents 2
	// cap used below -- and a sweep cursor partway through the id space
	// (offset 3, leaving w4/w5/w6 still to sweep). This models the state
	// after several prior calls, without needing to actually drive that
	// many real fetch/store round trips first.
	if err := db.SaveSyncState("performance:full_failed", "w1,w2,w3", 3); err != nil {
		t.Fatalf("seed backlog: %v", err)
	}
	if err := db.SaveSyncState("performance:full_progress", "", 3); err != nil {
		t.Fatalf("seed offset: %v", err)
	}

	plan, err := planDependentSync(db, "workouts", "performance", true, nil, nil, 2, false)
	if err != nil {
		t.Fatalf("planDependentSync: %v", err)
	}
	if len(plan.ids) != 2 {
		t.Fatalf("plan.ids = %v, want exactly 2 (the --max-parents cap)", plan.ids)
	}

	sweptIncluded := false
	for _, id := range plan.ids {
		if id == "w4" || id == "w5" || id == "w6" {
			sweptIncluded = true
		}
	}
	if !sweptIncluded {
		t.Fatalf("plan.ids = %v, want at least one fresh sweep id (w4/w5/w6) included alongside the backlog -- a backlog >= --max-parents must not consume the entire cap", plan.ids)
	}
	if plan.fullSweepCount == 0 {
		t.Fatalf("fullSweepCount = 0, want > 0 -- the sweep must always get some budget even when the backlog alone exceeds --max-parents")
	}
}

// TestFullSyncOffsetMaxParentsOneNeverEmptiesPlanWithPendingWork guards an
// independent blind code review's most severe finding: at --max-parents 1,
// once the backlog grows to cover the entire current sweep window (which
// happens naturally once every id a small account has has failed and
// wrapped into the backlog at least once), the sweep budget's ceiling
// split gives the sweep the cap's only slot while the sweep itself has no
// fresh id left to offer (everything in its window is already a backlog
// dup) -- producing a completely empty plan.ids despite real pending
// work. runDependentFanOut's len(plan.ids)==0 short-circuit then reports
// "already_up_to_date", silently masking a permanent deadlock. Drives 12
// consecutive real --full calls (well past the point where this triggers)
// and asserts plan.ids is never empty while totalPending is nonzero.
func TestFullSyncOffsetMaxParentsOneNeverEmptiesPlanWithPendingWork(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedPlanTestWorkouts(t, db, "w1", "w2")

	// Both ids fail on every attempt -- pathAwareSyncClient errors on any
	// path not in its (empty) map.
	client := &pathAwareSyncClient{byPath: map[string]json.RawMessage{}}

	for call := 1; call <= 12; call++ {
		plan, err := planDependentSync(db, "workouts", "performance", true, nil, nil, 1, false)
		if err != nil {
			t.Fatalf("call %d planDependentSync: %v", call, err)
		}
		if plan.totalPending > 0 && len(plan.ids) == 0 {
			t.Fatalf("call %d: plan.ids is empty but totalPending=%d -- this is the silent deadlock a blind review found (runDependentFanOut would report \"already_up_to_date\" while 2 ids are permanently stuck)", call, plan.totalPending)
		}
		res := syncPerformanceDependent(context.Background(), client, db, plan, 1, nil, io.Discard)
		if res.Err != nil {
			t.Fatalf("call %d syncPerformanceDependent: %v", call, res.Err)
		}
	}
}

// TestFullSyncOffsetMaxParentsOneAlternatesBothIDsGetRetried guards the
// same review finding's companion bug: without alternation, the ceiling
// split's "sweep always wins the only slot at cap 1" rule would let one
// id monopolize every call forever while the other -- once it lands in
// the backlog -- never gets attempted again. Over enough consecutive
// --full --max-parents 1 calls, both ids must actually get fetched
// (observed via the fake client's call count per path), not just one.
func TestFullSyncOffsetMaxParentsOneAlternatesBothIDsGetRetried(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedPlanTestWorkouts(t, db, "w1", "w2")

	attempts := map[string]int{}
	var mu sync.Mutex
	client := &countingFailAlwaysClient{onAttempt: func(path string) {
		mu.Lock()
		attempts[path]++
		mu.Unlock()
	}}

	for call := 1; call <= 12; call++ {
		plan, err := planDependentSync(db, "workouts", "performance", true, nil, nil, 1, false)
		if err != nil {
			t.Fatalf("call %d planDependentSync: %v", call, err)
		}
		res := syncPerformanceDependent(context.Background(), client, db, plan, 1, nil, io.Discard)
		if res.Err != nil {
			t.Fatalf("call %d syncPerformanceDependent: %v", call, res.Err)
		}
	}

	if attempts["/api/workout/w1/performance_graph"] == 0 || attempts["/api/workout/w2/performance_graph"] == 0 {
		t.Fatalf("attempts = %v, want both w1 and w2 attempted at least once across 12 calls -- one id was permanently starved at --max-parents 1", attempts)
	}
}

// TestFullSyncOffsetWrapWithOverlappingBacklogAdvancesPastWholeWindow
// guards a bug found independently by two different reviewers: a live PR
// review and a separate code-review pass both flagged that fullSweepCount
// was computed by counting how many ids were actually SELECTED as sweep
// work, not how many raw positions of the sweep window were consumed.
// Right after a --full pass wraps (offset resets to 0), the entire id
// space can simultaneously be "swept" and "in the backlog" if every id
// failed on the prior pass -- when that happens, the deduped-count
// formula would report 0 progress even though every position was
// examined, causing the cursor to stall at 0 and repeatedly re-examine
// the same window on every subsequent call instead of ever moving on.
// This test drives a wrap where the ENTIRE window overlaps the backlog
// and asserts the cursor still advances (does not stall at its
// pre-call value).
func TestFullSyncOffsetWrapWithOverlappingBacklogAdvancesPastWholeWindow(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedPlanTestWorkouts(t, db, "w1", "w2")

	// Seed a wrapped-pass state directly: offset at the end of the id
	// space (so the next call wraps to 0) and a backlog that already
	// contains BOTH ids -- i.e. the entire post-wrap sweep window is
	// backlog-shadowed, the exact condition that trips the position-vs-
	// count mismatch.
	if err := db.SaveSyncState("performance:full_progress", "", 2); err != nil {
		t.Fatalf("seed offset: %v", err)
	}
	if err := db.SaveSyncState("performance:full_failed", "w1,w2", 2); err != nil {
		t.Fatalf("seed backlog: %v", err)
	}

	// Use a cap large enough that both backlog ids fit in one call (no
	// --max-parents starvation logic involved here -- isolating the
	// position-counting bug specifically).
	client := &pathAwareSyncClient{byPath: map[string]json.RawMessage{
		"/api/workout/w1/performance_graph": json.RawMessage(`{"seconds_since_pedaling_start":[0],"metrics":[]}`),
		"/api/workout/w2/performance_graph": json.RawMessage(`{"seconds_since_pedaling_start":[0],"metrics":[]}`),
	}}
	plan, err := planDependentSync(db, "workouts", "performance", true, nil, nil, 10, false)
	if err != nil {
		t.Fatalf("planDependentSync: %v", err)
	}
	res := syncPerformanceDependent(context.Background(), client, db, plan, 1, nil, io.Discard)
	if res.Err != nil {
		t.Fatalf("syncPerformanceDependent: %v", res.Err)
	}

	cursor, _, count, err := db.GetSyncState("performance:full_progress")
	if err != nil {
		t.Fatalf("GetSyncState(full_progress): %v", err)
	}
	_ = cursor
	if count == 0 {
		t.Fatalf("full_progress count = 0, want > 0 -- the cursor must advance past a fully backlog-shadowed sweep window (both w1 and w2 succeeded this call), not stall at its pre-call value")
	}
}

// alwaysSucceedFullClient returns a valid response for any path -- for
// tests that only care about the sweep/turn bookkeeping across many
// consecutive successful calls, not per-id response shaping.
type alwaysSucceedFullClient struct{}

func (alwaysSucceedFullClient) Get(_ context.Context, _ string, _ map[string]string) (json.RawMessage, error) {
	return json.RawMessage(`{"id":"x","ride":{"id":"ride-a"}}`), nil
}
func (alwaysSucceedFullClient) RateLimit() float64 { return 0 }

// TestFullSyncTurnBitFlipsEveryCallAcrossMixedCapSizes is a live-account
// verification round's own reproduction: round-14 testing against a real
// account (workout_details, ~3,596 workouts, 7 consecutive --full calls at
// --max-parents 40 then 2) reported the sweep offset advancing correctly
// every call but the persisted turn bit (":full_turn") appearing to flip
// once and then stay flat for several calls, which looked inconsistent
// with "flips unconditionally every call" (see runDependentFanOut's step-3
// comment). This test drives the exact same call sequence (7 calls, a
// 500-id parent set, --max-parents 40 x5 then 2 x2, nothing ever failing)
// through the real fetch/store path and asserts the turn bit alternates
// 1,0,1,0,1,0,1 -- confirming the code itself behaves as designed. The
// live-account discrepancy was not reproduced here, so it's most likely an
// artifact of how that round's raw-SQL observation tool read the database
// (see the round-14 report reply) rather than a bug in this bookkeeping;
// this test exists so any future actual regression here is caught
// immediately, and as a citable reference for why the turn bit alone isn't
// the smoking gun.
func TestFullSyncTurnBitFlipsEveryCallAcrossMixedCapSizes(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	items := make([]json.RawMessage, 0, 500)
	for i := 0; i < 500; i++ {
		id := fmt.Sprintf("w%04d", i)
		items = append(items, json.RawMessage(`{"id":"`+id+`","ride_id":"ride-a","start_time":"2026-01-01T10:00:00Z"}`))
	}
	if _, _, err := db.UpsertBatchWithFacts("workouts", items); err != nil {
		t.Fatalf("seeding workouts: %v", err)
	}

	client := alwaysSucceedFullClient{}
	maxParentsSeq := []int{40, 40, 40, 40, 40, 2, 2}
	wantOffsets := []int{40, 80, 120, 160, 200, 202, 204}
	wantTurns := []int{1, 0, 1, 0, 1, 0, 1}
	for i, mp := range maxParentsSeq {
		plan, err := planDependentSync(db, "workouts", "workout_details", true, nil, nil, mp, false)
		if err != nil {
			t.Fatalf("call %d planDependentSync: %v", i+1, err)
		}
		res := syncWorkoutDetailsDependent(context.Background(), client, db, plan, 1, nil, io.Discard)
		if res.Err != nil {
			t.Fatalf("call %d syncWorkoutDetailsDependent: %v", i+1, res.Err)
		}
		_, _, offset, err := db.GetSyncState("workout_details:full_progress")
		if err != nil {
			t.Fatalf("call %d GetSyncState(full_progress): %v", i+1, err)
		}
		if offset != wantOffsets[i] {
			t.Fatalf("call %d offset = %d, want %d", i+1, offset, wantOffsets[i])
		}
		_, _, turn, err := db.GetSyncState("workout_details:full_turn")
		if err != nil {
			t.Fatalf("call %d GetSyncState(full_turn): %v", i+1, err)
		}
		if turn != wantTurns[i] {
			t.Fatalf("call %d turn = %d, want %d -- turn must flip unconditionally every full-sweep call, regardless of --max-parents", i+1, turn, wantTurns[i])
		}
	}
}

// countingFailAlwaysClient is a fixed-outcome fake sync client: every
// request fails, but onAttempt is invoked first so a test can observe
// which paths were actually tried (and how many times), which
// pathAwareSyncClient alone can't report.
type countingFailAlwaysClient struct {
	onAttempt func(path string)
}

func (c *countingFailAlwaysClient) Get(_ context.Context, path string, _ map[string]string) (json.RawMessage, error) {
	c.onAttempt(path)
	return nil, fmt.Errorf("simulated permanent failure for %s", path)
}

func (c *countingFailAlwaysClient) RateLimit() float64 { return 0 }

// TestPlanDependentSync_DryRunDoesNotPersistFullOffset guards --full's
// existing "a preview must not mutate sync-state" convention: planning
// under dryRun=true must not advance the persisted offset, so a --dry-run
// call doesn't silently consume part of a real backfill's progress.
func TestPlanDependentSync_DryRunDoesNotPersistFullOffset(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedPlanTestWorkouts(t, db, "w1", "w2", "w3")

	plan1, err := planDependentSync(db, "workouts", "performance", true, nil, nil, 1, true)
	if err != nil {
		t.Fatalf("planDependentSync (dry-run) call 1: %v", err)
	}
	plan2, err := planDependentSync(db, "workouts", "performance", true, nil, nil, 1, true)
	if err != nil {
		t.Fatalf("planDependentSync (dry-run) call 2: %v", err)
	}
	if plan1.ids[0] != plan2.ids[0] {
		t.Fatalf("dry-run planning advanced the persisted offset: call 1 = %v, call 2 = %v, want identical", plan1.ids, plan2.ids)
	}
}

// TestPlanDependentSync_MaxParentsCapsDefaultModeToo guards that
// --max-parents applies uniformly to the default (skip-existing) mode, not
// just --full mode.
func TestPlanDependentSync_MaxParentsCapsDefaultModeToo(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedPlanTestWorkouts(t, db, "w1", "w2", "w3", "w4", "w5")

	plan, err := planDependentSync(db, "workouts", "performance", false, nil, nil, 2, false)
	if err != nil {
		t.Fatalf("planDependentSync: %v", err)
	}
	if len(plan.ids) != 2 || !plan.capped || plan.totalPending != 5 {
		t.Fatalf("plan = %+v, want 2 capped ids out of 5 totalPending", plan)
	}
}

// TestDependentScopeSinceFor guards the RunE-level decision of whether to
// apply --latest-only's dependent run-scoping (NEW ISSUE B): it must fire
// only when --latest-only is actually in effect AND "workouts" was flat
// synced THIS invocation, preserving the documented "run a dependent
// without re-syncing its parent" contract (scanning the whole store) for
// every other combination.
func TestDependentScopeSinceFor(t *testing.T) {
	runStarted := time.Now().UTC()
	cases := []struct {
		name               string
		effectiveLatest    bool
		flatResources      []string
		wantScopingApplied bool
	}{
		{"latest-only with workouts flat-synced scopes", true, []string{"workouts", "classes"}, true},
		{"latest-only without workouts in this run's flat phase does not scope", true, []string{"classes"}, false},
		{"workouts flat-synced without latest-only does not scope", false, []string{"workouts"}, false},
		{"neither set does not scope", false, []string{"classes"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dependentScopeSinceFor(tc.effectiveLatest, tc.flatResources, runStarted)
			if tc.wantScopingApplied && got == nil {
				t.Fatal("expected scoping to apply (non-nil scopeSince), got nil")
			}
			if !tc.wantScopingApplied && got != nil {
				t.Fatalf("expected no scoping (nil scopeSince), got %v", *got)
			}
			if tc.wantScopingApplied && got != nil && !got.Equal(runStarted) {
				t.Fatalf("scopeSince = %v, want runStarted %v", *got, runStarted)
			}
		})
	}
}

// TestPlanDependentSync_ScopeSinceRestrictsToTouchedParents guards NEW
// ISSUE B: when scopeSince is set (the --latest-only run-scoping case),
// candidates must be restricted to only the parents touched at or after
// that time, not the whole local store -- --latest-only promises a bounded
// "refresh the top" operation.
func TestPlanDependentSync_ScopeSinceRestrictsToTouchedParents(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedPlanTestWorkouts(t, db, "w-old", "w-new")

	cutoff := time.Now().UTC()
	setProviderFactFetchedAt(t, db, "workouts", "w-old", cutoff.Add(-time.Hour))
	setProviderFactFetchedAt(t, db, "workouts", "w-new", cutoff.Add(time.Hour))

	plan, err := planDependentSync(db, "workouts", "performance", false, &cutoff, nil, 0, false)
	if err != nil {
		t.Fatalf("planDependentSync: %v", err)
	}
	if len(plan.ids) != 1 || plan.ids[0] != "w-new" {
		t.Fatalf("ids = %v, want exactly [w-new] (w-old was touched before scopeSince)", plan.ids)
	}
}

// TestParseStaleBefore guards --stale-before accepting both the absolute
// RFC3339 form (the natural way to say "before this fix was deployed") and
// the relative --since-style duration form (the natural way to say
// "anything older than a week").
func TestParseStaleBefore(t *testing.T) {
	got, err := parseStaleBefore("2026-08-14T09:00:00Z")
	if err != nil {
		t.Fatalf("parseStaleBefore(RFC3339): %v", err)
	}
	want := time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("parseStaleBefore(RFC3339) = %v, want %v", got, want)
	}

	before := time.Now().Add(-7 * 24 * time.Hour)
	got, err = parseStaleBefore("7d")
	if err != nil {
		t.Fatalf("parseStaleBefore(7d): %v", err)
	}
	after := time.Now().Add(-7 * 24 * time.Hour)
	if got.Before(before.Add(-time.Minute)) || got.After(after.Add(time.Minute)) {
		t.Fatalf("parseStaleBefore(7d) = %v, want ~7 days ago", got)
	}

	if _, err := parseStaleBefore("not-a-timestamp"); err == nil {
		t.Fatal("parseStaleBefore(garbage) should return an error")
	}
}

// TestPlanDependentSync_StaleBeforeRefetchesOldRecordsWithoutFull guards
// NEW ISSUE E from a fourth live post-fix verification sweep: the default
// (non-full) mode only checks whether a dependentResource record exists at
// all, which can't detect a record that exists but is stale in shape (e.g.
// a performance record synced before the every_n=1 fix landed). Without
// --stale-before, backfilling such a fix requires --full, which redoes
// every record and wastes calls walking past already-correct ones.
// staleBefore must treat an existing record as pending when its fetched_at
// predates the cutoff, while a genuinely fresh record (fetched_at at or
// after the cutoff) must NOT be needlessly reprocessed -- staleBefore is a
// targeted backfill tool, not --full's blunt "redo everything."
func TestPlanDependentSync_StaleBeforeRefetchesOldRecordsWithoutFull(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedPlanTestWorkouts(t, db, "w-stale", "w-fresh", "w-missing")

	if err := db.UpsertWithFacts("performance", "w-stale", json.RawMessage(`{"metrics":[]}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertWithFacts("performance", "w-fresh", json.RawMessage(`{"metrics":[]}`)); err != nil {
		t.Fatal(err)
	}
	cutoff := time.Now().UTC()
	setProviderFactFetchedAt(t, db, "performance", "w-stale", cutoff.Add(-time.Hour))
	setProviderFactFetchedAt(t, db, "performance", "w-fresh", cutoff.Add(time.Hour))
	// w-missing has no performance record at all -- must stay pending
	// regardless of staleBefore, same as the no-staleBefore default case.

	plan, err := planDependentSync(db, "workouts", "performance", false, nil, &cutoff, 0, false)
	if err != nil {
		t.Fatalf("planDependentSync: %v", err)
	}
	got := map[string]bool{}
	for _, id := range plan.ids {
		got[id] = true
	}
	if !got["w-stale"] {
		t.Fatalf("ids = %v, want w-stale included (its record predates --stale-before)", plan.ids)
	}
	if !got["w-missing"] {
		t.Fatalf("ids = %v, want w-missing included (no record at all is always pending)", plan.ids)
	}
	if got["w-fresh"] {
		t.Fatalf("ids = %v, want w-fresh excluded (its record is fresher than --stale-before, not stale)", plan.ids)
	}
	if len(plan.ids) != 2 {
		t.Fatalf("ids = %v, want exactly 2 (w-stale, w-missing)", plan.ids)
	}
}

// TestPlanDependentSync_StaleBeforePrioritizesOldestFirstUnderCap guards a
// fifth live post-fix verification sweep's finding against NEW ISSUE E:
// --stale-before correctly identified genuinely-stale records (confirmed
// separately), but a --max-parents cap was exhausting itself on whichever
// pending ids happened to sort first lexically, not the most-overdue ones.
// On the real account, a batch of records backfilled moments before the
// chosen --stale-before cutoff by an unrelated earlier run happened to sort
// early enough that two capped calls (100 each) never got past them,
// leaving the true ~1600-workout stale backlog (scattered elsewhere in the
// id space, fetched hours earlier) completely untouched. Candidates must
// be ordered oldest-fetched-first so a capped call always makes progress
// on the most-overdue records regardless of where their ids fall. This
// fixture deliberately gives the OLDEST record the lexically-LAST id (and
// the newest-but-still-stale record the lexically-FIRST id) so an
// id-ordered bug and a fetched_at-ordered correct result produce visibly
// different results.
func TestPlanDependentSync_StaleBeforePrioritizesOldestFirstUnderCap(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// "w-a..." sorts before "w-z...": deliberately opposite of fetched_at
	// order, so an id-ordered bug and the correct oldest-first order
	// disagree on which id comes first.
	seedPlanTestWorkouts(t, db, "w-a-newer-stale", "w-z-oldest-stale", "w-m-fresh")
	for _, id := range []string{"w-a-newer-stale", "w-z-oldest-stale", "w-m-fresh"} {
		if err := db.UpsertWithFacts("performance", id, json.RawMessage(`{"metrics":[]}`)); err != nil {
			t.Fatal(err)
		}
	}
	cutoff := time.Now().UTC()
	setProviderFactFetchedAt(t, db, "performance", "w-z-oldest-stale", cutoff.Add(-2*time.Hour))
	setProviderFactFetchedAt(t, db, "performance", "w-a-newer-stale", cutoff.Add(-time.Minute))
	setProviderFactFetchedAt(t, db, "performance", "w-m-fresh", cutoff.Add(time.Hour))

	plan, err := planDependentSync(db, "workouts", "performance", false, nil, &cutoff, 1, false)
	if err != nil {
		t.Fatalf("planDependentSync: %v", err)
	}
	if len(plan.ids) != 1 || plan.ids[0] != "w-z-oldest-stale" {
		t.Fatalf("ids = %v, want exactly [w-z-oldest-stale] (the most-overdue record, despite its id sorting last)", plan.ids)
	}
	if plan.totalPending != 2 {
		t.Fatalf("totalPending = %d, want 2 (both stale records pending, fresh one excluded)", plan.totalPending)
	}
}
