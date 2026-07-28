// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package storeapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestMajorPrices covers the single easiest way to be 100x wrong against this
// API: Store API prices are integer minor units in a string, with the divisor
// carried alongside them.
func TestMajorPrices(t *testing.T) {
	cases := []struct {
		name                     string
		prices                   Prices
		wantPrice, wantReg, want float64
	}{
		{
			name:      "usd two minor digits",
			prices:    Prices{Price: "4900", RegularPrice: "5900", SalePrice: "4900", CurrencyMinorUnit: 2},
			wantPrice: 49, wantReg: 59, want: 49,
		},
		{
			name:      "zero decimal currency",
			prices:    Prices{Price: "4900", RegularPrice: "4900", SalePrice: "0", CurrencyMinorUnit: 0},
			wantPrice: 4900, wantReg: 4900, want: 0,
		},
		{
			name:      "three minor digits",
			prices:    Prices{Price: "49000", RegularPrice: "49000", SalePrice: "0", CurrencyMinorUnit: 3},
			wantPrice: 49, wantReg: 49, want: 0,
		},
		{
			name:      "empty and unparseable are zero not panic",
			prices:    Prices{Price: "", RegularPrice: "n/a", SalePrice: "  ", CurrencyMinorUnit: 2},
			wantPrice: 0, wantReg: 0, want: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := Product{Prices: tc.prices}
			gotPrice, gotReg, gotSale := p.MajorPrices()
			if gotPrice != tc.wantPrice || gotReg != tc.wantReg || gotSale != tc.want {
				t.Fatalf("MajorPrices() = (%v, %v, %v), want (%v, %v, %v)",
					gotPrice, gotReg, gotSale, tc.wantPrice, tc.wantReg, tc.want)
			}
		})
	}
}

func TestNewNormalizesStoreURL(t *testing.T) {
	cases := []struct{ in, wantOrigin string }{
		{"https://store.example", "https://store.example"},
		{"https://store.example/", "https://store.example"},
		{"https://store.example/wp-json", "https://store.example"},
		{"https://store.example/wp-json/wc/store/v1/products", "https://store.example"},
		{"store.example", "https://store.example"},
	}
	for _, tc := range cases {
		c, err := New(tc.in, time.Second)
		if err != nil {
			t.Fatalf("New(%q) error = %v", tc.in, err)
		}
		if c.Origin() != tc.wantOrigin {
			t.Fatalf("New(%q).Origin() = %q, want %q", tc.in, c.Origin(), tc.wantOrigin)
		}
	}
}

func TestNewRejectsBadInput(t *testing.T) {
	for _, in := range []string{"", "   ", "ftp://store.example", "file:///etc/passwd"} {
		if _, err := New(in, time.Second); err == nil {
			t.Fatalf("New(%q) expected an error, got nil", in)
		}
	}
}

func TestProductsDecodesAndReportsTotalPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-WP-TotalPages", "7")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":12,"name":"Widget &#39;A&#39;","sku":"W-A","permalink":"https://s/p/12",
			"on_sale":true,"is_in_stock":true,
			"prices":{"price":"1999","regular_price":"2999","sale_price":"1999","currency_code":"USD","currency_minor_unit":2}}]`))
	}))
	defer srv.Close()

	c, err := New(srv.URL, 5*time.Second)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	products, totalPages, err := c.Products(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("Products() error = %v", err)
	}
	if totalPages != 7 {
		t.Fatalf("totalPages = %d, want 7", totalPages)
	}
	if len(products) != 1 {
		t.Fatalf("len(products) = %d, want 1", len(products))
	}
	price, regular, sale := products[0].MajorPrices()
	if price != 19.99 || regular != 29.99 || sale != 19.99 {
		t.Fatalf("prices = (%v,%v,%v), want (19.99,29.99,19.99)", price, regular, sale)
	}
	// HTML entities must not leak into stored names.
	if got := products[0].CleanName(); got != "Widget 'A'" {
		t.Fatalf("CleanName() = %q, want %q", got, "Widget 'A'")
	}
}

// TestProductsSurfacesRateLimit proves an exhausted 429 becomes a typed error
// rather than an empty result set. Empty must mean "no products", never
// "we got throttled".
func TestProductsSurfacesRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"code":"too_many_requests"}`))
	}))
	defer srv.Close()

	c, err := New(srv.URL, 5*time.Second)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	products, _, err := c.Products(context.Background(), 1, 10)
	if err == nil {
		t.Fatalf("Products() error = nil, want a rate-limit error; got %d products", len(products))
	}
	if products != nil {
		t.Fatalf("products = %v, want nil on rate limit", products)
	}
}
