// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	st, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestJobsUpsertListSearch(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	if err := st.UpsertJob(ctx, Job{PromptID: "p1", Kind: "depthmap", Status: "completed", ImageURL: "u1", Params: `{"style":"pro"}`, ResultURLs: `["a","b"]`, CreditCost: 15, CreatedAt: "2026-01-01 00:00:00"}); err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}
	if err := st.UpsertJob(ctx, Job{PromptID: "p2", Kind: "stl", Status: "submitted", CreditCost: 3, CreatedAt: "2026-01-02 00:00:00"}); err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}
	// Upsert same prompt updates, not duplicates.
	if err := st.UpsertJob(ctx, Job{PromptID: "p1", Kind: "depthmap", Status: "completed", CreditCost: 15, CreatedAt: "2026-01-01 00:00:00"}); err != nil {
		t.Fatalf("UpsertJob update: %v", err)
	}

	n, err := st.CountJobs(ctx)
	if err != nil || n != 2 {
		t.Fatalf("CountJobs = %d, %v; want 2", n, err)
	}

	jobs, err := st.ListJobs(ctx, 10)
	if err != nil || len(jobs) != 2 {
		t.Fatalf("ListJobs = %d, %v; want 2", len(jobs), err)
	}
	// Newest first by created_at.
	if jobs[0].PromptID != "p2" {
		t.Fatalf("ListJobs order: got %s first, want p2", jobs[0].PromptID)
	}

	found, err := st.SearchJobs(ctx, "stl", 10)
	if err != nil || len(found) != 1 || found[0].PromptID != "p2" {
		t.Fatalf("SearchJobs(stl) = %v, %v; want [p2]", found, err)
	}
	// Negative: a term matching nothing returns empty, not all rows.
	none, err := st.SearchJobs(ctx, "nonexistent-term-xyz", 10)
	if err != nil || len(none) != 0 {
		t.Fatalf("SearchJobs(miss) = %d rows; want 0", len(none))
	}
}

func TestCreditEventsAnalytics(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	events := []CreditEvent{
		{ID: "e1", ActionType: 5, ChangeNum: -10, Remarks: "API Draw: p1", CreateDate: "2026-01-01 10:00:00"},
		{ID: "e2", ActionType: 5, ChangeNum: -3, Remarks: "API Draw: p2", CreateDate: "2026-01-01 11:00:00"},
		{ID: "e3", ActionType: 1, ChangeNum: 100, Remarks: "check-in", CreateDate: "2026-01-02 09:00:00"},
	}
	for _, e := range events {
		if err := st.UpsertCreditEvent(ctx, e); err != nil {
			t.Fatalf("UpsertCreditEvent: %v", err)
		}
	}

	groups, err := st.AnalyticsCreditEvents(ctx, "actionType", 10)
	if err != nil {
		t.Fatalf("Analytics: %v", err)
	}
	byGroup := map[string]GroupCount{}
	for _, g := range groups {
		byGroup[g.Group] = g
	}
	if byGroup["5"].Count != 2 || byGroup["5"].TotalChange != -13 {
		t.Fatalf("actionType 5 = count %d total %d; want 2 / -13", byGroup["5"].Count, byGroup["5"].TotalChange)
	}
	if byGroup["1"].TotalChange != 100 {
		t.Fatalf("actionType 1 total = %d; want 100", byGroup["1"].TotalChange)
	}

	// day grouping
	dayGroups, err := st.AnalyticsCreditEvents(ctx, "day", 10)
	if err != nil || len(dayGroups) != 2 {
		t.Fatalf("Analytics day = %d groups, %v; want 2", len(dayGroups), err)
	}

	// unsupported group-by is an error, not a silent empty result.
	if _, err := st.AnalyticsCreditEvents(ctx, "bogus", 10); err == nil {
		t.Fatal("Analytics(bogus) should error")
	}
}

func TestSearchCreditEvents(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	// One older event matching "p1", then many newer non-matching events. A
	// fetch-newest-N-then-filter approach would push the match out of the
	// window; SQL-level filtering keeps it findable under a small --limit.
	if err := st.UpsertCreditEvent(ctx, CreditEvent{ID: "old", ChangeNum: -10, Remarks: "API Draw: p1-target", CreateDate: "2026-01-01 00:00:00"}); err != nil {
		t.Fatalf("UpsertCreditEvent: %v", err)
	}
	for i := 0; i < 30; i++ {
		id := "newer-" + strconv.Itoa(i)
		date := "2026-02-" + strconv.Itoa(i+1) + " 00:00:00"
		if err := st.UpsertCreditEvent(ctx, CreditEvent{ID: id, ChangeNum: 5, Remarks: "check-in", CreateDate: date}); err != nil {
			t.Fatalf("UpsertCreditEvent: %v", err)
		}
	}

	// Default-sized limit: the match is far outside the newest-20 window.
	found, err := st.SearchCreditEvents(ctx, "p1-target", 20)
	if err != nil || len(found) != 1 || found[0].ID != "old" {
		t.Fatalf("SearchCreditEvents(p1-target) = %v, %v; want [old]", found, err)
	}

	// Empty term returns the most recent N (acts like ListCreditEvents).
	recent, err := st.SearchCreditEvents(ctx, "", 5)
	if err != nil || len(recent) != 5 {
		t.Fatalf("SearchCreditEvents(\"\") = %d rows, %v; want 5", len(recent), err)
	}

	// A term matching nothing returns empty, not all rows.
	none, err := st.SearchCreditEvents(ctx, "nonexistent-xyz", 20)
	if err != nil || len(none) != 0 {
		t.Fatalf("SearchCreditEvents(miss) = %d rows; want 0", len(none))
	}

	// LIKE metacharacters in the term match literally, not as wildcards.
	if err := st.UpsertCreditEvent(ctx, CreditEvent{ID: "pct", ChangeNum: -1, Remarks: "50% off promo", CreateDate: "2026-03-01 00:00:00"}); err != nil {
		t.Fatalf("UpsertCreditEvent: %v", err)
	}
	pct, err := st.SearchCreditEvents(ctx, "50%", 20)
	if err != nil || len(pct) != 1 || pct[0].ID != "pct" {
		t.Fatalf("SearchCreditEvents(50%%) = %v, %v; want [pct]", pct, err)
	}
}

