// Copyright 2026 Charles Garrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored tests for the governor storage layer: the cross-process
// concurrency lease (cap enforcement + stale reclaim — the headline safety
// property), the credit ledger, SERP snapshots, and the budget ceiling.

package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func newTestExtras(t *testing.T) *ScrapeExtras {
	t.Helper()
	dir := t.TempDir()
	st, err := OpenWithContext(context.Background(), filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	x := NewScrapeExtras(st.DB())
	if err := x.EnsureScrapedoSchema(context.Background()); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	return x
}

func TestLeaseCapEnforcement(t *testing.T) {
	x := newTestExtras(t)
	ctx := context.Background()
	now := time.Now()

	id1, ok1, active1, err := x.AcquireLease(ctx, "a1", 2, now)
	if err != nil || !ok1 || active1 != 1 {
		t.Fatalf("acquire 1: ok=%v active=%d err=%v", ok1, active1, err)
	}
	_, ok2, active2, _ := x.AcquireLease(ctx, "a2", 2, now)
	if !ok2 || active2 != 2 {
		t.Fatalf("acquire 2: ok=%v active=%d", ok2, active2)
	}
	// Third acquire at cap=2 must be refused.
	_, ok3, active3, _ := x.AcquireLease(ctx, "a3", 2, now)
	if ok3 {
		t.Fatalf("acquire 3 should be refused at cap=2 (active=%d)", active3)
	}
	// Releasing one frees a slot.
	if err := x.ReleaseLease(ctx, id1); err != nil {
		t.Fatalf("release: %v", err)
	}
	_, ok4, _, _ := x.AcquireLease(ctx, "a4", 2, now)
	if !ok4 {
		t.Fatal("acquire after release should succeed")
	}
}

func TestLeaseStaleReclaim(t *testing.T) {
	x := newTestExtras(t)
	ctx := context.Background()
	t0 := time.Now()
	if _, ok, _, _ := x.AcquireLease(ctx, "old", 1, t0); !ok {
		t.Fatal("first acquire should succeed")
	}
	// At cap=1 a fresh acquire is refused...
	if _, ok, _, _ := x.AcquireLease(ctx, "blocked", 1, t0); ok {
		t.Fatal("acquire at cap=1 should be refused while a fresh lease is held")
	}
	// ...but once the prior lease is older than the TTL, it is reclaimed.
	future := t0.Add(2 * LeaseTTL)
	if _, ok, _, _ := x.AcquireLease(ctx, "reclaimer", 1, future); !ok {
		t.Fatal("stale lease should be reclaimed, allowing acquire")
	}
}

func TestLeaseUnlimited(t *testing.T) {
	x := newTestExtras(t)
	ctx := context.Background()
	now := time.Now()
	for i := 0; i < 50; i++ {
		if _, ok, _, _ := x.AcquireLease(ctx, "agent", 0, now); !ok {
			t.Fatalf("cap=0 (unlimited) should always grant; failed at %d", i)
		}
	}
}

func TestLedgerSpend(t *testing.T) {
	x := newTestExtras(t)
	ctx := context.Background()
	at := time.Now()
	calls := []CallRecord{
		{Kind: "google:search", Mode: modeGoogleForTest, Agent: "alpha", Cost: 10, CostSource: "header", RemainingCredits: 990, OK: true, At: at},
		{Kind: "scrape", Mode: "datacenter+render", Agent: "beta", Cost: 5, CostSource: "estimate", RemainingCredits: -1, OK: true, At: at},
		{Kind: "scrape", Mode: "datacenter", Agent: "alpha", Cost: 1, CostSource: "header", RemainingCredits: -1, OK: true, At: at},
		// A 429 (unbilled) records the job but no ledger debit.
		{Kind: "scrape", Mode: "datacenter", Agent: "beta", Cost: 0, CostSource: "unbilled", RemainingCredits: -1, OK: false, At: at},
	}
	for _, c := range calls {
		if err := x.RecordCall(ctx, c); err != nil {
			t.Fatalf("record: %v", err)
		}
	}
	spend, err := x.Spend(ctx, at.Add(-time.Hour))
	if err != nil {
		t.Fatalf("spend: %v", err)
	}
	if spend.TotalCredits != 16 {
		t.Errorf("total = %d, want 16 (10+5+1; unbilled excluded)", spend.TotalCredits)
	}
	if spend.Calls != 3 {
		t.Errorf("ledger calls = %d, want 3 (unbilled has no ledger row)", spend.Calls)
	}
	if spend.ByMode[modeGoogleForTest] != 10 {
		t.Errorf("by_mode google = %d, want 10", spend.ByMode[modeGoogleForTest])
	}
	if spend.ByAgent["alpha"] != 11 {
		t.Errorf("by_agent alpha = %d, want 11", spend.ByAgent["alpha"])
	}
}

const modeGoogleForTest = "google"

func TestSnapshotRoundTrip(t *testing.T) {
	x := newTestExtras(t)
	ctx := context.Background()
	hash := "abc123"
	older := SnapshotMeta{ParamHash: hash, Query: "best crm", FetchedAt: time.Now().Add(-time.Hour), Raw: `{"organic_results":[]}`}
	newer := SnapshotMeta{ParamHash: hash, Query: "best crm", FetchedAt: time.Now(), Raw: `{"organic_results":[]}`}
	if err := x.SaveSnapshot(ctx, older, []OrganicRow{{Position: 1, Domain: "a.com"}}); err != nil {
		t.Fatalf("save older: %v", err)
	}
	if err := x.SaveSnapshot(ctx, newer, []OrganicRow{{Position: 1, Domain: "b.com"}, {Position: 2, Domain: "a.com"}}); err != nil {
		t.Fatalf("save newer: %v", err)
	}
	cur, prev, err := x.TwoLatestSnapshots(ctx, hash)
	if err != nil || cur == nil || prev == nil {
		t.Fatalf("two latest: cur=%v prev=%v err=%v", cur, prev, err)
	}
	if !cur.FetchedAt.After(prev.FetchedAt) {
		t.Error("cur should be newer than prev")
	}
	org, err := x.OrganicForSnapshot(ctx, cur.ID)
	if err != nil || len(org) != 2 {
		t.Fatalf("organic for cur: got %d rows err=%v", len(org), err)
	}
	tracked, err := x.TrackedHashes(ctx)
	if err != nil || len(tracked) != 1 || tracked[0].Count != 2 {
		t.Fatalf("tracked: %+v err=%v", tracked, err)
	}
}

func TestSnapshotInsufficientForDrift(t *testing.T) {
	// Absence-of-correctness: a single snapshot must NOT yield a previous to
	// diff against (drift would otherwise fabricate movement).
	x := newTestExtras(t)
	ctx := context.Background()
	_ = x.SaveSnapshot(ctx, SnapshotMeta{ParamHash: "solo", Query: "x", FetchedAt: time.Now()}, nil)
	cur, prev, err := x.TwoLatestSnapshots(ctx, "solo")
	if err != nil {
		t.Fatalf("two latest: %v", err)
	}
	if cur == nil {
		t.Error("expected one snapshot present")
	}
	if prev != nil {
		t.Error("a single snapshot must not produce a previous to diff against")
	}
}

func TestBudgetSetGet(t *testing.T) {
	x := newTestExtras(t)
	ctx := context.Background()
	b0, _ := x.GetBudget(ctx)
	if b0.MaxCredits.Valid {
		t.Error("fresh budget should be unset")
	}
	mc := 500
	if err := x.SetBudget(ctx, &mc, nil, time.Now()); err != nil {
		t.Fatalf("set: %v", err)
	}
	b1, _ := x.GetBudget(ctx)
	if !b1.MaxCredits.Valid || b1.MaxCredits.Int64 != 500 {
		t.Errorf("max_credits = %+v, want 500", b1.MaxCredits)
	}
	// Updating only one field preserves the other.
	pct := 80.0
	if err := x.SetBudget(ctx, nil, &pct, time.Now()); err != nil {
		t.Fatalf("set pct: %v", err)
	}
	b2, _ := x.GetBudget(ctx)
	if !b2.MaxCredits.Valid || b2.MaxCredits.Int64 != 500 {
		t.Error("max_credits should be preserved when only pct is updated")
	}
	if !b2.MaxMonthlyPct.Valid || b2.MaxMonthlyPct.Float64 != 80.0 {
		t.Errorf("max_monthly_pct = %+v, want 80", b2.MaxMonthlyPct)
	}
}

func TestReserveSpendCeiling(t *testing.T) {
	// The reservation counts toward the next reserve's total, so concurrent
	// workers cannot over-grant the ceiling: with ceiling=25 and est=10, the
	// first two reserves fit (0→10, 10→20) and the third (20+10=30) is refused.
	x := newTestExtras(t)
	ctx := context.Background()
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	id1, ok1, t1, err := x.ReserveSpend(ctx, monthStart, 25, 10, "google", "a", "f", now)
	if err != nil || !ok1 || t1 != 0 {
		t.Fatalf("reserve1: ok=%v total=%d err=%v (want ok,0)", ok1, t1, err)
	}
	_, ok2, t2, _ := x.ReserveSpend(ctx, monthStart, 25, 10, "google", "a", "f", now)
	if !ok2 || t2 != 10 {
		t.Fatalf("reserve2: ok=%v total=%d (want ok,10 — reservation 1 counted)", ok2, t2)
	}
	_, ok3, t3, _ := x.ReserveSpend(ctx, monthStart, 25, 10, "google", "a", "f", now)
	if ok3 {
		t.Fatalf("reserve3 must be refused (20+10=30 > 25); total=%d", t3)
	}

	// Reconcile reservation 1 to the authoritative cost; it becomes a real debit.
	if err := x.ReconcileReservation(ctx, id1, CallRecord{Kind: "google:search", Mode: "google", Cost: 10, CostSource: "header", RemainingCredits: -1, OK: true, At: now}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	spend, _ := x.Spend(ctx, monthStart)
	if spend.TotalCredits != 20 {
		t.Errorf("spend total=%d, want 20 (10 reconciled + 10 still-pending reservation)", spend.TotalCredits)
	}
}

func TestReconcileUnbilledDeletesReservation(t *testing.T) {
	// An unbilled outcome (401/429/502/510 → cost 0) must delete the pending
	// reservation so it does not permanently inflate spend.
	x := newTestExtras(t)
	ctx := context.Background()
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	id, ok, _, _ := x.ReserveSpend(ctx, monthStart, 100, 10, "google", "a", "f", now)
	if !ok {
		t.Fatal("reserve should succeed under a 100 ceiling")
	}
	if err := x.ReconcileReservation(ctx, id, CallRecord{Kind: "google:search", Mode: "google", Cost: 0, CostSource: "unbilled", RemainingCredits: -1, OK: false, At: now}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	spend, _ := x.Spend(ctx, monthStart)
	if spend.TotalCredits != 0 {
		t.Errorf("spend total=%d, want 0 (unbilled reservation deleted)", spend.TotalCredits)
	}
}

func TestCancelReservation(t *testing.T) {
	x := newTestExtras(t)
	ctx := context.Background()
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	id, ok, _, _ := x.ReserveSpend(ctx, monthStart, 100, 10, "google", "a", "f", now)
	if !ok {
		t.Fatal("reserve should succeed")
	}
	if err := x.CancelReservation(ctx, id); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	spend, _ := x.Spend(ctx, monthStart)
	if spend.TotalCredits != 0 {
		t.Errorf("spend total=%d, want 0 after cancel", spend.TotalCredits)
	}
}
