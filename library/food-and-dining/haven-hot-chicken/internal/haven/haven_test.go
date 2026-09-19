package haven

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	_ "modernc.org/sqlite"
	"strings"
	"testing"
	"time"
)

func boolean(b bool) *bool      { return &b }
func number(n float64) *float64 { return &n }
func sample(loc int64) Snapshot {
	return Snapshot{LocationID: loc, FetchedAt: time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC), Complete: true, Items: []Item{{ID: 1, Name: "The Sandwich", PriceCents: 1099, Available: boolean(true)}}}
}
func TestParseMenuMoneyDedupVisibility(t *testing.T) {
	raw := `{"categories":[{"hidden":true,"items":[{"id":1,"name":"Meal","price":10.99,"available_now":true}]},{"items":[{"id":1,"name":"Meal","price":10.99,"available_now":true},{"id":2,"name":"Unknown","price":0}]}]}`
	s, err := ParseMenu([]byte(raw), 7, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Items) != 2 || s.Items[0].PriceCents != 1099 || s.Items[0].Hidden || s.Items[1].Available != nil || !s.Complete {
		t.Fatalf("bad parsed menu: %+v", s)
	}
	for _, price := range []string{`null`, `"1.00"`, `-1`, `1.001`, `92233720368547758.08`} {
		_, err = ParseMenu([]byte(`{"categories":[{"items":[{"id":1,"name":"Meal","price":`+price+`}]}]}`), 7, time.Now())
		if err == nil {
			t.Errorf("accepted invalid price %s", price)
		}
	}
	for _, raw := range []string{`{}`, `{"categories":null}`, `{"categories":[{}]}`, `{"categories":[{"items":null}]}`, `{"categories":[{"items":[{"id":0,"name":"X","price":1}]}]}`} {
		if _, err = ParseMenu([]byte(raw), 7, time.Now()); err == nil {
			t.Errorf("accepted incomplete/invalid: %s", raw)
		}
	}
	_, err = ParseMenu([]byte(`{"categories":[{"items":[{"id":1,"name":"Meal","price":1},{"id":1,"name":"Meal","price":2}]}]}`), 7, time.Now())
	if err == nil {
		t.Fatal("accepted conflicting duplicate")
	}
}
func TestParseLocations(t *testing.T) {
	ls, err := ParseLocations([]byte(`{"locations":[{"id":14208,"name":"North Haven, CT","latitude":41.396959,"longitude":-72.853759}]}`))
	if err != nil || len(ls) != 1 {
		t.Fatalf("parse: %v %+v", err, ls)
	}
	for _, raw := range []string{`{}`, `{"locations":null}`, `{"locations":[{"id":1,"name":"X","latitude":91,"longitude":0}]}`} {
		if _, err = ParseLocations([]byte(raw)); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
}
func TestSubtotalCentsAndRejects(t *testing.T) {
	s := sample(1)
	v, err := Subtotal(s, map[int64]int64{1: 3})
	if err != nil {
		t.Fatal(err)
	}
	if v.(map[string]any)["subtotal_cents"] != int64(3297) {
		t.Fatal(v)
	}
	for _, q := range []int64{0, -1, 10001} {
		if _, err = Subtotal(s, map[int64]int64{1: q}); err == nil {
			t.Fatalf("accepted quantity %d", q)
		}
	}
	if _, err = Subtotal(s, map[int64]int64{2: 1}); err == nil {
		t.Fatal("accepted absent item")
	}
	s.Items[0].PriceCents = math.MaxInt64
	if _, err = Subtotal(s, map[int64]int64{1: 2}); err == nil {
		t.Fatal("accepted overflow")
	}
	s = sample(1)
	for _, state := range []string{"unknown", "hidden", "disabled", "unavailable"} {
		copy := s
		copy.Items = append([]Item{}, s.Items...)
		switch state {
		case "unknown":
			copy.Items[0].Available = nil
		case "hidden":
			copy.Items[0].Hidden = true
		case "disabled":
			copy.Items[0].Disabled = true
		case "unavailable":
			copy.Items[0].Available = boolean(false)
		}
		if _, err = Subtotal(copy, map[int64]int64{1: 1}); err == nil {
			t.Errorf("accepted %s", state)
		}
	}
}
func TestComparePreservesLocationAndVariants(t *testing.T) {
	a, b := sample(1), sample(2)
	b.Items[0].PriceCents = 1299
	b.Items = append(b.Items, Item{ID: 2, Name: "  THE  Sandwich ", PriceCents: 1399, Available: boolean(true)})
	result, err := Compare([]Snapshot{a, b}, "the sandwich")
	if err != nil {
		t.Fatal(err)
	}
	rows := result.(map[string]any)["rows"].([]CompareRow)
	if rows[0].Items[0].PriceCents != 1099 || rows[1].Items[0].PriceCents != 1299 || !rows[1].Ambiguous || len(rows[1].Items) != 2 {
		t.Fatal(rows)
	}
	if _, err = Compare([]Snapshot{a, a}, "x"); err == nil {
		t.Fatal("accepted duplicate locations")
	}
}
func TestCommonUnknownMissingAndAvailability(t *testing.T) {
	a, b := sample(1), sample(2)
	b.Items[0].Available = nil
	v, err := Common([]Snapshot{a, b})
	if err != nil {
		t.Fatal(err)
	}
	m := v.(map[string]any)
	if len(m["rows"].([]CommonRow)) != 0 || m["excluded"].([]CommonRow)[0].Locations[1].Status != "unknown" {
		t.Fatal(v)
	}
	b.Items = []Item{}
	v, _ = Common([]Snapshot{a, b})
	if v.(map[string]any)["excluded"].([]CommonRow)[0].Locations[1].Status != "missing" {
		t.Fatal(v)
	}
	b = sample(2)
	v, _ = Common([]Snapshot{a, b})
	if len(v.(map[string]any)["rows"].([]CommonRow)) != 1 {
		t.Fatal(v)
	}
	b.Items[0].Hidden = true
	v, _ = Common([]Snapshot{a, b})
	if len(v.(map[string]any)["rows"].([]CommonRow)) != 0 {
		t.Fatal(v)
	}
}
func TestChangesRequiresRealCompleteHistory(t *testing.T) {
	a, b := sample(1), sample(1)
	if _, err := Changes(a, b); err == nil {
		t.Fatal("fabricated history accepted")
	}
	b.FetchedAt = b.FetchedAt.Add(time.Hour)
	b.Items = []Item{}
	b.Complete = false
	if _, err := Changes(a, b); err == nil {
		t.Fatal("partial removal accepted")
	}
	b.Complete = true
	b.LocationID = 2
	if _, err := Changes(a, b); err == nil {
		t.Fatal("cross-location diff accepted")
	}
	b.LocationID = 1
	v, err := Changes(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if v.(map[string]any)["rows"].([]Change)[0].Kind != "removed" {
		t.Fatal(v)
	}
}
func TestStoreAtomicityHistoryAndLocationIDs(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	a, b := sample(1), sample(2)
	b.Items[0].PriceCents = 1299
	if err = Save(ctx, db, []Location{{ID: 1, Name: "One"}, {ID: 2, Name: "Two"}}, []Snapshot{a, b}); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TRIGGER reject_two BEFORE INSERT ON haven_snapshots WHEN NEW.location_id=2 BEGIN SELECT RAISE(ABORT,'test rollback'); END`); err != nil {
		t.Fatal(err)
	}
	a.FetchedAt = a.FetchedAt.Add(time.Hour)
	if err = Save(ctx, db, []Location{{ID: 3, Name: "Three"}}, []Snapshot{a, b}); err == nil {
		t.Fatal("trigger should fail")
	}
	ls, err := LoadLocations(ctx, db)
	if err != nil || len(ls) != 2 {
		t.Fatalf("location rollback failed: %+v %v", ls, err)
	}
	first, err := Latest(ctx, db, 1, 2)
	if err != nil || len(first) != 1 {
		t.Fatalf("snapshot rollback failed: %+v %v", first, err)
	}
	other, _ := Latest(ctx, db, 2, 1)
	if other[0].Items[0].PriceCents != 1299 || first[0].Items[0].PriceCents != 1099 {
		t.Fatal("cross-location ids collided")
	}
	if err = Save(ctx, db, nil, []Snapshot{a}); err != nil {
		t.Fatal(err)
	}
	history, _ := Latest(ctx, db, 1, 2)
	if len(history) != 2 || !history[0].FetchedAt.After(history[1].FetchedAt) || history[0].ID == history[1].ID {
		t.Fatal(history)
	}
	partial := sample(4)
	partial.Complete = false
	if err = Save(ctx, db, nil, []Snapshot{partial}); err == nil {
		t.Fatal("saved incomplete observation")
	}
}
func TestNearbyBoundsAndMissingCoordinates(t *testing.T) {
	ls := []Location{{ID: 1, Name: "Here", Latitude: number(41), Longitude: number(-72)}, {ID: 2, Name: "Missing"}, {ID: 3, Name: "Further", Latitude: number(42), Longitude: number(-72)}}
	v, err := Nearby(ls, 41, -72, 2)
	if err != nil {
		t.Fatal(err)
	}
	rows := v.(map[string]any)["rows"].([]NearbyRow)
	if len(rows) != 2 || rows[0].DistanceMiles != 0 || math.Abs(rows[1].DistanceMiles-69.0934) > .01 {
		t.Fatal(v)
	}
	for _, coords := range [][2]float64{{91, 0}, {0, -181}, {math.NaN(), 0}, {0, math.Inf(1)}} {
		if _, err = Nearby(ls, coords[0], coords[1], 2); err == nil {
			t.Fatal("invalid coordinates accepted")
		}
	}
	if _, err = Nearby(ls, 41, -72, 0); err == nil {
		t.Fatal("invalid limit accepted")
	}
}
func TestEmptyOutputArrays(t *testing.T) {
	s := sample(1)
	s.Items = []Item{}
	v, err := Compare([]Snapshot{s}, "missing")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(v)
	if strings.Contains(string(raw), `"items":null`) {
		t.Fatal(string(raw))
	}
	v, _ = Nearby(nil, 41, -72, 2)
	raw, _ = json.Marshal(v)
	if strings.Contains(string(raw), `"rows":null`) {
		t.Fatal(string(raw))
	}
}