func TestReconcile(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	// A recorded job for p1, but not p2.
	if err := st.UpsertJob(ctx, Job{PromptID: "p1", Kind: "depthmap", CreatedAt: "2026-01-01 00:00:00"}); err != nil {
		t.Fatalf("UpsertJob: %v", err)
	}
	// Two spend events: one matches p1, one references p2 (no job).
	_ = st.UpsertCreditEvent(ctx, CreditEvent{ID: "e1", ChangeNum: -10, Remarks: "API Draw: p1", CreateDate: "2026-01-01 10:00:00"})
	_ = st.UpsertCreditEvent(ctx, CreditEvent{ID: "e2", ChangeNum: -3, Remarks: "API Draw: p2", CreateDate: "2026-01-01 11:00:00"})
	// A positive (earn) event must be ignored.
	_ = st.UpsertCreditEvent(ctx, CreditEvent{ID: "e3", ChangeNum: 100, Remarks: "check-in", CreateDate: "2026-01-02 09:00:00"})

	rows, err := st.Reconcile(ctx, 100)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(rows) != 1 || rows[0].EventID != "e2" {
		t.Fatalf("Reconcile = %v; want only e2 unmatched", rows)
	}
}

func TestDrawings(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	if err := st.UpsertDrawing(ctx, Drawing{ID: "d1", ImgURL: "http://x/1.png", CreateDate: "2026-01-01 00:00:00"}); err != nil {
		t.Fatalf("UpsertDrawing: %v", err)
	}
	n, err := st.CountDrawings(ctx)
	if err != nil || n != 1 {
		t.Fatalf("CountDrawings = %d, %v; want 1", n, err)
	}
	rows, err := st.ListDrawings(ctx, 10)
	if err != nil || len(rows) != 1 || rows[0].ImgURL != "http://x/1.png" {
		t.Fatalf("ListDrawings = %v, %v", rows, err)
	}
}

func TestSearchDrawings(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	drawings := []Drawing{
		{ID: "d1", ImgURL: "http://x/cat.png", CreateDate: "2026-01-01 00:00:00"},
		{ID: "d2", ImgURL: "http://x/dog.jpg", CreateDate: "2026-01-02 00:00:00"},
	}
	for _, d := range drawings {
		if err := st.UpsertDrawing(ctx, d); err != nil {
			t.Fatalf("UpsertDrawing: %v", err)
		}
	}

	// A term filters rather than being silently discarded.
	found, err := st.SearchDrawings(ctx, "cat", 10)
	if err != nil || len(found) != 1 || found[0].ID != "d1" {
		t.Fatalf("SearchDrawings(cat) = %v, %v; want [d1]", found, err)
	}
	// Matches on id too.
	byID, err := st.SearchDrawings(ctx, "d2", 10)
	if err != nil || len(byID) != 1 || byID[0].ID != "d2" {
		t.Fatalf("SearchDrawings(d2) = %v, %v; want [d2]", byID, err)
	}
	// Empty term lists recent rows.
	all, err := st.SearchDrawings(ctx, "", 10)
	if err != nil || len(all) != 2 {
		t.Fatalf("SearchDrawings(\"\") = %d rows, %v; want 2", len(all), err)
	}
	// A miss returns empty, not every row.
	none, err := st.SearchDrawings(ctx, "nonexistent-xyz", 10)
	if err != nil || len(none) != 0 {
		t.Fatalf("SearchDrawings(miss) = %d rows; want 0", len(none))
	}
	// LIKE metacharacters match literally.
	if err := st.UpsertDrawing(ctx, Drawing{ID: "d3", ImgURL: "http://x/50%off.png", CreateDate: "2026-01-03 00:00:00"}); err != nil {
		t.Fatalf("UpsertDrawing: %v", err)
	}
	pct, err := st.SearchDrawings(ctx, "50%", 10)
	if err != nil || len(pct) != 1 || pct[0].ID != "d3" {
		t.Fatalf("SearchDrawings(50%%) = %v, %v; want [d3]", pct, err)
	}
}

func TestOpenReadOnlyMissing(t *testing.T) {
	ctx := context.Background()
	missing := filepath.Join(t.TempDir(), "nope.db")
	st, ok, err := OpenReadOnly(ctx, missing)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	if ok || st != nil {
		t.Fatal("OpenReadOnly on missing file should return ok=false, nil store")
	}
}
