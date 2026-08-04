// Copyright 2026 chiotas and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored tests for the variants command helpers and stock
// classification (F1/F3 of the 2026-08 dogfood findings).

package cli

import (
	"encoding/json"
	"testing"
)

func TestFgProductSlugFromURL(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"https://nisifilters.it/prodotto/nisi-jetmag-pro-anelli-adattatori/", "nisi-jetmag-pro-anelli-adattatori", false},
		{"https://www.nisifilters.it/prodotto/some-slug", "some-slug", false},
		{"https://nisifilters.it/blog/not-a-product/", "", true},
		{"not-a-url", "", true},
	}
	for _, c := range cases {
		got, err := fgProductSlugFromURL(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("fgProductSlugFromURL(%q) err=%v, wantErr=%v", c.in, err, c.wantErr)
		}
		if got != c.want {
			t.Errorf("fgProductSlugFromURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFgStockStatus(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{`{"is_in_stock":true,"is_on_backorder":false}`, "in"},
		{`{"is_in_stock":true,"is_on_backorder":true}`, "backorder"}, // kit 6285 shape
		{`{"is_in_stock":false,"is_on_backorder":false}`, "out"},
		{`{}`, "out"},
	}
	for _, c := range cases {
		if got := fgStockStatus(fgDecode(json.RawMessage(c.raw))); got != c.want {
			t.Errorf("fgStockStatus(%s) = %q, want %q", c.raw, got, c.want)
		}
	}
}
