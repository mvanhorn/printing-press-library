// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

func TestCompareAlignedRows(t *testing.T) {
	f := newS2AFake(t)
	res := f.mustOK("compare", "324684580", "389801252", "--json")
	if res.requests != 1 {
		t.Fatalf("compare sent %d requests, want 1 batched Lookup", res.requests)
	}
	items, meta, failures := s2aItems(t, res.stdout)
	if len(items) != 2 || len(failures) != 0 {
		t.Fatalf("items %v failures %v", items, failures)
	}
	if items[0]["app"] != "324684580" || items[1]["app"] != "389801252" {
		t.Fatalf("rows must keep argument order: %v / %v", items[0]["app"], items[1]["app"])
	}
	if s2aNum(t, items[0]["ratingCount"]) != 42719156 || s2aNum(t, items[1]["ratingCount"]) != 29628149 {
		t.Fatalf("exact counts = %v / %v", items[0]["ratingCount"], items[1]["ratingCount"])
	}
	if len(items[0]) != len(items[1]) {
		t.Fatalf("rows are not aligned: %d vs %d keys", len(items[0]), len(items[1]))
	}
	for k := range items[0] {
		if _, ok := items[1][k]; !ok {
			t.Fatalf("key %q missing from the second row", k)
		}
	}
	if s2aNum(t, items[0]["sizeBytes"]) <= 0 || items[0]["developer"] == "" {
		t.Fatalf("row = %v", items[0])
	}
	if un, _ := meta["unavailable"].([]any); len(un) != 0 {
		t.Fatalf("unavailable = %v", un)
	}
	series, _ := store.RatingsSeries(context.Background(), f.openStore(), "389801252", "us", store.SourceLookup)
	if len(series) != 1 {
		t.Fatalf("compare should record a lookup ratings snapshot per app: %d", len(series))
	}
}

func TestCompareStorefrontsAndUnavailable(t *testing.T) {
	f := newS2AFake(t)
	items, meta, _ := s2aItems(t, f.mustOK("compare", "324684580", "553834731", "--countries", "us,gb,cn", "--json").stdout)
	if len(items) != 4 {
		t.Fatalf("items = %d, want 4 (us and gb only)", len(items))
	}
	order := []string{}
	for _, it := range items {
		order = append(order, it["storefront"].(string)+"/"+it["app"].(string))
	}
	if strings.Join(order, ",") != "us/324684580,us/553834731,gb/324684580,gb/553834731" {
		t.Fatalf("order = %v", order)
	}
	un, _ := meta["unavailable"].([]any)
	if len(un) != 2 {
		t.Fatalf("cn pairs should be listed as unavailable, never as rows: %v", meta["unavailable"])
	}
	items, meta, _ = s2aItems(t, f.mustOK("compare", "324684580", "1", "--json").stdout)
	if len(items) != 1 || items[0]["app"] != "324684580" {
		t.Fatalf("items = %v", items)
	}
	if un, _ := meta["unavailable"].([]any); len(un) != 1 {
		t.Fatalf("unavailable = %v", meta["unavailable"])
	}
}

func TestCompareNegative(t *testing.T) {
	f := newS2AFake(t)
	for _, args := range [][]string{
		{"compare", "324684580", "--json"},
		{"compare", "324684580", "324684580", "--json"},
		{"compare", "--json"},
		{"compare", "324684580", "not an app"},
	} {
		if res := f.run(args...); res.code != 2 || res.requests != 0 {
			t.Errorf("%v exit %d requests %d, want 2/0", args, res.code, res.requests)
		}
	}
	if res := f.run("compare", "1", "2", "--json"); res.code != 3 {
		t.Fatalf("compare 1 2 exit = %d, want 3", res.code)
	}
	res := f.mustOK("compare", "324684580", "389801252", "--human-friendly")
	if !strings.Contains(res.stdout, "FIELD") || !strings.Contains(res.stdout, "ratings") || !strings.Contains(res.stdout, "42,719,156") {
		t.Fatalf("human compare table:\n%s", res.stdout)
	}
}
