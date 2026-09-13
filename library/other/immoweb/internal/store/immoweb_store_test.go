package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/immo"
)

func fp(v float64) *float64 { return &v }
func ip(v int) *int         { return &v }

func openTestImmo(t *testing.T) *Store {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "immo.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.EnsureImmoSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureImmoSchema(context.Background()); err != nil {
		t.Fatalf("schema must be idempotent: %v", err)
	}
	return db
}

func TestUpsertTracksNewAndPriceChanges(t *testing.T) {
	ctx := context.Background()
	db := openTestImmo(t)
	t0 := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	l := immo.Listing{ID: 1000001, Deal: "FOR_SALE", Type: "HOUSE", PostalCode: "1050", Price: fp(400000), Surface: fp(100), Bedrooms: ip(3)}
	if err := db.UpsertImmoListings(ctx, []immo.Listing{l}, nil, t0); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertImmoListings(ctx, []immo.Listing{l}, nil, t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if hist, _ := db.PriceHistory(ctx, l.ID); len(hist) != 1 {
		t.Fatalf("an unchanged price must not add an observation: %+v", hist)
	}
	l.Price = fp(380000)
	if err := db.UpsertImmoListings(ctx, []immo.Listing{l}, nil, t0.Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	hist, err := db.PriceHistory(ctx, l.ID)
	if err != nil || len(hist) != 2 || hist[0].Price != 400000 || hist[1].Price != 380000 {
		t.Fatalf("history = %+v, %v", hist, err)
	}
	all, _ := db.AllPriceHistories(ctx)
	if len(all[l.ID]) != 2 {
		t.Errorf("AllPriceHistories should include listing with 2 prices: %+v", all)
	}
	rows, err := db.QueryListings(ctx, ListingFilter{Deal: "FOR_SALE", PostalCodes: []string{"BE-1050"}})
	if err != nil || len(rows) != 1 || rows[0].PricePerSqm == nil || *rows[0].PricePerSqm != 3800 {
		t.Fatalf("query/pps = %+v, %v", rows, err)
	}
	if rows, _ := db.QueryListings(ctx, ListingFilter{Deal: "FOR_RENT"}); len(rows) != 0 {
		t.Errorf("deal filter leaked rows: %+v", rows)
	}
}

func TestGoneAndSearchRuns(t *testing.T) {
	ctx := context.Background()
	db := openTestImmo(t)
	now := time.Now()
	a := immo.Listing{ID: 2000001, Deal: "FOR_RENT", Type: "APARTMENT", PostalCode: "1060", Price: fp(1200)}
	b := immo.Listing{ID: 2000002, Deal: "FOR_RENT", Type: "APARTMENT", PostalCode: "1060", Price: fp(1300)}
	if err := db.UpsertImmoListings(ctx, []immo.Listing{a, b}, nil, now); err != nil {
		t.Fatal(err)
	}
	crit := immo.Criteria{Types: []string{"APARTMENT"}, Deal: "FOR_RENT", PostalCodes: []string{"BE-1060"}}
	if err := db.SaveSearch(ctx, "sg", crit); err != nil {
		t.Fatal(err)
	}
	if err := db.RecordSearchRun(ctx, "sg", map[int64]*float64{a.ID: a.Price, b.ID: b.Price}, nil, now); err != nil {
		t.Fatal(err)
	}
	seen, _ := db.SeenIDs(ctx, "sg")
	if len(seen) != 2 {
		t.Fatalf("seen = %v", seen)
	}
	if seen[a.ID].LastPrice == nil || *seen[a.ID].LastPrice != 1200 {
		t.Errorf("seen price not recorded: %+v", seen[a.ID])
	}
	if err := db.RecordSearchRun(ctx, "sg", map[int64]*float64{a.ID: a.Price}, []int64{b.ID}, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if n, err := db.MarkGone(ctx, []int64{a.ID}, now.Add(-time.Minute), now.Add(time.Hour)); err != nil || n != 0 {
		t.Fatalf("a listing seen after the harvest started must not be marked gone: %d %v", n, err)
	}
	if n, err := db.MarkGone(ctx, []int64{b.ID}, now.Add(30*time.Minute), now.Add(time.Hour)); err != nil || n != 1 {
		t.Fatalf("MarkGone = %d %v", n, err)
	}
	seen, _ = db.SeenIDs(ctx, "sg")
	if len(seen) != 1 {
		t.Errorf("gone listing should leave the seen set: %v", seen)
	}
	active, _ := db.QueryListings(ctx, ListingFilter{Deal: "FOR_RENT"})
	if len(active) != 1 || active[0].ID != a.ID {
		t.Errorf("gone listing must be excluded by default: %+v", active)
	}
	gone, _ := db.QueryListings(ctx, ListingFilter{GoneSince: now.Add(-time.Hour).UTC().Format(time.RFC3339)})
	if len(gone) != 1 || gone[0].ID != b.ID {
		t.Errorf("GoneSince = %+v", gone)
	}
	// Seeing it again revives it.
	if err := db.UpsertImmoListings(ctx, []immo.Listing{b}, nil, now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if active, _ := db.QueryListings(ctx, ListingFilter{Deal: "FOR_RENT"}); len(active) != 2 {
		t.Errorf("re-seen listing should be active again: %d", len(active))
	}
	ss, err := db.GetSearch(ctx, "sg")
	if err != nil || ss.LastRunAt == "" || len(ss.Criteria.PostalCodes) != 1 {
		t.Errorf("GetSearch = %+v, %v", ss, err)
	}
	// Changing the criteria resets the watch history.
	crit.MaxPrice = 1500
	if err := db.SaveSearch(ctx, "sg", crit); err != nil {
		t.Fatal(err)
	}
	if ss, _ := db.GetSearch(ctx, "sg"); ss.LastRunAt != "" {
		t.Errorf("criteria change must reset last_run_at, got %q", ss.LastRunAt)
	}
	if seen, _ := db.SeenIDs(ctx, "sg"); len(seen) != 0 {
		t.Errorf("criteria change must clear the seen set: %v", seen)
	}
	if ok, _ := db.DeleteSearch(ctx, "sg"); !ok {
		t.Error("DeleteSearch should report true")
	}
	if ok, _ := db.DeleteSearch(ctx, "sg"); ok {
		t.Error("second delete should report false")
	}
}

func TestHiddenShortlistAndPulls(t *testing.T) {
	ctx := context.Background()
	db := openTestImmo(t)
	if err := db.SetHidden(ctx, 42, true, "too dark"); err != nil {
		t.Fatal(err)
	}
	h, _ := db.HiddenSet(ctx)
	if !h[42] {
		t.Error("hidden set missing 42")
	}
	_ = db.SetHidden(ctx, 42, false, "")
	if h, _ := db.HiddenSet(ctx); h[42] {
		t.Error("unhide failed")
	}
	_ = db.SetShortlist(ctx, 7, true, "visit")
	_ = db.SetShortlist(ctx, 7, true, "")
	sl, _ := db.ListShortlist(ctx)
	if len(sl) != 1 || sl[0].Note != "visit" {
		t.Errorf("shortlist note should survive an empty re-add: %+v", sl)
	}
	if at, _, _ := db.LastPull(ctx, "scope"); !at.IsZero() {
		t.Error("unknown scope must return zero time")
	}
	_ = db.RecordPull(ctx, "scope", 12, time.Now())
	if at, n, _ := db.LastPull(ctx, "scope"); at.IsZero() || n != 12 {
		t.Errorf("LastPull = %v %d", at, n)
	}
}
