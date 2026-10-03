package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func snapshot(id, projection, at string, peak int) json.RawMessage {
	b, _ := json.Marshal(map[string]any{"id": id, "projection": projection, "observed_at": at, "peak_m": peak})
	return b
}

func TestSnowJapanCatalogReplacementIsAtomic(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "facts.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	ctx := context.Background()
	cases := []struct {
		name  string
		facts []json.RawMessage
		fail  bool
		want  int
	}{
		{"first", []json.RawMessage{snapshot("a", "catalog-v1", "one", 1)}, false, 1},
		{"replacement", []json.RawMessage{snapshot("b", "catalog-v1", "two", 2)}, false, 1},
		{"invalid projection rolls back", []json.RawMessage{snapshot("c", "detail-v1", "three", 3)}, true, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := s.CaptureSnowJapan(ctx, tc.facts, true)
			if (e != nil) != tc.fail {
				t.Fatalf("error=%v", e)
			}
			v, complete, e := s.SnowJapanCatalog(ctx)
			if e != nil || !complete || len(v) != tc.want {
				t.Fatalf("catalog=%s complete=%v error=%v", v, complete, e)
			}
		})
	}
	v, _, _ := s.SnowJapanCatalog(ctx)
	var f map[string]any
	_ = json.Unmarshal(v[0], &f)
	if f["id"] != "b" {
		t.Fatal("failed capture replaced directory")
	}
}

func TestSnowJapanSnapshotsRetainTwoCompatibleObservations(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "facts.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	ctx := context.Background()
	for _, at := range []string{"one", "two", "three"} {
		if e = s.CaptureSnowJapan(ctx, []json.RawMessage{snapshot("a", "catalog-v1", at, 1)}, true); e != nil {
			t.Fatal(e)
		}
	}
	if e = s.CaptureSnowJapan(ctx, []json.RawMessage{snapshot("a", "detail-v1", "detail-only", 2)}, false); e != nil {
		t.Fatal(e)
	}
	pair, e := s.SnowJapanSnapshotPair(ctx, "a")
	if e != nil || len(pair) != 2 {
		t.Fatalf("pair=%s e=%v", pair, e)
	}
	var latest map[string]any
	_ = json.Unmarshal(pair[0], &latest)
	if latest["observed_at"] != "three" || latest["projection"] != "catalog-v1" {
		t.Fatal("incompatible single detail used as baseline")
	}
	v, e := s.SnowJapanSnapshotPair(ctx, "missing")
	if e != nil || len(v) != 0 {
		t.Fatal("missing baseline fabricated")
	}
}

func TestSnowJapanReadMethodsHandleMissingTables(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "facts.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	ctx := context.Background()
	v, complete, e := s.SnowJapanCatalog(ctx)
	if e != nil || complete || len(v) != 0 {
		t.Fatal("missing catalog not empty")
	}
	pair, e := s.SnowJapanSnapshotPair(ctx, "a")
	if e != nil || len(pair) != 0 {
		t.Fatal("missing snapshot not empty")
	}
	season, captured, _, e := s.SnowJapanSeason(ctx, "2025-2026")
	if e != nil || captured || len(season) != 0 {
		t.Fatal("missing season capture not empty")
	}
}

func TestSnowJapanSeasonCaptureIsScopedAndAtomic(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "facts.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	ctx := context.Background()
	fact := func(season, id string) json.RawMessage {
		b, _ := json.Marshal(map[string]any{"season": season, "id": id, "observed_at": "2026-10-03T00:00:00Z"})
		return b
	}
	for _, season := range []string{"2024-2025", "2025-2026"} {
		if e = s.CaptureSnowJapanSeason(ctx, season, []json.RawMessage{fact(season, "old")}); e != nil {
			t.Fatal(e)
		}
	}
	if e = s.CaptureSnowJapanSeason(ctx, "2025-2026", []json.RawMessage{fact("2025-2026", "new")}); e != nil {
		t.Fatal(e)
	}
	if e = s.CaptureSnowJapanSeason(ctx, "2025-2026", []json.RawMessage{fact("2024-2025", "wrong")}); e == nil {
		t.Fatal("mismatched winter replaced a complete capture")
	}
	for season, want := range map[string]string{"2024-2025": "old", "2025-2026": "new"} {
		rows, captured, at, e := s.SnowJapanSeason(ctx, season)
		if e != nil || !captured || len(rows) != 1 || at == "" {
			t.Fatalf("capture=%s complete=%v observed=%s error=%v", rows, captured, at, e)
		}
		var v map[string]any
		_ = json.Unmarshal(rows[0], &v)
		if v["id"] != want {
			t.Fatalf("season %s got %v want %s", season, v, want)
		}
	}
	_, captured, _, e := s.SnowJapanSeason(ctx, "2023-2024")
	if e != nil || captured {
		t.Fatal("another winter's capture was treated as requested evidence")
	}
}

func TestSnowJapanChangesUseMostRecentCompatibleProjection(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "facts.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	observations := []struct {
		projection, at string
		peak           int
	}{
		{"catalog-v1", "catalog-first", 100},
		{"detail-v1", "detail-first", 150},
		{"detail-v1", "detail-second", 150},
		{"catalog-v1", "catalog-latest", 200},
		{"detail-v1", "detail-latest", 250},
	}
	for i, observation := range observations {
		raw := snapshot("a", observation.projection, observation.at, observation.peak)
		if err := s.CaptureSnowJapan(ctx, []json.RawMessage{raw}, observation.projection == "catalog-v1"); err != nil {
			t.Fatal(err)
		}
		if i < 3 {
			continue
		}
		pair, err := s.SnowJapanSnapshotPair(ctx, "a")
		if err != nil || len(pair) != 2 {
			t.Fatalf("pair=%s error=%v", pair, err)
		}
		var latest, baseline map[string]any
		if err := json.Unmarshal(pair[0], &latest); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(pair[1], &baseline); err != nil {
			t.Fatal(err)
		}
		if latest["projection"] != observation.projection || latest["peak_m"] != float64(observation.peak) || baseline["projection"] != observation.projection {
			t.Fatalf("newer saved facts hidden or incompatible baseline: %s", pair)
		}
	}
}
