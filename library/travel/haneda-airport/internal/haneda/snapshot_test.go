package haneda

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func testSnapshot(t *testing.T) Snapshot {
	t.Helper()
	var b RawBoard
	if err := json.Unmarshal(fixture(t, "international-departures"), &b); err != nil {
		t.Fatal(err)
	}
	f, err := normalizeFlight(b.Flights[0], nil, nil, parseTimestamp(b.Date.Date))
	if err != nil {
		t.Fatal(err)
	}
	return Snapshot{Schema: SnapshotSchema, SavedAt: "2026-10-02T23:49:41+09:00", Board: BoardResult{ObservedAt: "2026-10-02T23:49:40+09:00", Coverage: Coverage{Kind: "international", Direction: "departure", RequestedDate: "2026-10-02", Origin: Origin}, Sources: []SourceInfo{{Kind: "international", Direction: "departure", SourceTotal: 1}}, Flights: []Flight{f}, ScannedRecords: 1, Complete: true, Notes: boardNotes()}}
}
func cloneSnapshot(s Snapshot) Snapshot {
	b, _ := json.Marshal(s)
	var out Snapshot
	_ = json.Unmarshal(b, &out)
	return out
}

func TestSnapshotAtomicRoundtripAndOverwriteGuard(t *testing.T) {
	s := testSnapshot(t)
	path := filepath.Join(t.TempDir(), "board.json")
	if err := SaveSnapshot(path, s, false); err != nil {
		t.Fatal(err)
	}
	read, err := LoadSnapshot(path)
	if err != nil || read.Board.Flights[0].ID != s.Board.Flights[0].ID {
		t.Fatalf("roundtrip: %v", err)
	}
	changed := cloneSnapshot(s)
	changed.Board.Flights[0].Terminal = ptr("T3")
	if err := SaveSnapshot(path, changed, false); err == nil {
		t.Fatal("existing snapshot was overwritten without opt-in")
	}
	unchanged, _ := LoadSnapshot(path)
	if *unchanged.Board.Flights[0].Terminal != *s.Board.Flights[0].Terminal {
		t.Fatal("failed exclusive write changed the old file")
	}
	if err := SaveSnapshot(path, changed, true); err != nil {
		t.Fatal(err)
	}
	updated, _ := LoadSnapshot(path)
	if *updated.Board.Flights[0].Terminal != "T3" {
		t.Fatal("explicit atomic replacement failed")
	}
	partial := cloneSnapshot(s)
	partial.Board.Sources[0].SourceTotal = 2
	if err := SaveSnapshot(filepath.Join(t.TempDir(), "partial.json"), partial, false); err == nil {
		t.Fatal("a sliced board must not claim a complete snapshot")
	}
}
func TestLoadSnapshotRejectsMalformedOrIncomplete(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Snapshot)
	}{
		{"schema", func(s *Snapshot) { s.Schema = "unknown" }},
		{"incomplete", func(s *Snapshot) { s.Board.Complete = false }},
		{"timestamp", func(s *Snapshot) { s.Board.ObservedAt = "unknown" }},
		{"wrong timezone", func(s *Snapshot) { s.Board.ObservedAt = "2026-10-02T14:49:40Z" }},
		{"identity", func(s *Snapshot) { s.Board.Flights[0].ID = "wrong" }},
		{"coverage", func(s *Snapshot) { s.Board.Coverage.Direction = "arrival" }},
		{"missing declared scopes", func(s *Snapshot) { s.Board.Coverage.Kind, s.Board.Coverage.Direction = "all", "both" }},
		{"duplicate source", func(s *Snapshot) { s.Board.Sources = append(s.Board.Sources, s.Board.Sources[0]) }},
		{"wrong per-source count", func(s *Snapshot) {
			s.Board.Coverage.Direction = "both"
			s.Board.Sources[0].SourceTotal = 0
			s.Board.Sources = append(s.Board.Sources, SourceInfo{Kind: "international", Direction: "arrival", SourceTotal: 1})
		}},
		{"malformed clock", func(s *Snapshot) { s.Board.Flights[0].ScheduledAt = ptr("x") }},
		{"empty flight list", func(s *Snapshot) { s.Board.Flights[0].ListedFlights = []ListedFlight{} }},
		{"contradictory status", func(s *Snapshot) { s.Board.Flights[0].Status.Known = !s.Board.Flights[0].Status.Known }},
		{"invented actual", func(s *Snapshot) { s.Board.Flights[0].ActualAt = s.Board.Flights[0].ScheduledAt }},
		{"lookup scope", func(s *Snapshot) { s.Board.Coverage.QueryMode = "exact_primary_lookup" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testSnapshot(t)
			tc.mutate(&s)
			b, _ := json.Marshal(s)
			path := filepath.Join(t.TempDir(), "bad.json")
			if err := os.WriteFile(path, b, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadSnapshot(path); err == nil {
				t.Fatal("invalid observation accepted")
			}
		})
	}
}
func TestDiffSnapshotsAbsenceChangesAndCoverage(t *testing.T) {
	before := testSnapshot(t)
	for _, tc := range []struct {
		name   string
		mutate func(*Snapshot)
		want   string
		count  int
	}{{"unchanged", func(s *Snapshot) {}, "", 0}, {"legacy v1 mode", func(s *Snapshot) { s.Board.Coverage.QueryMode = "board" }, "", 0}, {"gate", func(s *Snapshot) { s.Board.Flights[0].BoardingGates = []string{"999"} }, "changed", 1}, {"schedule", func(s *Snapshot) { s.Board.Flights[0].ScheduledAt = ptr("2026-10-02T00:10:00+09:00") }, "changed", 1}, {"gone", func(s *Snapshot) {
		s.Board.Flights = []Flight{}
		s.Board.ScannedRecords = 0
		s.Board.Sources[0].SourceTotal = 0
	}, "no_longer_reported", 1}} {
		t.Run(tc.name, func(t *testing.T) {
			after := cloneSnapshot(before)
			after.Board.ObservedAt = "2026-10-02T23:50:40+09:00"
			tc.mutate(&after)
			r, err := DiffSnapshots(before, after)
			if err != nil || len(r.Changes) != tc.count || r.Changes == nil {
				t.Fatalf("diff: %+v,%v", r, err)
			}
			if tc.count > 0 && r.Changes[0].Type != tc.want {
				t.Fatal("disappearance must never be called a cancellation")
			}
		})
	}
	other := cloneSnapshot(before)
	other.Board.Coverage.RequestedDate = "2026-10-03"
	if _, err := DiffSnapshots(before, other); err == nil {
		t.Fatal("incompatible service days must not produce a cancellation diff")
	}
	earlier := cloneSnapshot(before)
	earlier.Board.ObservedAt = "2026-10-02T23:48:00+09:00"
	if _, err := DiffSnapshots(before, earlier); err == nil {
		t.Fatal("reversed observation order accepted")
	}
}

