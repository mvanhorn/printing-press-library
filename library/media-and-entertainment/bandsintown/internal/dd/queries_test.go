package dd

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

// openTest opens a fresh DB inside the test's tempdir; closes it on cleanup.
func openTest(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestEscapeArtistName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Phoenix", "Phoenix"},
		{"AC/DC", "AC%252FDC"},
		{`Why?`, "Why%253F"},
		{`*Star*`, "%252AStar%252A"},
		{`She said "hi"`, `She said %27Chi%27C`},
		{`Normal Band`, `Normal Band`},
	}
	for _, c := range cases {
		got := EscapeArtistName(c.in)
		if got != c.want {
			t.Errorf("EscapeArtistName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAddListRemoveTracked(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	if err := AddTracked(ctx, db, "Phoenix", "mid"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := AddTracked(ctx, db, "Beach House", ""); err != nil {
		t.Fatalf("add 2: %v", err)
	}

	rows, err := ListTracked(ctx, db)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len=%d want 2", len(rows))
	}
	if rows[0].Name != "Beach House" || rows[1].Name != "Phoenix" {
		t.Errorf("ordering wrong: %v", rows)
	}

	// Idempotent: adding Phoenix again should not double-insert.
	if err := AddTracked(ctx, db, "Phoenix", "headliner"); err != nil {
		t.Fatalf("add idempotent: %v", err)
	}
	rows, _ = ListTracked(ctx, db)
	if len(rows) != 2 {
		t.Errorf("expected idempotent add, got len=%d", len(rows))
	}

	ok, err := RemoveTracked(ctx, db, "Phoenix")
	if err != nil || !ok {
		t.Fatalf("remove Phoenix: ok=%v err=%v", ok, err)
	}
	ok, _ = RemoveTracked(ctx, db, "Nonexistent")
	if ok {
		t.Errorf("remove of missing artist should return false")
	}
}

func TestRouteAndGaps(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	// Seed: Phoenix tracked, three events — Singapore 2026-08-12, Bangkok
	// 2026-08-22, Paris 2027-01-01. Target Jakarta 2026-08-15.
	_ = AddTracked(ctx, db, "Phoenix", "mid")
	_ = UpsertArtist(ctx, db, &Artist{Name: "Phoenix", TrackerCount: 50_000, UpcomingEventCount: 10})
	events := []Event{
		{ID: "e1", ArtistName: "Phoenix", Datetime: "2026-08-12T20:00:00", VenueCity: "Singapore", VenueCountry: "Singapore", VenueLat: 1.3521, VenueLng: 103.8198},
		{ID: "e2", ArtistName: "Phoenix", Datetime: "2026-08-22T20:00:00", VenueCity: "Bangkok", VenueCountry: "Thailand", VenueLat: 13.7563, VenueLng: 100.5018},
		{ID: "e3", ArtistName: "Phoenix", Datetime: "2027-01-01T20:00:00", VenueCity: "Paris", VenueCountry: "France", VenueLat: 48.85, VenueLng: 2.35},
	}
	if _, _, err := ReplaceEvents(ctx, db, "Phoenix", events); err != nil {
		t.Fatalf("replace: %v", err)
	}

	target, _ := time.Parse("2006-01-02", "2026-08-15")
	// 5-day window: only Singapore Aug 12 (gap=3) qualifies; Bangkok Aug 22 (gap=7) is out.
	rows, err := Route(ctx, db, -6.2088, 106.8456, target, 5, true, true)
	if err != nil {
		t.Fatalf("route: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 routing candidate (Singapore Aug 12) with 5d window, got %d: %+v", len(rows), rows)
	}
	if rows[0].VenueCity != "Singapore" {
		t.Errorf("expected Singapore, got %s", rows[0].VenueCity)
	}
	if rows[0].DistanceKM < 800 || rows[0].DistanceKM > 950 {
		t.Errorf("haversine Singapore→Jakarta expected ~880km, got %.0f", rows[0].DistanceKM)
	}
	if rows[0].Score == 0 {
		t.Errorf("score should be > 0 when --score on")
	}

	// Window widening to 30d should pick up Bangkok too.
	rows, _ = Route(ctx, db, -6.2088, 106.8456, target, 30, true, false)
	if len(rows) != 2 {
		t.Errorf("expected 2 routing candidates with 30d window, got %d", len(rows))
	}

	// Gaps: between e1 (Aug 12) and e2 (Aug 22) is 10 days, in SEA on both ends.
	gaps, err := Gaps(ctx, db, "Phoenix", 5, 30, "SEA")
	if err != nil {
		t.Fatalf("gaps: %v", err)
	}
	if len(gaps) != 1 {
		t.Fatalf("expected 1 SEA gap, got %d: %+v", len(gaps), gaps)
	}
	if gaps[0].GapDays != 10 {
		t.Errorf("expected 10-day gap, got %d", gaps[0].GapDays)
	}

	// Region filter eliminates the Bangkok→Paris span.
	gaps, _ = Gaps(ctx, db, "Phoenix", 5, 400, "SEA")
	if len(gaps) != 1 {
		t.Errorf("Paris leg should be filtered out, got %d gaps", len(gaps))
	}
}

