// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/cdc-pakistan/internal/store"
)

// These tests pin the definition of a "settled" (category, year) pair.
//
// The defect they guard against: settledPairs once selected any pair having a
// 'found' page, so a pair whose walk was cut short -- by a mid-page transport
// error, or by exhausting --max-scan-pages -- looked identical to a pair
// walked to its terminator. Every later run then SKIPPED it, reporting
// "nothing to probe", and the rest of that category/year was permanently
// missing from the corpus while --strict still exited 0. Downstream
// (identity ledger, eligibility state) has no completeness gate of its own, so
// a rename or suspension notice on an unfetched page reads as "does not exist"
// rather than "not fetched".

func newCoverageTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "cov.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.EnsureCDCSchema(context.Background(), db); err != nil {
		t.Fatalf("EnsureCDCSchema: %v", err)
	}
	return db
}

func bucket(t *testing.T, db *store.Store, cat string, year, paged int, state string) {
	t.Helper()
	if err := upsertBucket(context.Background(), db, cat, year, paged, state, 200, 1, "h", "2026-09-08T00:00:00Z"); err != nil {
		t.Fatalf("upsertBucket(%s,%d,%d,%s): %v", cat, year, paged, state, err)
	}
}

func TestSettledPairsRequiresATerminator(t *testing.T) {
	ctx := context.Background()
	db := newCoverageTestStore(t)

	// A: walked to the empty-page terminator -> settled.
	bucket(t, db, "complete", 2024, 1, "found")
	bucket(t, db, "complete", 2024, 2, "not-found-at-source")

	// B: page 1 succeeded, page 2 failed, pages 3+ never asked.
	// This is THE regression: it must NOT be settled.
	bucket(t, db, "poisoned", 2024, 1, "found")
	bucket(t, db, "poisoned", 2024, 2, "transport-error")

	// C: pages found but the walk never reached a terminator
	// (max-scan-pages exhaustion) -> must NOT be settled.
	bucket(t, db, "truncated", 2024, 1, "found")
	bucket(t, db, "truncated", 2024, 2, "found")

	// D: a single page that was genuinely empty at source -> settled.
	bucket(t, db, "emptyatsource", 2024, 1, "not-found-at-source")

	got, err := settledPairs(ctx, db)
	if err != nil {
		t.Fatalf("settledPairs: %v", err)
	}
	want := map[string]bool{
		pairKey("complete", 2024):      true,
		pairKey("emptyatsource", 2024): true,
	}
	for k := range want {
		if !got[k] {
			t.Errorf("pair %q should be settled but is not", k)
		}
	}
	for _, k := range []string{pairKey("poisoned", 2024), pairKey("truncated", 2024)} {
		if got[k] {
			t.Errorf("pair %q must NOT be settled: its walk never reached a terminator with everything succeeding", k)
		}
	}
	if len(got) != len(want) {
		t.Errorf("settled set = %v, want exactly %v", got, want)
	}
}

// TestTerminatorTruncatesStaleRows is the amendment that keeps the fix from
// swapping one permanent-skip bug for a permanent-RESCAN bug. A bare
// "no transport-error anywhere in this pair" test is unbounded in page number:
// upsertBucket can only overwrite a row at the same page, so an error row left
// at a high page by an earlier, longer walk would survive a later walk that
// terminated earlier and keep the pair unsettled forever.
func TestTerminatorTruncatesStaleRows(t *testing.T) {
	ctx := context.Background()
	db := newCoverageTestStore(t)

	// An earlier long walk left rows out to page 7, including a failure.
	bucket(t, db, "notices", 2024, 1, "found")
	bucket(t, db, "notices", 2024, 2, "found")
	bucket(t, db, "notices", 2024, 7, "transport-error")

	// A later walk terminates at page 3.
	bucket(t, db, "notices", 2024, 3, "not-found-at-source")
	if err := truncatePairBeyond(ctx, db, "notices", 2024, 3); err != nil {
		t.Fatalf("truncatePairBeyond: %v", err)
	}

	var n int
	if err := db.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM cdc_coverage_buckets WHERE category='notices' AND year_param=2024 AND paged>3`).
		Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d stale rows survived past the terminator; they would keep the pair unsettled forever", n)
	}

	got, err := settledPairs(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if !got[pairKey("notices", 2024)] {
		t.Error("a pair that terminated cleanly at page 3 must settle, even though an earlier longer walk had failed at page 7")
	}
}

// TestSettledPairsIsEmptyOnAFreshStore: a fresh install must not look settled.
func TestSettledPairsIsEmptyOnAFreshStore(t *testing.T) {
	got, err := settledPairs(context.Background(), newCoverageTestStore(t))
	if err != nil {
		t.Fatalf("settledPairs on an empty store: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("fresh store reported %d settled pairs, want 0", len(got))
	}
}

// TestLoadCoverageAgreesWithSettledPairs: the report and the skip logic must
// share one definition, or --strict passes on a corpus the probe path is
// silently refusing to finish.
func TestLoadCoverageAgreesWithSettledPairs(t *testing.T) {
	ctx := context.Background()
	db := newCoverageTestStore(t)

	bucket(t, db, "complete", 2024, 1, "found")
	bucket(t, db, "complete", 2024, 2, "not-found-at-source")
	bucket(t, db, "poisoned", 2024, 1, "found")
	bucket(t, db, "poisoned", 2024, 2, "transport-error")

	view := &coverageView{Mode: "report"}
	if err := loadCoverage(ctx, db, []string{"complete", "poisoned"}, []int{2024}, view); err != nil {
		t.Fatalf("loadCoverage: %v", err)
	}
	if view.PairsTotal != 2 {
		t.Errorf("PairsTotal = %d, want 2", view.PairsTotal)
	}
	if view.PairsSettled != 1 {
		t.Errorf("PairsSettled = %d, want 1 (only 'complete')", view.PairsSettled)
	}
	if view.PairsRemaining != 1 {
		t.Errorf("PairsRemaining = %d, want 1 (the poisoned pair)", view.PairsRemaining)
	}
	if view.PairsPartiallyWalked != 1 {
		t.Errorf("PairsPartiallyWalked = %d, want 1 -- a partially walked pair is more dangerous than a never-asked one because it already has documents stored", view.PairsPartiallyWalked)
	}
	if view.Note == "" {
		t.Error("an unsettled pair must be reported in-band")
	}
}