func TestSnapshotQueryDistinguishesUncoveredScopesFromEmptyMatches(t *testing.T) {
	s := testSnapshot(t)
	for _, tc := range []struct {
		name, kind, direction, date string
		covered                     bool
	}{
		{"saved scope", "international", "departure", "2026-10-02", true},
		{"other kind", "domestic", "departure", "2026-10-02", false},
		{"broader kind", "all", "departure", "2026-10-02", false},
		{"other direction", "international", "arrival", "2026-10-02", false},
		{"broader direction", "international", "both", "2026-10-02", false},
		{"other request date", "international", "departure", "2026-10-03", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSnapshotQuery(s, Query{Kind: tc.kind, Direction: tc.direction, Date: tc.date})
			if (err == nil) != tc.covered {
				t.Fatalf("covered=%t: %v", tc.covered, err)
			}
		})
	}
	s.Board.Coverage.Kind, s.Board.Coverage.Direction = "all", "both"
	if err := ValidateSnapshotQuery(s, Query{Kind: "domestic", Direction: "arrival", Date: "2026-10-02"}); err != nil {
		t.Fatalf("a saved superset covers the narrowed scope: %v", err)
	}
}

func TestDiffSnapshotProviderFacilityAndMapChanges(t *testing.T) {
	before := testSnapshot(t)
	before.Board.Flights[0].Facilities = []Facility{{Type: "gate", Title: "Published gate", Name: "144", MapURL: ptr(Origin + "/map-before")}}
	for _, tc := range []struct {
		name   string
		change func(*Facility)
	}{
		{"facility name", func(f *Facility) { f.Name = "145" }},
		{"map handoff", func(f *Facility) { f.MapURL = ptr(Origin + "/map-after") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			after := cloneSnapshot(before)
			tc.change(&after.Board.Flights[0].Facilities[0])
			r, err := DiffSnapshots(before, after)
			if err != nil || len(r.Changes) != 1 || len(r.Changes[0].Fields) != 1 || r.Changes[0].Fields[0].Field != "facilities" {
				t.Fatalf("provider facility change without separate gate/counter change: %+v, %v", r, err)
			}
		})
	}
}
