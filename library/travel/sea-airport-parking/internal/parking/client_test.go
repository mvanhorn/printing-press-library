// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.

package parking

import (
	"testing"
	"time"
)

func TestParseQuoteAvailable(t *testing.T) {
	body := `
	<div data-product-name="Reserved Parking" data-product-code="RP" data-position="1"></div>
	<script>var d = {"quotes":[{"id":335,"price":188.0,"originalPrice":"$210.00"}]};</script>
	`
	q := &Quote{EntryStr: "2026-08-15T11:00", ExitStr: "2026-08-18T11:00", Nights: 3}
	got, err := parseQuote(body, "https://x/book/SEA/Parking?parkingCmd=selectProduct", q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Available || got.SoldOut || got.Invalid {
		t.Fatalf("expected available, got %+v", got)
	}
	if got.TotalPrice != 188.0 {
		t.Errorf("TotalPrice = %v, want 188", got.TotalPrice)
	}
	if got.OriginalPrice != 210.0 {
		t.Errorf("OriginalPrice = %v, want 210", got.OriginalPrice)
	}
	if got.ProductCode != "RP" || got.ProductName != "Reserved Parking" {
		t.Errorf("product = %q/%q", got.ProductCode, got.ProductName)
	}
}

func TestParseQuoteSoldOut(t *testing.T) {
	body := `<div data-product-name="Reserved Parking" data-product-code="RP"></div>
	<p>Sorry, this product is unavailable for the dates you requested.</p>`
	q := &Quote{}
	got, err := parseQuote(body, "https://x/book/SEA/Parking?parkingCmd=selectProduct", q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.SoldOut || got.Available {
		t.Fatalf("expected sold out, got %+v", got)
	}
	if got.Reason == "" {
		t.Error("expected a sold-out reason")
	}
}

func TestParseQuoteInvalid(t *testing.T) {
	body := `<span class="error">A minimum 2 day stay is required.</span>`
	q := &Quote{}
	got, err := parseQuote(body, "https://x/book/SEA/Parking?parkingCmd=collectParkingDetails", q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Invalid {
		t.Fatalf("expected invalid, got %+v", got)
	}
	if got.Reason == "" {
		t.Error("expected an invalid reason")
	}
}

func TestParseNightlyPricesDedup(t *testing.T) {
	// The array is echoed in list + card views; dedup by date.
	body := `[{"date":"2026-08-12T00:00:00Z","rollupPrice":0.0,"price":47.0},` +
		`{"date":"2026-08-13T00:00:00Z","rollupPrice":0.0,"price":47.0}]` +
		`[{"date":"2026-08-12T00:00:00Z","rollupPrice":0.0,"price":47.0},` +
		`{"date":"2026-08-13T00:00:00Z","rollupPrice":0.0,"price":47.0}]`
	got := parseNightlyPrices(body)
	if len(got) != 2 {
		t.Fatalf("expected 2 deduped nights, got %d: %+v", len(got), got)
	}
	if got[0].Date != "2026-08-12" || got[0].Price != 47.0 {
		t.Errorf("first night = %+v", got[0])
	}
}

func TestSnapTime(t *testing.T) {
	cases := []struct {
		h, m int
		want string
	}{
		{11, 0, "11:00"},
		{0, 0, "00:01"},
		{0, 15, "00:01"},
		{23, 30, "23:59"},
		{9, 45, "10:00"},
		{14, 20, "14:00"},
	}
	for _, c := range cases {
		got := snapTime(time.Date(2026, 8, 15, c.h, c.m, 0, 0, time.UTC))
		if got != c.want {
			t.Errorf("snapTime(%02d:%02d) = %q, want %q", c.h, c.m, got, c.want)
		}
	}
}

func TestBilledNights(t *testing.T) {
	base := time.Date(2026, 8, 15, 11, 0, 0, 0, time.UTC)
	cases := []struct {
		exit time.Time
		want int
	}{
		{base.AddDate(0, 0, 3), 3},                    // exactly 3 days
		{base.AddDate(0, 0, 3).Add(2 * time.Hour), 4}, // 3 days + 2h -> 4th started day
		{base.AddDate(0, 0, 1), 1},
		{base, 0},                // exit == entry
		{base.Add(time.Hour), 1}, // < 1 day rounds up to 1
	}
	for _, c := range cases {
		got := billedNights(base, c.exit)
		if got != c.want {
			t.Errorf("billedNights(exit=%v) = %d, want %d", c.exit, got, c.want)
		}
	}
}
