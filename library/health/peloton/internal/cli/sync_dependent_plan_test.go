// Copyright 2026 Felix Banuchi and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
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

	plan, err := planDependentSync(db, "workouts", "performance", false, nil, 0, false)
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

	plan, err := planDependentSync(db, "workouts", "performance", false, nil, 0, false)
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

	plan, err := planDependentSync(db, "workouts", "performance", true, nil, 0, false)
	if err != nil {
		t.Fatalf("planDependentSync: %v", err)
	}
	if len(plan.ids) != 3 {
		t.Fatalf("ids = %v, want all 3 workouts under --full regardless of existing records", plan.ids)
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
	plan1, err := planDependentSync(db, "workouts", "performance", true, nil, 2, false)
	if err != nil {
		t.Fatalf("planDependentSync call 1: %v", err)
	}
	if len(plan1.ids) != 2 || !plan1.capped {
		t.Fatalf("call 1 ids=%v capped=%v, want 2 ids capped=true", plan1.ids, plan1.capped)
	}
	if plan1.totalPending != 5 {
		t.Fatalf("call 1 totalPending=%d, want 5 (full backlog before capping)", plan1.totalPending)
	}

	// Second call: should continue from offset 2, not restart from the top.
	plan2, err := planDependentSync(db, "workouts", "performance", true, nil, 2, false)
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

	// Third call: only 1 remains (5 - 2 - 2), not capped.
	plan3, err := planDependentSync(db, "workouts", "performance", true, nil, 2, false)
	if err != nil {
		t.Fatalf("planDependentSync call 3: %v", err)
	}
	if len(plan3.ids) != 1 || plan3.capped {
		t.Fatalf("call 3 ids=%v capped=%v, want exactly 1 id, not capped", plan3.ids, plan3.capped)
	}

	// Fourth call: the pass is complete (offset >= len(parentIDs)); a
	// fresh --full call must wrap around and start over, not return empty
	// forever.
	plan4, err := planDependentSync(db, "workouts", "performance", true, nil, 2, false)
	if err != nil {
		t.Fatalf("planDependentSync call 4: %v", err)
	}
	if len(plan4.ids) != 2 {
		t.Fatalf("call 4 (fresh pass after completion) ids=%v, want 2 (wrapped back to the start)", plan4.ids)
	}
}

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

	plan1, err := planDependentSync(db, "workouts", "performance", true, nil, 1, true)
	if err != nil {
		t.Fatalf("planDependentSync (dry-run) call 1: %v", err)
	}
	plan2, err := planDependentSync(db, "workouts", "performance", true, nil, 1, true)
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

	plan, err := planDependentSync(db, "workouts", "performance", false, nil, 2, false)
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

	plan, err := planDependentSync(db, "workouts", "performance", false, &cutoff, 0, false)
	if err != nil {
		t.Fatalf("planDependentSync: %v", err)
	}
	if len(plan.ids) != 1 || plan.ids[0] != "w-new" {
		t.Fatalf("ids = %v, want exactly [w-new] (w-old was touched before scopeSince)", plan.ids)
	}
}