func TestLineupCoBill(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	events := []Event{
		{ID: "f1", ArtistName: "Phoenix", Datetime: "2025-06-01T20:00:00", Lineup: []string{"Phoenix", "Beach House", "Tame Impala"}},
		{ID: "f2", ArtistName: "Phoenix", Datetime: "2025-07-04T20:00:00", Lineup: []string{"Phoenix", "Beach House"}},
		{ID: "f3", ArtistName: "Phoenix", Datetime: "2025-08-10T20:00:00", Lineup: []string{"Phoenix", "Tame Impala", "MGMT"}},
	}
	if _, _, err := ReplaceEvents(ctx, db, "Phoenix", events); err != nil {
		t.Fatalf("replace: %v", err)
	}

	rows, err := LineupCoBill(ctx, db, "Phoenix", "", 2)
	if err != nil {
		t.Fatalf("co-bill: %v", err)
	}
	// Beach House appears 2x, Tame Impala 2x, MGMT 1x — min-shared 2 returns 2 rows.
	if len(rows) != 2 {
		t.Fatalf("expected 2 co-bill rows with min-shared=2, got %d: %+v", len(rows), rows)
	}
	got := map[string]int{}
	for _, r := range rows {
		got[r.Collaborator] = r.Shared
	}
	if got["Beach House"] != 2 || got["Tame Impala"] != 2 {
		t.Errorf("unexpected co-bill counts: %v", got)
	}

	rows, _ = LineupCoBill(ctx, db, "Phoenix", "", 1)
	if len(rows) != 3 {
		t.Errorf("expected 3 with min-shared=1 (Beach House, Tame Impala, MGMT), got %d", len(rows))
	}
}

func TestSnapshotAndTrend(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	a := &Artist{Name: "Phoenix", TrackerCount: 100, UpcomingEventCount: 5}
	// Insert an old snapshot directly via SQL (bypassing time.Now()).
	if _, err := db.ExecContext(ctx,
		`INSERT INTO dd_artist_snapshots (artist_name, snapped_at, tracker_count, upcoming_event_count) VALUES (?,?,?,?)`,
		"Phoenix", time.Now().Add(-10*24*time.Hour).UTC().Format(time.RFC3339), 80, 4); err != nil {
		t.Fatalf("seed old snapshot: %v", err)
	}
	if _, err := SnapshotArtist(ctx, db, a); err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	rows, err := Trend(ctx, db, 30, 10)
	if err != nil {
		t.Fatalf("trend: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 trend row, got %d", len(rows))
	}
	if rows[0].Delta != 20 {
		t.Errorf("expected delta=20 (100-80), got %d", rows[0].Delta)
	}
	if rows[0].PercentChange < 24 || rows[0].PercentChange > 26 {
		t.Errorf("expected ~25%% change, got %.1f", rows[0].PercentChange)
	}
}

