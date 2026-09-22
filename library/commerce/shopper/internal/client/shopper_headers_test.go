// Copyright 2026 educrvz and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for shopper_headers.go — PATCH: store-scoping (6 stores, cache-aware)

package client

import (
	"strings"
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
		// PATCH: store-cluster-truth. unica and pet are cluster 1, not 3.
		// GET /features/stores is the only authority: cluster_id is an
		// independent dimension, never a copy of the store id. Asserting the
		// store id here as the cluster is what let the original bug through.
		{"unica", "3", "1", true},        // store 3, cluster 1 — NOT 3/3
		{"pet", "5", "1", true},          // store 5, cluster 1 — NOT 5/3
		{"now", "6", "11", true},         // ultra-fast store
		{"now-bebidas", "8", "11", true}, // beverages ultra-fast
		{"mensal", "1", "1", true},       // alias for programada
		{"pontual", "3", "1", true},      // alias for unica, cluster 1
		{"FRESH", "2", "1", true},        // case-insensitive
		{"  fresh ", "2", "1", true},     // trimmed
		{"2", "2", "1", true},            // raw id maps to known cluster
		{"5", "5", "1", true},            // raw id keeps pet's cluster 1
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

// TestClusterIDIsNeverCopiedFromStoreID is the regression guard for the bug
// this patch fixes: the store catalog once carried cluster_id = store_id for
// `unica`, which routed every --store unica request to a cart bucket the
// website never opens. Only the two ultra-fast storefronts sit off cluster 1.
//
// PATCH: store-cluster-truth.
func TestClusterIDIsNeverCopiedFromStoreID(t *testing.T) {
	wantCluster := map[string]string{
		"programada":  "1",
		"fresh":       "1",
		"unica":       "1",
		"pet":         "1",
		"now":         "11",
		"now-bebidas": "11",
	}
	for name, want := range wantCluster {
		st, ok := ResolveStore(name)
		if !ok {
			t.Fatalf("ResolveStore(%q) not found", name)
		}
		if st.ClusterID != want {
			t.Errorf("%s: cluster = %s, want %s", name, st.ClusterID, want)
		}
		if st.ClusterID == st.StoreID && want != st.StoreID {
			t.Errorf("%s: cluster was copied from the store id (%s)", name, st.StoreID)
		}
	}
}

// TestStoreCatalogDriftDetectsClusterMismatch feeds StoreCatalogDrift the exact
// shape GET /features/stores returns and asserts it names a cluster mismatch.
// This is the guard that would have caught the original bug.
//
// PATCH: store-cluster-truth.
func TestStoreCatalogDriftDetectsClusterMismatch(t *testing.T) {
	// Live payload as of 2026-09-22 — matches the baked table exactly.
	clean := []byte(`{"stores":[
		{"number":1,"subdomain":"programada","cluster_id":1,"with_recurrence":true,"is_ultra_fast_delivery":false},
		{"number":2,"subdomain":"fresh","cluster_id":1,"with_recurrence":true,"is_ultra_fast_delivery":false},
		{"number":5,"subdomain":"pet","cluster_id":1,"with_recurrence":true,"is_ultra_fast_delivery":false},
		{"number":6,"subdomain":"now","cluster_id":11,"with_recurrence":false,"is_ultra_fast_delivery":true},
		{"number":8,"subdomain":"now-bebidas","cluster_id":11,"with_recurrence":false,"is_ultra_fast_delivery":true},
		{"number":3,"subdomain":"unica","cluster_id":1,"with_recurrence":false,"is_ultra_fast_delivery":false}]}`)
	if drift := StoreCatalogDrift(clean); len(drift) != 0 {
		t.Errorf("clean payload reported drift: %v", drift)
	}

	// The API moves unica to a new cluster: the CLI must say so.
	moved := []byte(`{"stores":[{"number":3,"subdomain":"unica","cluster_id":7,"with_recurrence":false,"is_ultra_fast_delivery":false}]}`)
	drift := StoreCatalogDrift(moved)
	if len(drift) != 1 || !strings.Contains(drift[0], "cluster_id is 7") {
		t.Errorf("moved cluster drift = %v, want one line naming cluster_id 7", drift)
	}

	// A brand-new storefront the table does not know about.
	added := []byte(`{"stores":[{"number":9,"subdomain":"vinhos","cluster_id":11,"with_recurrence":false,"is_ultra_fast_delivery":true}]}`)
	if drift := StoreCatalogDrift(added); len(drift) != 1 || !strings.Contains(drift[0], "missing from the CLI store table") {
		t.Errorf("new storefront drift = %v, want a missing-store line", drift)
	}

	// Best-effort: garbage must never turn a working `stores` read into noise.
	if drift := StoreCatalogDrift([]byte(`not json`)); drift != nil {
		t.Errorf("unparseable body reported drift: %v", drift)
	}
}

// TestUltraFastIsNotTheInverseOfRecurrence pins the distinction that
// checkout preview got wrong: `unica` has no recurring basket AND is not
// ultra-fast, so "not a subscription store" can never stand in for "ultra fast".
//
// PATCH: store-cluster-truth.
func TestUltraFastIsNotTheInverseOfRecurrence(t *testing.T) {
	unica := StoreFor("unica")
	if unica.WithRecurrence {
		t.Error("unica should not be a recurring store")
	}
	if unica.UltraFast {
		t.Error("unica is not ultra-fast; the API reports is_ultra_fast_delivery=false")
	}
	for _, name := range []string{"now", "now-bebidas"} {
		if !StoreFor(name).UltraFast {
			t.Errorf("%s should be ultra-fast", name)
		}
	}
	// An unknown selector falls back to programada rather than a zero Store.
	if got := StoreFor("bogus"); got.StoreID != "1" || got.Subdomain != "programada" {
		t.Errorf("StoreFor(bogus) = %+v, want the programada default", got)
	}
}
