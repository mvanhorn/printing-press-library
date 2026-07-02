// Copyright 2026 wandreis and contributors. Licensed under Apache-2.0. See LICENSE.

package mlextract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func TestParseSearchGraph(t *testing.T) {
	listings, err := ParseSearchGraph(readFixture(t, "search_graph.json"))
	if err != nil {
		t.Fatalf("ParseSearchGraph error: %v", err)
	}
	if len(listings) != 3 {
		t.Fatalf("expected 3 listings (SearchResultsPage skipped), got %d", len(listings))
	}

	cases := []struct {
		idx       int
		catalogID string
		price     float64
		brand     string
	}{
		{0, "MLB51764304", 14999, "Asus"},
		{1, "MLBU3664751795", 32999, "Asus"},
		{2, "MLB40287816", 5794.90, "Lenovo"},
	}
	for _, c := range cases {
		got := listings[c.idx]
		if got.CatalogID != c.catalogID {
			t.Errorf("listing[%d] catalog_id = %q, want %q", c.idx, got.CatalogID, c.catalogID)
		}
		if got.Price != c.price {
			t.Errorf("listing[%d] price = %v, want %v", c.idx, got.Price, c.price)
		}
		if got.Brand != c.brand {
			t.Errorf("listing[%d] brand = %q, want %q", c.idx, got.Brand, c.brand)
		}
		if got.Currency != "BRL" {
			t.Errorf("listing[%d] currency = %q, want BRL", c.idx, got.Currency)
		}
		if got.Availability != "InStock" {
			t.Errorf("listing[%d] availability = %q, want InStock", c.idx, got.Availability)
		}
	}

	// First listing carries a rating; the second (no aggregateRating) is zero.
	if listings[0].RatingValue != 4.8 || listings[0].RatingCount != 131 {
		t.Errorf("listing[0] rating = %v/%d, want 4.8/131", listings[0].RatingValue, listings[0].RatingCount)
	}
	if listings[1].RatingCount != 0 {
		t.Errorf("listing[1] rating_count = %d, want 0 (no aggregateRating)", listings[1].RatingCount)
	}

	// StoreJSON must add the id field keyed to catalog_id.
	var rec map[string]any
	if err := json.Unmarshal(listings[0].StoreJSON(), &rec); err != nil {
		t.Fatalf("StoreJSON unmarshal: %v", err)
	}
	if rec["id"] != "MLB51764304" || rec["catalog_id"] != "MLB51764304" {
		t.Errorf("StoreJSON id/catalog_id = %v/%v, want MLB51764304", rec["id"], rec["catalog_id"])
	}
}

func TestParseProduct(t *testing.T) {
	p, err := ParseProduct(readFixture(t, "product.json"))
	if err != nil {
		t.Fatalf("ParseProduct error: %v", err)
	}
	if p.CatalogID != "MLB51764304" {
		t.Errorf("catalog_id = %q, want MLB51764304", p.CatalogID)
	}
	if p.Price != 13349.11 {
		t.Errorf("price = %v, want 13349.11", p.Price)
	}
	if p.ReviewCount != 71 {
		t.Errorf("review_count = %d, want 71", p.ReviewCount)
	}
	if p.RatingValue != 4.8 || p.RatingCount != 131 {
		t.Errorf("rating = %v/%d, want 4.8/131", p.RatingValue, p.RatingCount)
	}
	if p.Availability != "InStock" {
		t.Errorf("availability = %q, want InStock", p.Availability)
	}
	if p.Currency != "BRL" {
		t.Errorf("currency = %q, want BRL", p.Currency)
	}
	if p.Brand != "Asus" {
		t.Errorf("brand = %q, want Asus", p.Brand)
	}
}

func TestParseProductOfferShapes(t *testing.T) {
	// Array of offers -> lowest price wins; availability/url from chosen offer.
	arr := []byte(`{"name":"X","sku":"MLB999","offers":[
		{"@type":"Offer","price":200,"priceCurrency":"BRL","availability":"https://schema.org/InStock","url":"https://x/p/MLB999"},
		{"@type":"Offer","price":150,"priceCurrency":"BRL","availability":"https://schema.org/InStock","url":"https://x/p/MLB999"}]}`)
	p, err := ParseProduct(arr)
	if err != nil {
		t.Fatalf("array offers: %v", err)
	}
	if p.Price != 150 {
		t.Errorf("array offers price = %v, want 150 (lowest)", p.Price)
	}

	// AggregateOffer with lowPrice fallback (no bare price).
	agg := []byte(`{"name":"Y","sku":"MLB888","offers":{"@type":"AggregateOffer","lowPrice":"99.90","highPrice":"120","priceCurrency":"BRL","url":"https://y/up/MLBU888"}}`)
	p2, err := ParseProduct(agg)
	if err != nil {
		t.Fatalf("aggregate offer: %v", err)
	}
	if p2.Price != 99.90 {
		t.Errorf("aggregate offer price = %v, want 99.90 (lowPrice)", p2.Price)
	}
	if p2.CatalogID != "MLBU888" {
		t.Errorf("aggregate offer catalog_id = %q, want MLBU888", p2.CatalogID)
	}
}

func TestParseSpecTable(t *testing.T) {
	attrs, err := ParseSpecTable(readFixture(t, "spec_table.html"))
	if err != nil {
		t.Fatalf("ParseSpecTable error: %v", err)
	}
	if len(attrs) != 7 {
		t.Fatalf("expected 7 attributes, got %d: %+v", len(attrs), attrs)
	}
	want := map[string]string{
		"Capacidade de disco SSD":     "1 TB",
		"Taxa de atualização da tela": "240 Hz",
		"Tamanho da tela":             "16 \"",
	}
	got := map[string]string{}
	for _, a := range attrs {
		got[a.Name] = a.Value
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("attr %q = %q, want %q", k, got[k], v)
		}
	}
}

func TestExtractLeadingNumber(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"700 W", 700, true},
		{"240 Hz", 240, true},
		{"1 TB", 1, true},
		{"2560 px x 1600 px", 2560, true},
		{"220V", 220, true},
		{"5794,90", 5794.90, true},
		{"NVIDIA", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := ExtractLeadingNumber(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("ExtractLeadingNumber(%q) = (%v,%v), want (%v,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestCatalogIDFromURL(t *testing.T) {
	cases := map[string]string{
		"https://www.mercadolivre.com.br/notebook/p/MLB51764304":     "MLB51764304",
		"https://www.mercadolivre.com.br/notebook/up/MLBU3664751795": "MLBU3664751795",
		"https://x/p/MLB40287816?ref=abc":                            "MLB40287816",
	}
	for in, want := range cases {
		if got := CatalogIDFromURL(in); got != want {
			t.Errorf("CatalogIDFromURL(%q) = %q, want %q", in, got, want)
		}
	}
}
