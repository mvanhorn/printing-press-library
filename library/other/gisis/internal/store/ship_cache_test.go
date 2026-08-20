// Hand-authored — NOT generated. Tests for the GISIS ship cache/watchlist layer.
package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func upsertTestShip(t *testing.T, s *Store, fields map[string]any) {
	t.Helper()
	imo, _ := fields["imo_number"].(string)
	data, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := s.UpsertShipByIMO(imo, data); err != nil {
		t.Fatalf("UpsertShipByIMO(%s): %v", imo, err)
	}
}

// The generated UpsertShip keys by extractObjectID, which for a GISIS payload
// (no "id") falls through to the ship name. Vessels rename, so the cache must
// key by IMO: a rename must update the existing row, not create a second one.
func TestUpsertShipByIMO_KeysByIMONotName(t *testing.T) {
	s := newTestStore(t)
	upsertTestShip(t, s, map[string]any{"imo_number": "9866641", "name": "SIDER ABIDJAN", "flag": "Comoros"})
	upsertTestShip(t, s, map[string]any{"imo_number": "9866641", "name": "RENAMED VESSEL", "flag": "Panama"})

	if n, err := s.Count("ship"); err != nil || n != 1 {
		t.Fatalf("Count after rename = %d (err %v), want 1", n, err)
	}
	rows, err := s.ListShips(ListShipsOptions{})
	if err != nil {
		t.Fatalf("ListShips: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].IMONumber != "9866641" || rows[0].Name != "RENAMED VESSEL" || rows[0].Flag != "Panama" {
		t.Fatalf("row not updated in place: %+v", rows[0])
	}
	if _, err := s.Get("ship", "9866641"); err != nil {
		t.Fatalf("generic resources row should be keyed by IMO: %v", err)
	}
}

func TestUpsertShipByIMO_DerivesYearBuilt(t *testing.T) {
	s := newTestStore(t)
	upsertTestShip(t, s, map[string]any{"imo_number": "111", "name": "A", "date_of_build": "2019-05-01"})
	upsertTestShip(t, s, map[string]any{"imo_number": "222", "name": "B", "date_of_build": "unknown"})

	rows, err := s.ListShips(ListShipsOptions{})
	if err != nil {
		t.Fatalf("ListShips: %v", err)
	}
	byIMO := map[string]ShipRow{}
	for _, r := range rows {
		byIMO[r.IMONumber] = r
	}
	if byIMO["111"].YearBuilt != 2019 {
		t.Fatalf("year_built for 111 = %d, want 2019", byIMO["111"].YearBuilt)
	}
	if byIMO["222"].YearBuilt != 0 {
		t.Fatalf("year_built for 222 = %d, want 0 (no parseable year)", byIMO["222"].YearBuilt)
	}
}

func TestListShips_Filters(t *testing.T) {
	s := newTestStore(t)
	upsertTestShip(t, s, map[string]any{"imo_number": "1", "name": "SIDER ABIDJAN", "flag": "Panama", "ship_type": "Oil Tanker", "registered_owner": "KIVIK SHIPPING LTD"})
	upsertTestShip(t, s, map[string]any{"imo_number": "2", "name": "BLUE STAR", "flag": "Liberia", "ship_type": "Bulk Carrier", "registered_owner": "ACME MARINE"})
	upsertTestShip(t, s, map[string]any{"imo_number": "3", "name": "SIDER LAGOS", "flag": "Panama", "ship_type": "Oil Tanker", "registered_owner": "KIVIK HOLDINGS"})

	cases := []struct {
		name string
		opts ListShipsOptions
		want []string
	}{
		{"flag", ListShipsOptions{Flag: "panama"}, []string{"1", "3"}},
		{"type", ListShipsOptions{ShipType: "Bulk Carrier"}, []string{"2"}},
		{"owner-substring", ListShipsOptions{Owner: "kivik"}, []string{"1", "3"}},
		{"name-like", ListShipsOptions{NameLike: "sider"}, []string{"1", "3"}},
		{"name-like-matches-owner", ListShipsOptions{NameLike: "acme"}, []string{"2"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := s.ListShips(tc.opts)
			if err != nil {
				t.Fatalf("ListShips: %v", err)
			}
			got := map[string]bool{}
			for _, r := range rows {
				got[r.IMONumber] = true
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d rows %v, want %v", len(got), got, tc.want)
			}
			for _, w := range tc.want {
				if !got[w] {
					t.Fatalf("missing IMO %s in %v", w, got)
				}
			}
		})
	}
}

func TestListShips_PinnedOnly(t *testing.T) {
	s := newTestStore(t)
	upsertTestShip(t, s, map[string]any{"imo_number": "1", "name": "A"})
	upsertTestShip(t, s, map[string]any{"imo_number": "2", "name": "B"})
	if err := s.PinShip("2", "watch"); err != nil {
		t.Fatalf("PinShip: %v", err)
	}

	rows, err := s.ListShips(ListShipsOptions{PinnedOnly: true})
	if err != nil {
		t.Fatalf("ListShips: %v", err)
	}
	if len(rows) != 1 || rows[0].IMONumber != "2" || !rows[0].Pinned || rows[0].PinLabel != "watch" {
		t.Fatalf("pinned-only result wrong: %+v", rows)
	}
}

func TestOwnerFleet_ExactVsLike(t *testing.T) {
	s := newTestStore(t)
	upsertTestShip(t, s, map[string]any{"imo_number": "1", "name": "A", "registered_owner": "KIVIK SHIPPING LTD"})
	upsertTestShip(t, s, map[string]any{"imo_number": "2", "name": "B", "registered_owner": "KIVIK HOLDINGS"})

	exact, err := s.OwnerFleet("KIVIK SHIPPING LTD", false)
	if err != nil {
		t.Fatalf("OwnerFleet exact: %v", err)
	}
	if len(exact) != 1 || exact[0].IMONumber != "1" {
		t.Fatalf("exact match wrong: %+v", exact)
	}
	like, err := s.OwnerFleet("kivik", true)
	if err != nil {
		t.Fatalf("OwnerFleet like: %v", err)
	}
	if len(like) != 2 {
		t.Fatalf("like match got %d, want 2", len(like))
	}
}

func TestStaleShips(t *testing.T) {
	s := newTestStore(t)
	upsertTestShip(t, s, map[string]any{"imo_number": "old", "name": "OLD"})
	upsertTestShip(t, s, map[string]any{"imo_number": "fresh", "name": "FRESH"})
	// Backdate the "old" row's sync time.
	if _, err := s.DB().Exec(`UPDATE "ship" SET synced_at=? WHERE id=?`, "2020-01-01T00:00:00Z", "old"); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	rows, err := s.StaleShips(cutoff, false)
	if err != nil {
		t.Fatalf("StaleShips: %v", err)
	}
	if len(rows) != 1 || rows[0].IMONumber != "old" {
		t.Fatalf("stale result wrong: %+v", rows)
	}

	// pinnedOnly excludes the unpinned stale row.
	pinnedStale, err := s.StaleShips(cutoff, true)
	if err != nil {
		t.Fatalf("StaleShips pinned: %v", err)
	}
	if len(pinnedStale) != 0 {
		t.Fatalf("pinned-only stale got %d, want 0", len(pinnedStale))
	}
}

func TestPins_Lifecycle(t *testing.T) {
	s := newTestStore(t)
	if err := s.PinShip("9866641", "Lagos deal"); err != nil {
		t.Fatalf("PinShip: %v", err)
	}
	// Re-pin updates the label rather than duplicating.
	if err := s.PinShip("9866641", "Lagos deal v2"); err != nil {
		t.Fatalf("re-PinShip: %v", err)
	}
	pins, err := s.ListPins()
	if err != nil {
		t.Fatalf("ListPins: %v", err)
	}
	if len(pins) != 1 || pins[0].Label != "Lagos deal v2" {
		t.Fatalf("pins wrong: %+v", pins)
	}
	imos, err := s.PinnedIMOs()
	if err != nil {
		t.Fatalf("PinnedIMOs: %v", err)
	}
	if len(imos) != 1 || imos[0] != "9866641" {
		t.Fatalf("PinnedIMOs wrong: %v", imos)
	}

	removed, err := s.UnpinShip("9866641")
	if err != nil || !removed {
		t.Fatalf("UnpinShip = %v, %v; want true, nil", removed, err)
	}
	removed, err = s.UnpinShip("9866641")
	if err != nil || removed {
		t.Fatalf("second UnpinShip = %v, %v; want false, nil", removed, err)
	}
}
