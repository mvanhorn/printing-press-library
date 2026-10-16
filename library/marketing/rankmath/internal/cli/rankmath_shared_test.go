// Copyright 2026 ElizabethcStone and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/store"
)

func TestStoreScopeIdentityIncludesSite(t *testing.T) {
	a := &config.Config{TemplateVars: map[string]string{"site": "HTTPS://Example.COM/"}}
	b := &config.Config{TemplateVars: map[string]string{"site": "other.example"}}
	aSame := &config.Config{TemplateVars: map[string]string{"site": "example.com"}}
	if a.StoreScopeCredential() == b.StoreScopeCredential() {
		t.Fatal("different sites shared a store scope")
	}
	if a.StoreScopeCredential() != aSame.StoreScopeCredential() {
		t.Fatal("equivalent site hosts produced different store scopes")
	}
}

func TestLoadPublishedInventoryFiltersConfiguredSite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inventory.db")
	db, err := store.OpenWithContext(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for id, link := range map[string]string{"1": "https://example.com/one", "2": "https://other.example/two"} {
		row, _ := json.Marshal(map[string]string{"link": link, "status": "publish"})
		if err := db.Upsert("posts", id, row); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("RANKMATH_SITE", "https://EXAMPLE.com/")
	items, err := loadPublishedInventory(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].URL != "https://example.com/one" {
		t.Fatalf("inventory=%+v", items)
	}
}

func TestStripWWWMatchesSiteAliases(t *testing.T) {
	for _, tc := range []struct{ a, b string }{
		{"https://www.example.com/post/", "example.com"},
		{"https://example.com/post/", "www.example.com"},
	} {
		if stripWWW(normalizeSiteHost(tc.a)) != stripWWW(normalizeSiteHost(tc.b)) {
			t.Fatalf("%q and %q should be the same site", tc.a, tc.b)
		}
	}
	if stripWWW(normalizeSiteHost("https://other.example.com/")) == stripWWW(normalizeSiteHost("example.com")) {
		t.Fatal("different subdomain must not match")
	}
}
