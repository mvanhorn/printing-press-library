package cycling

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func ptr[T any](x T) *T { return &x }

func TestChangesRetainUsabilityTransitions(t *testing.T) {
	now := time.Unix(1790952697, 0)
	before := sample(now)
	old := before.Stations(before.ObservedAt, 5*time.Minute, "2")
	stale := before.Stations(now.Add(10*time.Minute), 5*time.Minute, "2")
	changes, n := Changes(old, stale, "5112", 10)
	if n != 1 || changes[0].Before.RentalState != "available" || changes[0].After.RentalState != "stale" {
		t.Fatalf("timestamp-only stale transition missing: %#v", changes)
	}
	unknown := sample(now)
	unknown.Statuses[0].Docks = ptr(0)
	unknown.Statuses[0].VehicleDocks[0].Count = ptr(0)
	compatible := unknown.Stations(now, 5*time.Minute, "2")
	unknown.Statuses[0].VehicleDocks[0].IDs = []string{"3"}
	incompatible := unknown.Stations(now, 5*time.Minute, "2")
	changes, n = Changes(compatible, incompatible, "5112", 10)
	if n != 1 || changes[0].Before.ReturnState != "full" || *changes[0].Before.SelectedSpaces != 0 || *changes[0].After.SelectedSpaces != 0 || changes[0].After.ReturnState != "incompatible" {
		t.Fatalf("compatibility-only transition missing: %#v", changes)
	}
	heartbeat := sample(now)
	heartbeat.Statuses[0].Reported++
	_, n = Changes(old, heartbeat.Stations(now, 5*time.Minute, "2"), "5112", 10)
	if n != 0 {
		t.Fatal("routine fresh heartbeat should not masquerade as an inventory/usability change")
	}
}
func sample(now time.Time) Snapshot {
	return Snapshot{ObservedAt: now, Feeds: map[string]FeedMeta{"station_status": {LastUpdated: now.Unix()}}, Information: []Info{{ID: "5112", Name: "プラーズタワー東新宿", Address: "東京都新宿区歌舞伎町", Lat: ptr(35.697315), Lon: ptr(139.704995), Capacity: json.RawMessage(`"13"`)}, {ID: "17", Name: "新御徒町ステーション", Lat: ptr(35.707252), Lon: ptr(139.777587), Capacity: json.RawMessage(`8`)}}, Statuses: []Status{{ID: "5112", Reported: now.Unix() - 10, Bikes: ptr(1), Docks: ptr(11), Installed: ptr(true), Renting: ptr(true), Returning: ptr(true), Vehicles: []TypeCount{{"2", ptr(1)}}, VehicleDocks: []DockCount{{[]string{"2"}, ptr(11)}}}, {ID: "17", Reported: now.Unix() - 20, Bikes: ptr(6), Docks: ptr(2), Installed: ptr(true), Renting: ptr(true), Returning: ptr(true), Vehicles: []TypeCount{{"2", ptr(6)}}, VehicleDocks: []DockCount{{[]string{"2"}, ptr(2)}}}}, Vehicles: []Vehicle{{ID: "2", Propulsion: "electric_assist"}}, Warnings: []string{}, Requests: 4, Bytes: 1234}
}
func TestStationStates(t *testing.T) {
	now := time.Unix(1790952697, 0)
	for _, tc := range []struct {
		name      string
		edit      func(*Snapshot)
		rent, ret string
	}{
		{"available", func(s *Snapshot) {}, "available", "available"},
		{"empty", func(s *Snapshot) { s.Statuses[0].Bikes = ptr(0); s.Statuses[0].Vehicles[0].Count = ptr(0) }, "empty", "available"},
		{"full", func(s *Snapshot) { s.Statuses[0].Docks = ptr(0); s.Statuses[0].VehicleDocks[0].Count = ptr(0) }, "available", "full"},
		{"rent closed", func(s *Snapshot) { s.Statuses[0].Renting = ptr(false) }, "closed", "available"},
		{"return closed", func(s *Snapshot) { s.Statuses[0].Returning = ptr(false) }, "available", "closed"},
		{"uninstalled", func(s *Snapshot) { s.Statuses[0].Installed = ptr(false) }, "uninstalled", "uninstalled"},
		{"missing row", func(s *Snapshot) { s.Statuses = s.Statuses[1:] }, "source_missing", "source_missing"},
		{"unknown flag", func(s *Snapshot) { s.Statuses[0].Renting = nil }, "unknown", "available"},
		{"unknown inventory", func(s *Snapshot) { s.Statuses[0].Vehicles = nil; s.Statuses[0].VehicleDocks = nil }, "unknown", "unknown"},
		{"negative count", func(s *Snapshot) { s.Statuses[0].Vehicles[0].Count = ptr(-1) }, "unknown", "available"},
		{"stale row", func(s *Snapshot) { s.Statuses[0].Reported = now.Add(-10 * time.Minute).Unix() }, "stale", "stale"},
		{"stale feed", func(s *Snapshot) {
			s.Feeds["station_status"] = FeedMeta{LastUpdated: now.Add(-10 * time.Minute).Unix()}
		}, "stale", "stale"},
		{"future timestamp", func(s *Snapshot) { s.Statuses[0].Reported = now.Add(2 * time.Minute).Unix() }, "stale", "stale"},
		{"incompatible dock", func(s *Snapshot) { s.Statuses[0].VehicleDocks[0].IDs = []string{"3"} }, "available", "incompatible"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sample(now)
			tc.edit(&s)
			r := s.Stations(now, 5*time.Minute, "2")[0]
			if r.RentalState != tc.rent || r.ReturnState != tc.ret {
				t.Fatalf("got %s/%s want %s/%s", r.RentalState, r.ReturnState, tc.rent, tc.ret)
			}
			if *r.Capacity != 13 {
				t.Fatal("string source capacity lost")
			}
		})
	}
}
func TestNearbyAndFind(t *testing.T) {
	now := time.Unix(1790952697, 0)
	s := sample(now)
	rows := s.Stations(now, 5*time.Minute, "2")
	for _, tc := range []struct {
		q     string
		count int
	}{{"東新宿", 1}, {"東京都新宿", 1}, {"５１１２", 1}, {"__definitely_absent__", 0}} {
		r, n := Find(rows, tc.q, 5)
		if n != tc.count || len(r) != tc.count {
			t.Errorf("%s = %d,%d", tc.q, n, len(r))
		}
		if r == nil {
			t.Fatal("empty rows must encode []")
		}
	}
	r, n := Nearby(rows, 35.697315, 139.704995, 1000, "pickup", true, 5)
	if n != 1 || r[0].ID != "5112" || *r[0].Distance != 0 {
		t.Fatalf("nearby = %#v", r)
	}
	s.Statuses[0].Renting = ptr(false)
	rows = s.Stations(now, 5*time.Minute, "2")
	r, _ = Nearby(rows, 35.697315, 139.704995, 1000, "pickup", true, 5)
	if len(r) != 0 {
		t.Fatal("closed pickup selected")
	}
	r, _ = Nearby(rows, 35.697315, 139.704995, 1000, "return", true, 5)
	if len(r) != 1 {
		t.Fatal("direction-specific closure lost")
	}
	for _, tc := range []struct {
		lat, lon float64
		ok       bool
	}{{35, 139, true}, {-90, -180, true}, {91, 0, false}, {0, 181, false}} {
		if ValidCoordinates(tc.lat, tc.lon) != tc.ok {
			t.Fatal(tc)
		}
	}
	for _, tc := range []struct {
		n  int
		ok bool
	}{{1, true}, {50, true}, {0, false}, {51, false}} {
		if (ValidateLimit(tc.n, 50) == nil) != tc.ok {
			t.Fatal(tc)
		}
	}
}
func TestCompareCompatibilityAndStaleness(t *testing.T) {
	now := time.Unix(1790952697, 0)
	for _, tc := range []struct {
		name  string
		edit  func(*Snapshot)
		count int
	}{
		{"compatible", func(s *Snapshot) {}, 1},
		{"incompatible return", func(s *Snapshot) { s.Statuses[1].VehicleDocks[0].IDs = []string{"3"} }, 0},
		{"stale pickup", func(s *Snapshot) { s.Statuses[0].Reported = now.Add(-6 * time.Minute).Unix() }, 0},
		{"closed return", func(s *Snapshot) { s.Statuses[1].Returning = ptr(false) }, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sample(now)
			tc.edit(&s)
			pairs := Compare(s.Stations(now, 5*time.Minute, "2"), 35.697315, 139.704995, 35.707252, 139.777587, 1000, 5)
			if len(pairs) != tc.count {
				t.Fatalf("got %d pairs", len(pairs))
			}
			if len(pairs) > 0 && pairs[0].AccessDistance != 0 {
				t.Fatal("endpoint access distance incorrect")
			}
		})
	}
}
func TestChangesAndOfflineTimestamps(t *testing.T) {
	now := time.Unix(1790952697, 0)
	for _, name := range []string{"snapshot.db", "snapshot ?#.db"} {
		t.Run(name, func(t *testing.T) {
			s := sample(now)
			path := filepath.Join(t.TempDir(), name)
			if e := SaveSnapshot(path, s); e != nil {
				t.Fatal(e)
			}
			loaded, e := LoadSnapshot(path)
			if e != nil {
				t.Fatal(e)
			}
			if !loaded.ObservedAt.Equal(now) {
				t.Fatal("offline observation time refreshed")
			}
			meta := loaded.Meta("local", now.Add(time.Hour))
			if meta.Requests != 0 || meta.Bytes != 0 || meta.Source != "local" {
				t.Fatal(meta)
			}
			if loaded.Stations(now.Add(time.Hour), 5*time.Minute, "2")[0].RentalState != "stale" {
				t.Fatal("old offline counts considered available")
			}
			after := sample(now)
			after.Statuses[0].Bikes = ptr(2)
			r, n := Changes(s.Stations(now, 5*time.Minute, ""), after.Stations(now, 5*time.Minute, ""), "新宿", 5)
			if n != 1 || r[0].Kind != "changed" || *r[0].Before.Bikes != 1 || *r[0].After.Bikes != 2 {
				t.Fatal(r)
			}
			after.Information = after.Information[1:]
			r, n = Changes(s.Stations(now, 5*time.Minute, ""), after.Stations(now, 5*time.Minute, ""), "5112", 5)
			if n != 1 || r[0].Kind != "source_missing_after" {
				t.Fatal(r)
			}
		})
	}
	path := filepath.Join(t.TempDir(), "missing.db")
	if _, e := LoadSnapshot(path); e == nil {
		t.Fatal("missing snapshot accepted")
	}
	if _, e := os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("read created cache")
	}
}

func TestCompareOnePairAtOverlappingEndpoints(t *testing.T) {
	now := time.Unix(1790952697, 0)
	s := sample(now)
	for _, limit := range []int{1, 2} {
		pairs := Compare(s.Stations(now, 5*time.Minute, "2"), 35.697315, 139.704995, 35.697315, 139.704995, 10000, limit)
		if len(pairs) != limit {
			t.Fatalf("limit=%d got %d despite compatible distinct stations", limit, len(pairs))
		}
		for _, p := range pairs {
			if p.Pickup.ID == p.Dropoff.ID {
				t.Fatal("same-station pair")
			}
		}
	}
}

func TestRelativeSnapshotPath(t *testing.T) {
	t.Chdir(t.TempDir())
	s := sample(time.Now())
	if e := SaveSnapshot("nested/snapshot.db", s); e != nil {
		t.Fatal(e)
	}
	loaded, e := LoadSnapshot("nested/snapshot.db")
	if e != nil || len(loaded.Information) != 2 {
		t.Fatal(e)
	}
}
