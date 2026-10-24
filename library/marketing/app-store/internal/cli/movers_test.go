// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

func TestMoversHelpWires(t *testing.T) {
	r := s3aRun(t, "movers", "--help")
	if r.code != 0 || !strings.Contains(r.stdout, "movers [flags]") || !strings.Contains(r.stdout, "climbed") {
		t.Fatalf("movers --help: exit %d\n%s", r.code, r.stdout)
	}
	// Every movers flag has a default, so a bare invocation runs the default
	// comparison (here against an empty store) instead of printing help.
	if r := s3aRun(t, "movers"); r.code != 0 || strings.Contains(r.stdout, "Usage:") {
		t.Fatalf("bare movers: exit %d\n%s", r.code, r.stdout)
	}
}

func TestMoversTwoSnapshotsEnteredExited(t *testing.T) {
	db := s3aSeededDB(t)
	r := s3aRun(t, "movers", "--db", db, "--genre", "6014", "--country", "us", "--json")
	var rows []moversRow
	s3aDecode(t, r, &rows)
	got := map[string][]string{}
	byApp := map[string]moversRow{}
	for _, row := range rows {
		got[row.Movement] = append(got[row.Movement], row.AppID)
		byApp[row.AppID] = row
	}
	want := map[string][]string{
		"climbed": {"1621328561", "1482155847"},
		"dropped": {"1094591345", "553834731"},
		"entered": {"6446308106"},
		"exited":  {"431946152"},
	}
	for kind, ids := range want {
		if strings.Join(got[kind], ",") != strings.Join(ids, ",") {
			t.Errorf("%s = %v, want %v (in order)", kind, got[kind], ids)
		}
	}
	if len(rows) != 6 {
		t.Fatalf("rows = %d, want 6 (unchanged apps must not appear): %+v", len(rows), rows)
	}
	if order := []string{rows[0].Movement, rows[2].Movement, rows[4].Movement, rows[5].Movement}; strings.Join(order, ",") != "climbed,dropped,entered,exited" {
		t.Fatalf("movement order = %v", order)
	}
	entered := byApp["6446308106"]
	if entered.Rank == nil || *entered.Rank != 4 || entered.PreviousRank != nil || entered.Delta != nil || entered.Title != "Sample New Game" {
		t.Fatalf("entered row = %+v", entered)
	}
	exited := byApp["431946152"]
	if exited.Rank != nil || exited.PreviousRank == nil || *exited.PreviousRank != 4 || exited.Delta != nil || exited.Title != "Roblox" {
		t.Fatalf("exited row = %+v", exited)
	}
	mono := byApp["1621328561"]
	if *mono.Delta != 2 || *mono.Rank != 3 || *mono.PreviousRank != 5 {
		t.Fatalf("climber delta = %+v", mono)
	}
	poke := byApp["1094591345"]
	if *poke.Delta != -2 || *poke.Rank != 5 || *poke.PreviousRank != 3 {
		t.Fatalf("dropper delta = %+v", poke)
	}
	if !entered.CapturedAt.Equal(s3aT0.Add(24*time.Hour)) || !entered.PreviousCapturedAt.Equal(s3aT0) {
		t.Fatalf("capture times = %v / %v", entered.CapturedAt, entered.PreviousCapturedAt)
	}
	if !strings.Contains(r.stderr, "1 entered, 1 exited") {
		t.Fatalf("stderr summary missing: %q", r.stderr)
	}
	// Raw JSON keeps explicit nulls for the absent side.
	if !strings.Contains(r.stdout, `"rank": null`) || !strings.Contains(r.stdout, `"previousRank": null`) {
		t.Fatalf("expected explicit nulls:\n%s", r.stdout)
	}
}

func TestMoversOneSnapshotReturnsEmpty(t *testing.T) {
	db := s3aSeededDB(t)
	r := s3aRun(t, "movers", "--db", db, "--chart", "top-paid", "--country", "gb", "--json")
	if r.code != 0 || strings.TrimSpace(r.stdout) != "[]" {
		t.Fatalf("one snapshot: exit %d stdout %q", r.code, r.stdout)
	}
	if !strings.Contains(r.stderr, "only one comparable capture") {
		t.Fatalf("missing honest note: %q", r.stderr)
	}
}

func TestMoversEmptyStoreAndMissingMirror(t *testing.T) {
	r := s3aRun(t, "movers", "--db", t.TempDir()+"/absent.db", "--json")
	if r.code != 0 || strings.TrimSpace(r.stdout) != "[]" || !strings.Contains(r.stderr, "no local mirror at") || !strings.Contains(r.stderr, "top --chart top-free") {
		t.Fatalf("missing mirror: exit %d stdout %q stderr %q", r.code, r.stdout, r.stderr)
	}
	r = s3aRun(t, "movers", "--db", s3aEmptyDB(t), "--json")
	if r.code != 0 || strings.TrimSpace(r.stdout) != "[]" || !strings.Contains(r.stderr, "no stored captures") {
		t.Fatalf("empty store: exit %d stdout %q stderr %q", r.code, r.stdout, r.stderr)
	}
}

