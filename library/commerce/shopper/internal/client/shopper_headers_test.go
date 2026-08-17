// Copyright 2026 educrvz and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for shopper_headers.go — PATCH: store-scoping (6 stores, cache-aware)

package client

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/shopper/internal/config"
)

func TestShopperHeadersInjected(t *testing.T) {
	cfg := &config.Config{BaseURL: "https://siteapi.shopper.com.br"}
	c := New(cfg, 0, 0)
	// PatchShopperHeaders is called by newClient() in the CLI layer; call it
	// explicitly here to test the header injection in isolation.
	PatchShopperHeaders(c)

	required := map[string]string{
		"app-os-x-version": "web:1002",
		"x-store-id":       "1",
		"x-cluster-id":     "1",
	}
	for k, want := range required {
		got := c.Config.Headers[k]
		if got != want {
			t.Errorf("header %q = %q, want %q", k, got, want)
		}
	}
}

func TestResolveStore(t *testing.T) {
	cases := []struct {
		in        string
		wantStore string
		wantClu   string
		wantOK    bool
	}{
		{"programada", "1", "1", true},
		{"fresh", "2", "1", true},
		{"unica", "3", "3", true}, // cluster 3, not 1 (PATCH: store-scoping)
		{"pet", "5", "3", true},
		{"now", "6", "11", true},         // ultra-fast store (NEW)
		{"now-bebidas", "8", "11", true}, // beverages ultra-fast (NEW)
		{"mensal", "1", "1", true},       // alias for programada
		{"pontual", "3", "3", true},      // alias for unica, cluster 3
		{"FRESH", "2", "1", true},        // case-insensitive
		{"  fresh ", "2", "1", true},     // trimmed
		{"2", "2", "1", true},            // raw id maps to known cluster
		{"5", "5", "3", true},            // raw id keeps pet's cluster 3
		{"6", "6", "11", true},           // raw id maps to now cluster 11
		{"99", "99", "1", true},          // unknown id defaults cluster 1
		{"bogus", "", "", false},
		{"", "", "", false},
	}
	for _, c := range cases {
		st, ok := ResolveStore(c.in)
		if ok != c.wantOK {
			t.Errorf("ResolveStore(%q) ok = %v, want %v", c.in, ok, c.wantOK)
			continue
		}
		if ok && (st.StoreID != c.wantStore || st.ClusterID != c.wantClu) {
			t.Errorf("ResolveStore(%q) = %s/%s, want %s/%s", c.in, st.StoreID, st.ClusterID, c.wantStore, c.wantClu)
		}
	}
}

func TestSpendStoreNamesCoversAllSix(t *testing.T) {
	got := SpendStoreNames()
	if len(got) != 6 {
		t.Fatalf("SpendStoreNames len = %d, want 6 (%v)", len(got), got)
	}
	for _, n := range got {
		if _, ok := ResolveStore(n); !ok {
			t.Errorf("SpendStoreNames includes %q which does not resolve", n)
		}
	}
	want := map[string]bool{
		"programada": true, "fresh": true, "pet": true,
		"unica": true, "now": true, "now-bebidas": true,
	}
	for _, n := range got {
		if !want[n] {
			t.Errorf("unexpected store %q in SpendStoreNames", n)
		}
		delete(want, n)
	}
	if len(want) != 0 {
		t.Errorf("SpendStoreNames missing stores: %v", want)
	}
}

func TestSubscriptionStoreNames(t *testing.T) {
	got := SubscriptionStoreNames()
	if len(got) != 3 {
		t.Fatalf("SubscriptionStoreNames len = %d, want 3", len(got))
	}
	for _, n := range got {
		st, ok := ResolveStore(n)
		if !ok {
			t.Errorf("subscription store %q does not resolve", n)
		}
		if !st.WithRecurrence {
			t.Errorf("store %q is not marked with_recurrence=true", n)
		}
	}
}

func TestSetStoreHeadersOverridesDefault(t *testing.T) {
	cfg := &config.Config{BaseURL: "https://siteapi.shopper.com.br"}
	c := New(cfg, 0, 0)
	SetStoreHeaders(c, Store{StoreID: "2", ClusterID: "1"})
	if c.Config.Headers["x-store-id"] != "2" {
		t.Errorf("x-store-id = %q, want 2 after SetStoreHeaders", c.Config.Headers["x-store-id"])
	}
}

// TestCacheKeyStoreAware guards the cache-collision bug: two stores using the
// same path+params must produce different cache keys. Uses SetStoreHeaders so
// the x-shopper-store-api-version sentinel is set (it is the field that
// canonicalRepresentationHeaders includes in the key hash).
func TestCacheKeyStoreAware(t *testing.T) {
	mk := func(st Store) string {
		c := &Client{
			BaseURL: "https://siteapi.shopper.com.br",
			Config:  &config.Config{},
		}
		c.Config.Headers = make(map[string]string)
		SetStoreHeaders(c, st)
		return c.cacheKey("/orders/orders", map[string]string{"size": "500"})
	}
	programada := mk(Store{StoreID: "1", ClusterID: "1"})
	fresh := mk(Store{StoreID: "2", ClusterID: "1"})
	now := mk(Store{StoreID: "6", ClusterID: "11"})
	if programada == fresh {
		t.Fatal("cacheKey must differ between store 1 and store 2 for the same path")
	}
	if fresh == now {
		t.Fatal("cacheKey must differ between store 2 and store 6 (now)")
	}
	if mk(Store{StoreID: "2", ClusterID: "1"}) != fresh {
		t.Error("cacheKey must be stable for the same store")
	}
}

func TestStorefrontURL(t *testing.T) {
	cases := map[string]string{
		"programada":  "https://programada.shopper.com.br",
		"fresh":       "https://fresh.shopper.com.br",
		"unica":       "https://unica.shopper.com.br",
		"pet":         "https://pet.shopper.com.br",
		"now":         "https://now.shopper.com.br",
		"now-bebidas": "https://now-bebidas.shopper.com.br",
		"mensal":      "https://programada.shopper.com.br",
	}
	for in, want := range cases {
		got := StorefrontURL(in)
		if got != want {
			t.Errorf("StorefrontURL(%q) = %q, want %q", in, got, want)
		}
	}
}
