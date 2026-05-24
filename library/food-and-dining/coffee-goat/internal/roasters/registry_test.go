// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

package roasters

import (
	"strings"
	"testing"
)

func TestRegistryCount(t *testing.T) {
	if got := Count(); got != 24 {
		t.Fatalf("expected 24 roasters, got %d", got)
	}
}

func TestBySlug(t *testing.T) {
	cases := []struct {
		slug    string
		want    string
		wantErr bool
	}{
		{slug: "onyx", want: "Onyx Coffee Lab"},
		{slug: "sey", want: "Sey Coffee"},
		{slug: "black-and-white", want: "Black & White Coffee"},
		{slug: "tim-wendelboe", want: "Tim Wendelboe"},
		{slug: "dak", want: "DAK Coffee Roasters"},
		{slug: "no-such-roaster", wantErr: true},
	}
	for _, c := range cases {
		got, ok := BySlug(c.slug)
		if c.wantErr {
			if ok {
				t.Errorf("BySlug(%q) = %+v, want not-found", c.slug, got)
			}
			continue
		}
		if !ok {
			t.Errorf("BySlug(%q) not found", c.slug)
			continue
		}
		if got.Name != c.want {
			t.Errorf("BySlug(%q).Name = %q, want %q", c.slug, got.Name, c.want)
		}
	}
}

func TestShopifyOnly(t *testing.T) {
	shopify := ShopifyOnly()
	if len(shopify) != 21 {
		t.Fatalf("expected 21 Shopify roasters, got %d", len(shopify))
	}
	for _, r := range shopify {
		if r.Transport != TransportShopify {
			t.Errorf("ShopifyOnly returned non-Shopify roaster: %s/%s", r.Slug, r.Transport)
		}
	}
}

func TestAllAreReachable(t *testing.T) {
	for _, r := range All() {
		if r.Slug == "" {
			t.Errorf("roaster %q has empty slug", r.Name)
		}
		if r.Name == "" {
			t.Errorf("roaster %q has empty name", r.Slug)
		}
		if !strings.HasPrefix(r.SyncURL, "https://") {
			t.Errorf("roaster %q sync URL must be HTTPS: %q", r.Slug, r.SyncURL)
		}
		switch r.Transport {
		case TransportShopify, TransportWooCommerce, TransportSquareOnline, TransportSnipcart:
		default:
			t.Errorf("roaster %q has unknown transport %q", r.Slug, r.Transport)
		}
	}
}

func TestBlackAndWhiteHasCoffeeFilter(t *testing.T) {
	bw, ok := BySlug("black-and-white")
	if !ok {
		t.Fatal("black-and-white not found")
	}
	if got := bw.Filters["product_type"]; got != "Coffee" {
		t.Errorf("black-and-white product_type filter = %q, want %q", got, "Coffee")
	}
}
