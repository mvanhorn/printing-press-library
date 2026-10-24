// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

func TestPricesBatchedPerStorefront(t *testing.T) {
	f := newS2AFake(t)
	res := f.mustOK("prices", "324684580", "553834731", "--countries", "us,gb", "--json")
	if res.requests != 2 {
		t.Fatalf("prices sent %d requests, want 2 (one batched Lookup per storefront): %v", res.requests, f.requestLog())
	}
	items, meta, failures := s2aItems(t, res.stdout)
	if len(items) != 4 || len(failures) != 0 {
		t.Fatalf("items %d failures %v", len(items), failures)
	}
	want := []struct{ cc, app, currency string }{
		{"us", "324684580", "USD"}, {"us", "553834731", "USD"}, {"gb", "324684580", "GBP"}, {"gb", "553834731", "GBP"},
	}
	for i, w := range want {
		it := items[i]
		if it["storefront"] != w.cc || it["app"] != w.app || it["currency"] != w.currency || it["available"] != true || it["formattedPrice"] != "Free" || s2aNum(t, it["price"]) != 0 {
			t.Fatalf("row %d = %v, want %+v", i, it, w)
		}
	}
	if apps, _ := meta["apps"].([]any); len(apps) != 2 {
		t.Fatalf("meta.apps = %v", meta["apps"])
	}
	snaps, _ := store.AppSnapshots(context.Background(), f.openStore(), "553834731", "", s2aFixedNow.AddDate(-10, 0, 0))
	if len(snaps) != 2 || snaps[0].Source != store.SourceLookup {
		t.Fatalf("lookup snapshots = %+v", snaps)
	}
}

func TestPricesUnavailableIsAnHonestRow(t *testing.T) {
	f := newS2AFake(t)
	items, _, _ := s2aItems(t, f.mustOK("prices", "324684580", "--countries", "us,cn", "--json").stdout)
	if len(items) != 2 || items[1]["storefront"] != "cn" || items[1]["available"] != false {
		t.Fatalf("items = %v", items)
	}
	for _, k := range []string{"price", "currency", "formattedPrice"} {
		if v, ok := items[1][k]; !ok || v != nil {
			t.Fatalf("unavailable row %s = %v, want null", k, v)
		}
	}
	snaps, _ := store.AppSnapshots(context.Background(), f.openStore(), "324684580", "cn", s2aFixedNow.AddDate(-10, 0, 0))
	if len(snaps) != 0 {
		t.Fatalf("an unavailable app must not get a snapshot: %+v", snaps)
	}
	items, _, _ = s2aItems(t, f.mustOK("prices", "324684580", "1", "--json").stdout)
	if len(items) != 2 || items[1]["app"] != "1" || items[1]["available"] != false {
		t.Fatalf("mixed ids = %v", items)
	}
}

func TestPricesBundleIDsAndNegative(t *testing.T) {
	f := newS2AFake(t)
	items, _, failures := s2aItems(t, f.mustOK("prices", "com.spotify.client", "com.example.nope", "--json").stdout)
	if len(items) != 1 || items[0]["app"] != "324684580" {
		t.Fatalf("bundle resolution items = %v", items)
	}
	if len(failures) != 1 || failures[0]["id"] != "com.example.nope" {
		t.Fatalf("unknown bundle should be a fetch failure: %v", failures)
	}
	if res := f.run("prices", "1", "--json"); res.code != 3 {
		t.Fatalf("prices 1 exit = %d, want 3", res.code)
	}
	if res := f.run("prices", "--json"); res.code != 2 || res.requests != 0 {
		t.Fatalf("prices without apps exit %d requests %d", res.code, res.requests)
	}
	if res := f.run("prices", "324684580", "bogus app"); res.code != 2 || res.requests != 0 {
		t.Fatalf("bad app ref exit %d requests %d", res.code, res.requests)
	}
}
