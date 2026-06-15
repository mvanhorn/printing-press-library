// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Tests for the hand-authored Daraz helpers and novel-command wiring.

package cli

import "testing"

func TestParseMoney(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"7199", 7199},
		{"21,000", 21000},
		{"Rs. 1,499.50", 1499.50},
		{"", 0},
		{"free", 0},
	}
	for _, c := range cases {
		if got := parseMoney(c.in); got != c.want {
			t.Errorf("parseMoney(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestLeadingInt(t *testing.T) {
	// leadingInt backs discount %% and review counts (plain leading digits).
	cases := []struct {
		in   string
		want int
	}{
		{"546", 546},
		{"66% Off", 66},
		{"151", 151},
		{"none", 0},
		{"", 0},
	}
	for _, c := range cases {
		if got := leadingInt(c.in); got != c.want {
			t.Errorf("leadingInt(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseSold(t *testing.T) {
	// Sold counts use k/m/b shorthand; the deals score depends on expanding them.
	cases := []struct {
		in   string
		want int
	}{
		{"546 sold", 546},
		{"1.2k sold", 1200},
		{"2.5K+ sold", 2500},
		{"3m sold", 3000000},
		{"sold out", 0},
		{"", 0},
	}
	for _, c := range cases {
		if got := parseSold(c.in); got != c.want {
			t.Errorf("parseSold(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestDiscountPct(t *testing.T) {
	// Label wins when present.
	p := darazProduct{Discount: "66% Off", Price: "7199", OriginalPrice: "21000"}
	if got := p.discountPct(); got != 66 {
		t.Errorf("labelled discountPct = %v, want 66", got)
	}
	// Computed from prices when label absent.
	p2 := darazProduct{Price: "50", OriginalPrice: "100"}
	if got := p2.discountPct(); got != 50 {
		t.Errorf("computed discountPct = %v, want 50", got)
	}
	// No discount when price >= original and no label.
	p3 := darazProduct{Price: "100", OriginalPrice: "100"}
	if got := p3.discountPct(); got != 0 {
		t.Errorf("no-discount discountPct = %v, want 0", got)
	}
}

func TestDealScoreMonotonic(t *testing.T) {
	// Higher discount, rating, and sales should each raise the score.
	base := dealScore(20, 4.0, 100)
	if dealScore(40, 4.0, 100) <= base {
		t.Error("higher discount should increase deal score")
	}
	if dealScore(20, 5.0, 100) <= base {
		t.Error("higher rating should increase deal score")
	}
	if dealScore(20, 4.0, 1000) <= base {
		t.Error("more sales should increase deal score")
	}
	// Unrated items are damped, not zeroed.
	if dealScore(50, 0, 0) <= 0 {
		t.Error("unrated item with a discount should still score above zero")
	}
}

func TestMedianFloat(t *testing.T) {
	cases := []struct {
		in   []float64
		want float64
	}{
		{nil, 0},
		{[]float64{5}, 5},
		{[]float64{3, 1, 2}, 2},
		{[]float64{4, 1, 3, 2}, 2.5},
	}
	for _, c := range cases {
		if got := medianFloat(c.in); got != c.want {
			t.Errorf("medianFloat(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestExtractProductDetail(t *testing.T) {
	html := `<html><head>
<script type="application/ld+json">{"@type":"Product","@context":"https://schema.org",
"name":"Test Widget","category":"Gadgets","sku":"SKU123",
"brand":{"@type":"Brand","name":"Acme"},
"image":["https://img/1.png","https://img/2.png"],
"description":"A &amp; useful <b>widget</b>",
"url":"https://www.daraz.pk/products/test-i42.html",
"offers":{"@type":"AggregateOffer","availability":"https://schema.org/InStock"}}</script>
</head><body></body></html>`
	pd := extractProductDetail(html, "42")
	if pd.Name != "Test Widget" {
		t.Errorf("name = %q, want Test Widget", pd.Name)
	}
	if pd.Brand != "Acme" {
		t.Errorf("brand = %q, want Acme", pd.Brand)
	}
	if pd.Category != "Gadgets" {
		t.Errorf("category = %q, want Gadgets", pd.Category)
	}
	if pd.Availability != "InStock" {
		t.Errorf("availability = %q, want InStock", pd.Availability)
	}
	if pd.Image != "https://img/1.png" {
		t.Errorf("image = %q, want first image", pd.Image)
	}
	if pd.ItemID != "42" {
		t.Errorf("itemId = %q, want 42", pd.ItemID)
	}
}

// Dry-run wiring: every novel command must short-circuit cleanly under
// --dry-run before any network or store access, returning nil.
func TestNovelCommandsDryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	type tc struct {
		name string
		run  func() error
	}
	cmds := []tc{
		{"deals", func() error { c := newNovelDealsCmd(flags); return c.RunE(c, []string{"laptop"}) }},
		{"value", func() error { c := newNovelValueCmd(flags); return c.RunE(c, []string{"laptop"}) }},
		{"compare", func() error { c := newNovelCompareCmd(flags); return c.RunE(c, []string{"laptop"}) }},
		{"since", func() error { c := newNovelSinceCmd(flags); return c.RunE(c, []string{"laptop"}) }},
		{"watch", func() error { c := newNovelWatchCmd(flags); return c.RunE(c, []string{"laptop"}) }},
		{"price-history", func() error { c := newNovelPriceHistoryCmd(flags); return c.RunE(c, []string{"42"}) }},
		{"products-get", func() error { c := newProductsGetCmd(flags); return c.RunE(c, []string{"42"}) }},
		{"seller-stats", func() error { c := newNovelSellerStatsCmd(flags); return c.RunE(c, []string{"123"}) }},
		{"seller-products", func() error { c := newSellerProductsCmd(flags); return c.RunE(c, []string{"123"}) }},
	}
	for _, c := range cmds {
		if err := c.run(); err != nil {
			t.Errorf("%s --dry-run returned error: %v", c.name, err)
		}
	}
}
