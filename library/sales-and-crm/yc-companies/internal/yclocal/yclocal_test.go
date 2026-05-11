package yclocal

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	_ "modernc.org/sqlite"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// Mirror the generator's companies schema (subset needed for tests).
	_, err = db.Exec(`CREATE TABLE companies (
        id TEXT PRIMARY KEY,
        slug TEXT, name TEXT, batch TEXT, status TEXT,
        team_size INTEGER, is_hiring INTEGER, top_company INTEGER,
        industry TEXT, tags TEXT, regions TEXT, one_liner TEXT, launched_at INTEGER
    )`)
	if err != nil {
		t.Fatalf("create companies: %v", err)
	}
	return db
}

func seedCompanies(t *testing.T, db *sql.DB, rows [][]any) {
	t.Helper()
	for _, r := range rows {
		_, err := db.Exec(`INSERT INTO companies(id, slug, name, batch, status, team_size, is_hiring, top_company, industry, tags, regions, one_liner, launched_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, r...)
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
}

func TestParseTags(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"null literal", "null", nil},
		{"json array", `["AI","Devtools"]`, []string{"AI", "Devtools"}},
		{"comma fallback", "AI, Devtools , Robotics", []string{"AI", "Devtools", "Robotics"}},
		{"single value array", `["Robotics"]`, []string{"Robotics"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseTags(c.in)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("parseTags(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestBatchSortKey(t *testing.T) {
	cases := []struct {
		batch string
		want  int
	}{
		{"Winter 2024", 20240},
		{"Spring 2024", 20241},
		{"Summer 2024", 20242},
		{"Fall 2024", 20243},
		{"Winter 2025", 20250},
		{"", 0},
		{"Unknown 2024", 20240},
	}
	for _, c := range cases {
		got := batchSortKey(c.batch)
		if got != c.want {
			t.Errorf("batchSortKey(%q) = %d, want %d", c.batch, got, c.want)
		}
	}
}

func TestBatchProximityBonus(t *testing.T) {
	cases := []struct {
		anchor, peer int
		want         float64
	}{
		{20250, 20250, 0.10}, // same batch
		{20250, 20242, 0.05}, // ~half-year apart (diff 8)
		{20250, 20240, 0.05}, // 1 year apart (diff 10)
		{20250, 20220, 0.02}, // 3 years apart (diff 30)
		{20250, 20100, 0},    // far
		{0, 20250, 0},
		{20250, 0, 0},
	}
	for _, c := range cases {
		got := batchProximityBonus(c.anchor, c.peer)
		if got != c.want {
			t.Errorf("batchProximityBonus(%d, %d) = %v, want %v", c.anchor, c.peer, got, c.want)
		}
	}
}

func TestMatchTo(t *testing.T) {
	cases := []struct {
		v    any
		want string
		ok   bool
	}{
		{"Acquired", "acquired", true},
		{"Active", "acquired", false},
		{true, "true", true},
		{true, "false", false},
		{false, "false", true},
		{int64(50), "50", true},
		{int64(50), "100", false},
	}
	for _, c := range cases {
		if got := matchTo(c.v, c.want); got != c.ok {
			t.Errorf("matchTo(%v, %q) = %v, want %v", c.v, c.want, got, c.ok)
		}
	}
}

func TestRound3(t *testing.T) {
	if got := round3(1.23456); got != 1.235 {
		t.Errorf("round3 = %v", got)
	}
	if got := round3(0); got != 0 {
		t.Errorf("round3 = %v", got)
	}
}

func TestJaccardSets(t *testing.T) {
	anchor := map[string]bool{"ai": true, "devtools": true, "saas": true}
	shared, union := jaccardSets(anchor, []string{"AI", "Robotics", "Saas"})
	if union != 4 { // {ai, devtools, saas, robotics}
		t.Errorf("union = %d, want 4", union)
	}
	if len(shared) != 2 {
		t.Errorf("shared = %v, want 2 items", shared)
	}
}

func TestEnsureSchemaAndWatch(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	defer db.Close()
	seedCompanies(t, db, [][]any{
		{"1", "stripe", "Stripe", "Summer 2009", "Public", 8000, 1, 1, "B2B", `["Payments","Fintech"]`, `["United States"]`, "Payments infrastructure", 1247000000},
		{"2", "airbnb", "Airbnb", "Winter 2009", "Public", 7000, 1, 1, "Consumer", `["Travel"]`, `["United States"]`, "Marketplace for stays", 1230000000},
	})

	added, skipped, err := WatchAdd(ctx, db, []string{"stripe", "ghost", "airbnb"})
	if err != nil {
		t.Fatalf("WatchAdd: %v", err)
	}
	if len(added) != 2 || len(skipped) != 1 {
		t.Errorf("WatchAdd added=%v skipped=%v", added, skipped)
	}
	added2, skipped2, _ := WatchAdd(ctx, db, []string{"stripe"})
	if len(added2) != 0 || len(skipped2) != 1 {
		t.Errorf("re-add: added=%v skipped=%v", added2, skipped2)
	}

	list, err := WatchList(ctx, db)
	if err != nil {
		t.Fatalf("WatchList: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("WatchList = %d, want 2", len(list))
	}

	slugs, _ := WatchedSlugs(ctx, db)
	if len(slugs) != 2 {
		t.Errorf("WatchedSlugs = %v", slugs)
	}

	removed, missed, _ := WatchRemove(ctx, db, []string{"stripe", "ghost"})
	if len(removed) != 1 || len(missed) != 1 {
		t.Errorf("WatchRemove removed=%v missed=%v", removed, missed)
	}
}

func TestSnapshotAndChanges(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	defer db.Close()

	// Initial: stripe Active, team_size=10
	seedCompanies(t, db, [][]any{
		{"1", "stripe", "Stripe", "Summer 2009", "Active", 10, 0, 1, "B2B", `["Payments"]`, `["US"]`, "Payments", 1247000000},
	})
	snap1, n, err := CaptureSnapshot(ctx, db)
	if err != nil {
		t.Fatalf("snapshot1: %v", err)
	}
	if n != 1 {
		t.Errorf("snap1 rows = %d", n)
	}

	// Mutate: now Public, team_size=8000, hiring
	_, _ = db.Exec(`UPDATE companies SET status = 'Public', team_size = 8000, is_hiring = 1 WHERE slug = 'stripe'`)
	// Add a new company that wasn't in snap1
	seedCompanies(t, db, [][]any{
		{"2", "newco", "NewCo", "Winter 2025", "Active", 5, 1, 0, "Consumer", `["AI"]`, `["US"]`, "Brand new", 1730000000},
	})
	snap2, _, err := CaptureSnapshot(ctx, db)
	if err != nil {
		t.Fatalf("snapshot2: %v", err)
	}
	if snap1 == snap2 {
		t.Skip("snapshot ids identical (timestamp resolution); skipping diff")
	}

	// Changes on status
	changes, err := Changes(ctx, db, ChangesQuery{Field: "status", FromSnap: snap1, ToSnap: snap2})
	if err != nil {
		t.Fatalf("changes: %v", err)
	}
	found := false
	for _, c := range changes {
		if c.Slug == "stripe" && c.From == "Active" && c.To == "Public" {
			found = true
		}
	}
	if !found {
		t.Errorf("status change for stripe not detected: %#v", changes)
	}

	// Changes filtered by --to value
	hiringFlips, err := Changes(ctx, db, ChangesQuery{Field: "is_hiring", FromSnap: snap1, ToSnap: snap2, ToValueSet: true, ToValue: "true"})
	if err != nil {
		t.Fatalf("hiring changes: %v", err)
	}
	if len(hiringFlips) == 0 {
		t.Errorf("expected at least one hiring flip")
	}

	// NewSince: newco appeared after snap1
	news, err := NewSince(ctx, db, snap1, 10)
	if err != nil {
		t.Fatalf("new since: %v", err)
	}
	if len(news) != 1 || news[0].Slug != "newco" {
		t.Errorf("NewSince = %v, want [newco]", news)
	}
}

func TestSimilar(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	defer db.Close()
	seedCompanies(t, db, [][]any{
		{"1", "stripe", "Stripe", "Summer 2009", "Public", 8000, 1, 1, "B2B", `["Payments","Fintech","Developer Tools"]`, `["US"]`, "Payments infra", 0},
		{"2", "plaid", "Plaid", "Summer 2014", "Active", 700, 1, 1, "B2B", `["Payments","Fintech","API"]`, `["US"]`, "Fintech infra", 0},
		{"3", "airbnb", "Airbnb", "Winter 2009", "Public", 7000, 1, 1, "Consumer", `["Travel","Marketplace"]`, `["US"]`, "Stays", 0},
		{"4", "dropbox", "Dropbox", "Summer 2007", "Public", 3000, 1, 1, "Consumer", `["Storage"]`, `["US"]`, "Cloud storage", 0},
	})
	hits, err := Similar(ctx, db, "stripe", 5)
	if err != nil {
		t.Fatalf("Similar: %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("no hits")
	}
	// Plaid shares 2 tags with stripe — should rank first.
	if hits[0].Slug != "plaid" {
		t.Errorf("top hit = %s, want plaid", hits[0].Slug)
	}
	if hits[0].TagOverlap == 0 {
		t.Errorf("expected non-zero overlap")
	}
}

func TestStats(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	defer db.Close()
	seedCompanies(t, db, [][]any{
		{"1", "a", "A", "Summer 2024", "Active", 10, 1, 0, "Fintech", `["A"]`, `["US"]`, "", 0},
		{"2", "b", "B", "Summer 2024", "Acquired", 20, 0, 1, "Fintech", `["A"]`, `["US"]`, "", 0},
		{"3", "c", "C", "Winter 2025", "Active", 5, 1, 0, "Fintech", `["A"]`, `["US"]`, "", 0},
	})

	rows, err := Stats(ctx, db, StatsQuery{GroupBy: "batch", Industry: "Fintech"})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("Stats rows = %d, want 2", len(rows))
	}
	// Summer 2024 should come first chronologically
	if rows[0].Key != "Summer 2024" {
		t.Errorf("first batch = %s, want Summer 2024", rows[0].Key)
	}
	if rows[0].Count != 2 {
		t.Errorf("Summer 2024 count = %d, want 2", rows[0].Count)
	}
}

func TestBatchSummary(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	defer db.Close()
	seedCompanies(t, db, [][]any{
		{"1", "a", "A", "Winter 2025", "Active", 5, 1, 0, "Fintech", `["AI"]`, `["US"]`, "", 0},
		{"2", "b", "B", "Winter 2025", "Active", 15, 0, 1, "Fintech", `["AI","Saas"]`, `["US"]`, "", 0},
		{"3", "c", "C", "Winter 2025", "Acquired", 25, 0, 0, "Healthcare", `["Health","AI"]`, `["US"]`, "", 0},
	})
	card, err := BatchSummary(ctx, db, "w25")
	if err != nil {
		t.Fatalf("BatchSummary: %v", err)
	}
	if card.Batch != "Winter 2025" {
		t.Errorf("Batch = %s, want Winter 2025", card.Batch)
	}
	if card.CompanyCount != 3 {
		t.Errorf("CompanyCount = %d, want 3", card.CompanyCount)
	}
	if len(card.TopIndustries) == 0 {
		t.Error("TopIndustries empty")
	}
	if len(card.TopTags) == 0 {
		t.Error("TopTags empty")
	}
	if card.StatusBreakdown["Active"] != 2 {
		t.Errorf("Active = %d, want 2", card.StatusBreakdown["Active"])
	}
}

func TestSnapshotHelpers(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	defer db.Close()
	seedCompanies(t, db, [][]any{
		{"1", "stripe", "Stripe", "Summer 2009", "Active", 100, 1, 1, "B2B", `[]`, `[]`, "", 0},
	})
	id, err := EnsureRecentSnapshot(ctx, db, 0)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if id == "" {
		t.Error("snapshot id empty")
	}
	latest, _ := LatestSnapshotID(ctx, db)
	if latest != id {
		t.Errorf("latest = %s, want %s", latest, id)
	}
	ids, _ := ListSnapshots(ctx, db)
	if len(ids) != 1 {
		t.Errorf("ListSnapshots = %d", len(ids))
	}
}
