// Copyright 2026 adam-birddog and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/sutra-fitness/internal/store"
)

func TestRound2(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{25.0, 25.0},
		{-25.0, -25.0},   // regression: naive int64(f*100+0.5) yields -24.99
		{-100.0, -100.0}, // regression: delta_pct
		{0.0, 0.0},
		{1.236, 1.24},
		{-1.236, -1.24},
		{199.985, 199.99},
	}
	for _, c := range cases {
		if got := round2(c.in); got != c.want {
			t.Errorf("round2(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestPct(t *testing.T) {
	cases := []struct {
		part, whole int
		want        float64
	}{
		{2, 5, 40},
		{1, 3, 33.33},
		{0, 0, 0}, // no divide-by-zero
		{9, 10, 90},
	}
	for _, c := range cases {
		if got := pct(c.part, c.whole); got != c.want {
			t.Errorf("pct(%d,%d) = %v, want %v", c.part, c.whole, got, c.want)
		}
	}
}

func TestParseLocalTime(t *testing.T) {
	if _, ok := parseLocalTime(""); ok {
		t.Error("empty string should not parse")
	}
	if _, ok := parseLocalTime("not-a-date"); ok {
		t.Error("garbage should not parse")
	}
	for _, s := range []string{
		"2026-06-10T08:00:00Z",
		"2026-06-10T08:00:00",
		"2026-06-10 08:00:00",
		"2026-06-10",
		`"2026-06-10T08:00:00Z"`, // quoted (json_extract raw)
	} {
		if _, ok := parseLocalTime(s); !ok {
			t.Errorf("parseLocalTime(%q) failed", s)
		}
	}
}

func newAnalyticsTestStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := store.OpenWithContext(context.Background(), path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db, path
}

func mustUpsert(t *testing.T, fn func(json.RawMessage) error, raw string) {
	t.Helper()
	if err := fn(json.RawMessage(raw)); err != nil {
		t.Fatalf("upsert %s: %v", raw, err)
	}
}

func runNovel(t *testing.T, ctor func(*rootFlags) *cobra.Command, dbPath string, args ...string) []byte {
	t.Helper()
	flags := &rootFlags{asJSON: true}
	cmd := ctor(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(append([]string{"--db", dbPath}, args...))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute %v: %v", args, err)
	}
	return out.Bytes()
}

func TestAnalyticsBehavior(t *testing.T) {
	db, path := newAnalyticsTestStore(t)

	mustUpsert(t, db.UpsertClasses, `{"id":"K1","name":"Flow","instructor_name":"Alice","location_id":"loc1","max_capacity":10,"total_booked":8,"canceled":false,"deleted":false,"start_time":"2026-06-10T08:00:00Z"}`)
	mustUpsert(t, db.UpsertReservations, `{"id":"r1","classes_id":"K1","class_id":"K1","client_id":"c1","status":"CHECKED_IN","checked_in":true,"checked_in_at":"2026-06-10T08:05:00Z"}`)
	mustUpsert(t, db.UpsertReservations, `{"id":"r2","classes_id":"K1","class_id":"K1","client_id":"c2","status":"CHECKED_IN","checked_in":true,"checked_in_at":"2026-06-10T08:05:00Z"}`)
	mustUpsert(t, db.UpsertReservations, `{"id":"r3","classes_id":"K1","class_id":"K1","client_id":"c3","status":"NO_SHOW","checked_in":false}`)
	mustUpsert(t, db.UpsertClients, `{"id":"c1","first_name":"Ann","last_name":"Lee","email":"a@x.com","created_at":"2025-01-01T00:00:00Z","removed":false}`)
	mustUpsert(t, db.UpsertPurchases, `{"id":"p1","client_id":"c1","type":"subscription","name":"Monthly","status":"ACTIVE","price":100.0,"start_date":"2026-06-01T00:00:00Z","end_date":"2026-07-01T00:00:00Z"}`)

	// scorecard: fill 8/10=80, no_show 1/3=33.33, check_in 66.67
	var sc scorecardView
	if err := json.Unmarshal(runNovel(t, newNovelScorecardCmd, path), &sc); err != nil {
		t.Fatalf("scorecard json: %v", err)
	}
	if len(sc.Instructors) != 1 {
		t.Fatalf("scorecard: want 1 instructor, got %d", len(sc.Instructors))
	}
	got := sc.Instructors[0]
	if got.Name != "Alice" || got.FillRate != 80 || got.NoShowRate != 33.33 || got.CheckInRate != 66.67 {
		t.Errorf("scorecard = %+v, want Alice fill80 ns33.33 ci66.67", got)
	}

	// no-shows by instructor
	var ns []noShowRow
	if err := json.Unmarshal(runNovel(t, newNovelNoShowsCmd, path, "--group-by", "instructor"), &ns); err != nil {
		t.Fatalf("no-shows json: %v", err)
	}
	if len(ns) != 1 || ns[0].NoShows != 1 || ns[0].NoShowRate != 33.33 {
		t.Errorf("no-shows = %+v, want 1 no-show @ 33.33%%", ns)
	}

	// ltv: c1 spend 100
	var ltv []ltvRow
	if err := json.Unmarshal(runNovel(t, newNovelLtvCmd, path), &ltv); err != nil {
		t.Fatalf("ltv json: %v", err)
	}
	if len(ltv) != 1 || ltv[0].TotalSpend != 100 || ltv[0].ClientName != "Ann Lee" {
		t.Errorf("ltv = %+v, want Ann Lee 100", ltv)
	}

	// revenue with prior comparison: subscription 100 in current window
	var rev revenueView
	if err := json.Unmarshal(runNovel(t, newNovelRevenueCmd, path, "--compare-prior"), &rev); err != nil {
		t.Fatalf("revenue json: %v", err)
	}
	if rev.TotalRevenue != 100 {
		t.Errorf("revenue total = %v, want 100", rev.TotalRevenue)
	}

	// missing-mirror guard: a non-existent DB yields empty output, exit 0.
	out := runNovel(t, newNovelScorecardCmd, filepath.Join(t.TempDir(), "nope.db"))
	var empty scorecardView
	if err := json.Unmarshal(out, &empty); err != nil {
		t.Fatalf("missing-mirror json: %v", err)
	}
	if len(empty.Instructors) != 0 {
		t.Errorf("missing-mirror scorecard should be empty, got %+v", empty)
	}
}

// TestNoShowRealSemantics locks in the real-Sutra attendance model discovered
// via live testing: attendance is status ATTENDED (no CHECKED_IN/NO_SHOW), and
// a no-show is a BOOKED reservation for a class that has already started. A
// BOOKED reservation for a future class must NOT count as a no-show.
func TestNoShowRealSemantics(t *testing.T) {
	db, path := newAnalyticsTestStore(t)

	// Past class (already started) and a future class.
	mustUpsert(t, db.UpsertClasses, `{"id":"P","name":"Past","instructor_name":"Pat","max_capacity":10,"total_booked":2,"start_time":"2026-06-01T08:00:00Z"}`)
	mustUpsert(t, db.UpsertClasses, `{"id":"F","name":"Future","instructor_name":"Fran","max_capacity":10,"total_booked":1,"start_time":"2099-01-01T08:00:00Z"}`)
	// Past class: one ATTENDED, one BOOKED (= no-show).
	mustUpsert(t, db.UpsertReservations, `{"id":"pa","classes_id":"P","client_id":"c1","status":"ATTENDED","checked_in":true}`)
	mustUpsert(t, db.UpsertReservations, `{"id":"pb","classes_id":"P","client_id":"c2","status":"BOOKED","checked_in":false}`)
	// Future class: BOOKED must NOT be a no-show.
	mustUpsert(t, db.UpsertReservations, `{"id":"fa","classes_id":"F","client_id":"c3","status":"BOOKED","checked_in":false}`)

	var sc scorecardView
	if err := json.Unmarshal(runNovel(t, newNovelScorecardCmd, path), &sc); err != nil {
		t.Fatalf("scorecard json: %v", err)
	}
	byName := map[string]instructorScore{}
	for _, s := range sc.Instructors {
		byName[s.Name] = s
	}
	pat := byName["Pat"]
	if pat.CheckedIn != 1 || pat.NoShows != 1 || pat.NoShowRate != 50 || pat.CheckInRate != 50 {
		t.Errorf("Pat = %+v, want checked_in 1, no_shows 1, rates 50/50", pat)
	}
	fran := byName["Fran"]
	if fran.NoShows != 0 {
		t.Errorf("Fran (future BOOKED) should have 0 no-shows, got %d", fran.NoShows)
	}

	// no-shows command by instructor: Pat has a 50% no-show rate.
	var ns []noShowRow
	if err := json.Unmarshal(runNovel(t, newNovelNoShowsCmd, path, "--group-by", "instructor"), &ns); err != nil {
		t.Fatalf("no-shows json: %v", err)
	}
	var patRow *noShowRow
	for i := range ns {
		if ns[i].Key == "Pat" {
			patRow = &ns[i]
		}
	}
	if patRow == nil || patRow.NoShows != 1 || patRow.NoShowRate != 50 {
		t.Errorf("no-shows Pat = %+v, want 1 no-show @ 50%%", patRow)
	}
}
