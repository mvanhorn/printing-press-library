package client

import (
	"github.com/mvanhorn/printing-press-library/library/commerce/shopper/internal/config"
	"testing"
)

func TestShopperHeadersInjected(t *testing.T) {
	cfg := &config.Config{BaseURL: "https://siteapi.shopper.com.br"}
	c := New(cfg, 0, 0)

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
		{"unica", "3", "1", true},
		{"pet", "5", "3", true},
		{"mensal", "1", "1", true},   // alias
		{"pontual", "3", "1", true},  // alias
		{"FRESH", "2", "1", true},    // case-insensitive
		{"  fresh ", "2", "1", true}, // trimmed
		{"2", "2", "1", true},        // raw id maps to known cluster
		{"5", "5", "3", true},        // raw id keeps pet's cluster 3
		{"99", "99", "1", true},      // unknown id defaults cluster 1
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

func TestSpendStoreNamesCoversAllFour(t *testing.T) {
	got := SpendStoreNames()
	if len(got) != 4 {
		t.Fatalf("SpendStoreNames len = %d, want 4 (%v)", len(got), got)
	}
	// Every default spend store must resolve to a real store/cluster pair.
	for _, n := range got {
		if _, ok := ResolveStore(n); !ok {
			t.Errorf("SpendStoreNames includes %q which does not resolve", n)
		}
	}
	want := map[string]bool{"programada": true, "fresh": true, "pet": true, "unica": true}
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

func TestSetStoreHeadersOverridesDefault(t *testing.T) {
	cfg := &config.Config{BaseURL: "https://siteapi.shopper.com.br"}
	c := New(cfg, 0, 0) // defaults to store 1
	SetStoreHeaders(c, Store{StoreID: "2", ClusterID: "1"})
	if c.Config.Headers["x-store-id"] != "2" {
		t.Errorf("x-store-id = %q, want 2 after SetStoreHeaders", c.Config.Headers["x-store-id"])
	}
}

// TestCacheKeyStoreAware guards the bug where two stores collided in the cache:
// the same path+params under different x-store-id headers must hash differently.
func TestCacheKeyStoreAware(t *testing.T) {
	mk := func(storeID, cluster string) string {
		c := &Client{
			BaseURL: "https://siteapi.shopper.com.br",
			Config:  &config.Config{Headers: map[string]string{"x-store-id": storeID, "x-cluster-id": cluster}},
		}
		return c.cacheKey("/orders/orders", map[string]string{"size": "500"})
	}
	programada := mk("1", "1")
	fresh := mk("2", "1")
	if programada == fresh {
		t.Fatal("cacheKey must differ between store 1 and store 2 for the same path")
	}
	if mk("2", "1") != fresh {
		t.Error("cacheKey must be stable for the same store")
	}
}
