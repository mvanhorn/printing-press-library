package gql

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return raw
}

// items_availability.json is a scrubbed live instacart.ca Items response:
// one null entry and a partial "Internal Error" on one item's price.
func TestParseItemDetails(t *testing.T) {
	got := parseItemDetails(readFixture(t, "items_availability.json"))
	if len(got) != 5 {
		t.Fatalf("want 5 items (null entry skipped), got %d", len(got))
	}
	tests := []struct {
		id, size, price, perUnit, label, availability string
	}{
		{"items_1000-9000000", "1.5 kg", "$20.99", "$1.40/100g", "Many in stock", AvailabilityAvailable},
		{"items_1000-9000001", "400 g", "", "", "Likely out of stock", AvailabilityLowStock},
		{"items_1000-9000002", "250 g", "$8.99", "$0.90/100g", "Out of stock", AvailabilityUnavailable},
		{"items_1000-9000003", "908 g", "", "", "Out of stock", AvailabilityUnavailable},
		{"items_1000-9000004", "3 lb", "$15.99", "$0.33/oz", "", AvailabilityAvailable},
	}
	for i, tc := range tests {
		d := got[i]
		if d.ItemID != tc.id || d.Size != tc.size || d.Price != tc.price || d.PricePerUnit != tc.perUnit || d.StockLabel != tc.label {
			t.Errorf("item %d: got %+v, want %+v", i, d, tc)
		}
		if a := d.Availability(); a != tc.availability {
			t.Errorf("item %d (%s): availability %q, want %q", i, d.Name, a, tc.availability)
		}
	}
}

// Unavailable items often keep stockLevel "inStock"; available=false wins.
func TestAvailabilityPrefersAvailableFlag(t *testing.T) {
	f, tr := false, true
	cases := []struct {
		d    ItemDetails
		want string
	}{
		{ItemDetails{Available: &f, StockLevel: "inStock"}, AvailabilityUnavailable},
		{ItemDetails{Available: &tr, StockLevel: "lowStock"}, AvailabilityLowStock},
		{ItemDetails{Available: &tr, StockLevel: "highlyInStock"}, AvailabilityAvailable},
		{ItemDetails{StockLevel: "inStock"}, AvailabilityUnknown},
	}
	for _, c := range cases {
		if got := c.d.Availability(); got != c.want {
			t.Errorf("%+v: got %q want %q", c.d, got, c.want)
		}
	}
}

func TestParseItemsResponseFillsSearchFields(t *testing.T) {
	got := parseItemsResponse(readFixture(t, "items_availability.json"), "costco-canada", 3)
	if len(got) != 3 {
		t.Fatalf("limit 3: got %d", len(got))
	}
	r := got[2]
	if r.PriceDisplay != "$8.99" || r.Size != "250 g" || r.Availability != AvailabilityUnavailable ||
		r.Available == nil || *r.Available || r.ProductID != "9000002" || r.Retailer != "costco-canada" {
		t.Fatalf("unexpected search result: %+v", r)
	}
	// Existing JSON keys stay; new ones are additive.
	out, _ := json.Marshal(r)
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	for _, k := range []string{"name", "item_id", "product_id", "price_display", "retailer", "size", "available", "availability", "stock_label", "price_per_unit"} {
		if _, ok := m[k]; !ok {
			t.Errorf("search JSON missing %q: %s", k, out)
		}
	}
}

func TestApplyItemDetailsToCartLines(t *testing.T) {
	raw, err := parseCartData(readFixture(t, "cart_data.json"))
	if err != nil {
		t.Fatal(err)
	}
	lines := make([]CartLineItem, 0, len(raw))
	for _, r := range raw {
		lines = append(lines, CartLineItem{ItemID: r.itemID, Quantity: r.quantity, Availability: AvailabilityUnknown})
	}
	applyItemDetails(lines, parseItemDetails(readFixture(t, "items_availability.json")))

	if l := lines[0]; l.Availability != AvailabilityAvailable || l.Price != "$20.99" || l.Size != "1.5 kg" || l.Name != "Poppies Profiteroles" || l.StockLevel != "highlyInStock" {
		t.Errorf("line 0: %+v", l)
	}
	if l := lines[1]; l.Availability != AvailabilityUnavailable || l.Available == nil || *l.Available || l.Quantity != 2 {
		t.Errorf("line 1 should be unavailable: %+v", l)
	}
	// Not returned by Items (not offered at this store).
	if l := lines[2]; l.Availability != AvailabilityUnknown || l.Available != nil || l.Price != "" {
		t.Errorf("line 2 should be unknown: %+v", l)
	}
	out, _ := json.Marshal(lines[2])
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	if m["availability"] != AvailabilityUnknown {
		t.Errorf("availability must always be present in cart JSON: %s", out)
	}
	if _, ok := m["available"]; ok {
		t.Errorf("available must be omitted when unknown: %s", out)
	}
}
