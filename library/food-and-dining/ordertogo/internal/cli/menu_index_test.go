// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/ordertogo/internal/store"
)

const sampleMenu = `{"meta":{"source":"live"},"results":[
	{"id":19001,"item_id":"Nigiri Two Pieces04","name":"Salmon","price":4.99},
	{"id":19031,"item_id":"Roll08","name":"Salmon and Avocado Roll 8PC","price":12.99},
	{"id":0,"item_id":"bad","name":"Zero","price":1.00}
]}`

func buildIndex(t *testing.T, raw string) *menuIndex {
	t.Helper()
	results, err := menuResults([]byte(raw))
	if err != nil {
		t.Fatalf("menuResults: %v", err)
	}
	idx := &menuIndex{byID: map[string]menuEntry{}, byName: map[string]menuEntry{}}
	for _, r := range results {
		id := r.ID.String()
		if id == "" || id == "0" {
			continue
		}
		e := menuEntry{id: id, price: r.Price, name: r.Name}
		idx.byID[id] = e
		if r.Name != "" {
			if _, ok := idx.byName[lower(r.Name)]; !ok {
				idx.byName[lower(r.Name)] = e
			}
		}
	}
	return idx
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

func TestMenuResults_Envelope(t *testing.T) {
	got, err := menuResults([]byte(sampleMenu))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 results, got %d", len(got))
	}
}

func TestMenuResults_BareArray(t *testing.T) {
	bare := `[{"id":19001,"name":"Salmon","price":4.99}]`
	got, err := menuResults([]byte(bare))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Salmon" {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestMenuResults_Empty(t *testing.T) {
	if _, err := menuResults([]byte(`{"meta":{},"results":[]}`)); err == nil {
		t.Fatal("expected error for empty menu")
	}
}

func TestResolveCartItems_ByID(t *testing.T) {
	idx := buildIndex(t, sampleMenu)
	items := []store.OrderItem{{ItemID: "19001", ID: "19001", Quantity: 1}}
	if err := resolveCartItems(idx, items); err != nil {
		t.Fatalf("err: %v", err)
	}
	if items[0].ID != "19001" || items[0].Price != 4.99 || items[0].Name != "Salmon" {
		t.Fatalf("unexpected item: %+v", items[0])
	}
}

func TestResolveCartItems_ByName(t *testing.T) {
	idx := buildIndex(t, sampleMenu)
	items := []store.OrderItem{{ItemID: "salmon and avocado roll 8pc", ID: "salmon and avocado roll 8pc", Quantity: 2}}
	if err := resolveCartItems(idx, items); err != nil {
		t.Fatalf("err: %v", err)
	}
	if items[0].ID != "19031" || items[0].Price != 12.99 {
		t.Fatalf("name lookup failed: %+v", items[0])
	}
}

func TestResolveCartItems_Unknown(t *testing.T) {
	idx := buildIndex(t, sampleMenu)
	items := []store.OrderItem{{ItemID: "99999", ID: "99999", Quantity: 1}}
	err := resolveCartItems(idx, items)
	if err == nil {
		t.Fatal("expected error for unknown item")
	}
}

func TestMenuIndex_SkipsZeroID(t *testing.T) {
	idx := buildIndex(t, sampleMenu)
	if _, ok := idx.byID["0"]; ok {
		t.Fatal("zero-id item should be skipped")
	}
	if len(idx.byID) != 2 {
		t.Fatalf("want 2 indexed items, got %d", len(idx.byID))
	}
}