func TestSeaRadar(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()
	_ = AddTracked(ctx, db, "Phoenix", "mid")
	_ = AddTracked(ctx, db, "Random", "emerging")
	_ = UpsertArtist(ctx, db, &Artist{Name: "Phoenix", TrackerCount: 50_000})
	_ = UpsertArtist(ctx, db, &Artist{Name: "Random", TrackerCount: 100})
	_ = UpsertArtist(ctx, db, &Artist{Name: "Untracked", TrackerCount: 9_000_000})

	if _, _, err := ReplaceEvents(ctx, db, "Phoenix", []Event{
		{ID: "s1", ArtistName: "Phoenix", Datetime: "2026-08-12T20:00:00", VenueCity: "Singapore", VenueCountry: "Singapore", VenueLat: 1.35, VenueLng: 103.82},
		{ID: "s3", ArtistName: "Phoenix", Datetime: "2026-09-01T20:00:00", VenueCity: "Paris", VenueCountry: "France"},
	}); err != nil {
		t.Fatalf("replace phoenix: %v", err)
	}
	if _, _, err := ReplaceEvents(ctx, db, "Random", []Event{
		{ID: "s2", ArtistName: "Random", Datetime: "2026-08-15T20:00:00", VenueCity: "Bangkok", VenueCountry: "Thailand", VenueLat: 13.75, VenueLng: 100.5},
	}); err != nil {
		t.Fatalf("replace random: %v", err)
	}
	if _, _, err := ReplaceEvents(ctx, db, "Untracked", []Event{
		{ID: "s4", ArtistName: "Untracked", Datetime: "2026-08-20T20:00:00", VenueCity: "Singapore", VenueCountry: "Singapore"},
	}); err != nil {
		t.Fatalf("replace untracked: %v", err)
	}

	rows, err := SeaRadar(ctx, db, "2026-08-01", "2026-08-31", "")
	if err != nil {
		t.Fatalf("sea-radar: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows (Phoenix+Random), got %d: %+v", len(rows), rows)
	}
	for _, r := range rows {
		if r.Artist == "Untracked" {
			t.Errorf("untracked artist leaked into sea-radar: %+v", r)
		}
		if r.Country == "France" {
			t.Errorf("non-SEA country leaked into sea-radar: %+v", r)
		}
	}

	rows, _ = SeaRadar(ctx, db, "2026-08-01", "2026-08-31", "mid")
	if len(rows) != 1 || rows[0].Artist != "Phoenix" {
		t.Errorf("tier filter expected 1 Phoenix row, got %d: %+v", len(rows), rows)
	}
}

func TestParseRegionFilter(t *testing.T) {
	if parseRegionFilter("") != nil {
		t.Errorf("empty region should return nil filter")
	}
	sea := parseRegionFilter("SEA")
	if !sea["Indonesia"] || !sea["ID"] || !sea["Singapore"] {
		t.Errorf("SEA expansion missing entries: %v", sea)
	}
	custom := parseRegionFilter("Japan,KR")
	if !custom["Japan"] || !custom["KR"] || custom["US"] {
		t.Errorf("custom filter wrong: %v", custom)
	}
}

func TestCountTracked(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()
	n, _ := CountTracked(ctx, db)
	if n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
	_ = AddTracked(ctx, db, "A", "")
	_ = AddTracked(ctx, db, "B", "")
	n, _ = CountTracked(ctx, db)
	if n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
}

func TestReplaceEventsDiff(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	// First pull: 3 events.
	a := "Phoenix"
	v1 := []Event{
		{ID: "e1", ArtistName: a, Datetime: "2026-08-12T20:00:00"},
		{ID: "e2", ArtistName: a, Datetime: "2026-08-22T20:00:00"},
		{ID: "e3", ArtistName: a, Datetime: "2026-09-01T20:00:00"},
	}
	added, removed, err := ReplaceEvents(ctx, db, a, v1)
	if err != nil {
		t.Fatalf("first pull: %v", err)
	}
	if added != 3 || removed != 0 {
		t.Errorf("first pull expected added=3 removed=0, got %d/%d", added, removed)
	}

	// Reschedule: same count, completely different IDs. Net-delta logic would
	// report 0/0; set-difference logic must report 3/3.
	v2 := []Event{
		{ID: "e4", ArtistName: a, Datetime: "2026-08-13T20:00:00"},
		{ID: "e5", ArtistName: a, Datetime: "2026-08-23T20:00:00"},
		{ID: "e6", ArtistName: a, Datetime: "2026-09-02T20:00:00"},
	}
	added, removed, err = ReplaceEvents(ctx, db, a, v2)
	if err != nil {
		t.Fatalf("reschedule: %v", err)
	}
	if added != 3 || removed != 3 {
		t.Errorf("reschedule (3-for-3 swap) expected added=3 removed=3, got %d/%d", added, removed)
	}

	// Partial overlap: keep e4 and e5, drop e6, add e7. Expect added=1 removed=1.
	v3 := []Event{
		{ID: "e4", ArtistName: a, Datetime: "2026-08-13T20:00:00"},
		{ID: "e5", ArtistName: a, Datetime: "2026-08-23T20:00:00"},
		{ID: "e7", ArtistName: a, Datetime: "2026-09-09T20:00:00"},
	}
	added, removed, err = ReplaceEvents(ctx, db, a, v3)
	if err != nil {
		t.Fatalf("partial overlap: %v", err)
	}
	if added != 1 || removed != 1 {
		t.Errorf("partial-overlap expected added=1 removed=1, got %d/%d", added, removed)
	}

	// No-op pull: identical event set. Expect 0/0.
	added, removed, err = ReplaceEvents(ctx, db, a, v3)
	if err != nil {
		t.Fatalf("no-op: %v", err)
	}
	if added != 0 || removed != 0 {
		t.Errorf("no-op pull expected added=0 removed=0, got %d/%d", added, removed)
	}
}