func TestMoversIgnoresIncomparableCaptures(t *testing.T) {
	db := s3aSeededDB(t)
	st, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	// A later RSS capture of the web-captured games chart is not comparable.
	rss := store.ChartSnapshot{CapturedAt: s3aT0.Add(72 * time.Hour), Storefront: "us", Platform: "iphone", GenreID: "6014",
		Chart: "top-free", Source: store.SourceRSS, RowLimit: 100, Rows: s3aRows(s3aRoblox)}
	if err := store.RecordChart(context.Background(), st, rss); err != nil {
		t.Fatal(err)
	}
	_ = st.Close()
	r := s3aRun(t, "movers", "--db", db, "--genre", "6014", "--json")
	if r.code != 0 || strings.TrimSpace(r.stdout) != "[]" || !strings.Contains(r.stderr, "not comparable") {
		t.Fatalf("incomparable: exit %d stdout %q stderr %q", r.code, r.stdout, r.stderr)
	}
	// Pinning the web source still finds the two comparable captures.
	var rows []moversRow
	s3aDecode(t, s3aRun(t, "movers", "--db", db, "--genre", "6014", "--source", "web", "--json"), &rows)
	if len(rows) != 6 {
		t.Fatalf("pinned web source rows = %d, want 6", len(rows))
	}
}

func TestMoversMinChangeLimitAndSelect(t *testing.T) {
	db := s3aSeededDB(t)
	var rows []moversRow
	s3aDecode(t, s3aRun(t, "movers", "--db", db, "--genre", "games", "--min-change", "2", "--json"), &rows)
	ids := []string{}
	for _, r := range rows {
		ids = append(ids, r.Movement+":"+r.AppID)
	}
	if strings.Join(ids, ",") != "climbed:1621328561,dropped:1094591345,entered:6446308106,exited:431946152" {
		t.Fatalf("--min-change 2 rows = %v", ids)
	}
	rows = nil
	s3aDecode(t, s3aRun(t, "movers", "--db", db, "--genre", "6014", "--limit", "1", "--json"), &rows)
	if len(rows) != 4 || rows[0].AppID != "1621328561" || rows[1].AppID != "1094591345" {
		t.Fatalf("--limit 1 per kind = %+v", rows)
	}
	r := s3aRun(t, "movers", "--db", db, "--genre", "6014", "--json", "--select", "appId,movement")
	var selected []map[string]any
	s3aDecode(t, r, &selected)
	if len(selected) != 6 || len(selected[0]) != 2 {
		t.Fatalf("--select rows = %+v", selected)
	}
}

func TestMoversUsageErrors(t *testing.T) {
	cases := [][]string{
		{"movers", "--chart", "top-weekly"},
		{"movers", "--platform", "watch"},
		{"movers", "--genre", "nope"},
		{"movers", "--source", "ftp"},
		{"movers", "--min-change", "0"},
		{"movers", "--limit", "-1"},
		{"movers", "extra", "--json"},
		{"movers", "--country", "zz"},
	}
	for _, args := range cases {
		if r := s3aRun(t, args...); r.code != 2 {
			t.Errorf("%v: exit %d, want 2 (%v)", args, r.code, r.err)
		}
	}
	r := s3aRun(t, "movers", "--dry-run", "--json")
	var dry map[string]any
	if r.code != 0 || json.Unmarshal([]byte(r.stdout), &dry) != nil || dry["dry_run"] != true {
		t.Fatalf("dry-run: exit %d %q", r.code, r.stdout)
	}
}

func TestComputeMoversPure(t *testing.T) {
	prev := &store.ChartSnapshot{CapturedAt: s3aT0, Rows: s3aRows("a", "b", "c", "d")}
	latest := &store.ChartSnapshot{CapturedAt: s3aT0.Add(time.Hour), Rows: s3aRows("b", "a", "c", "e")}
	rows, counts := s3aComputeMovers(prev, latest, 1, 0)
	if counts != (moversCounts{Climbed: 1, Dropped: 1, Entered: 1, Exited: 1}) || len(rows) != 4 {
		t.Fatalf("counts %+v rows %+v", counts, rows)
	}
	// Identical captures produce no movers at all.
	same, counts := s3aComputeMovers(prev, prev, 1, 0)
	if len(same) != 0 || counts != (moversCounts{}) {
		t.Fatalf("identical captures fabricated movers: %+v", same)
	}
}
