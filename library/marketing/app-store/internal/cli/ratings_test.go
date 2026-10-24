// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

func TestRatingsExactLookupCounts(t *testing.T) {
	f := newS2AFake(t)
	res := f.mustOK("ratings", "324684580", "--json")
	if res.requests != 1 {
		t.Fatalf("ratings sent %d requests, want 1 (Lookup only)", res.requests)
	}
	items, meta, failures := s2aItems(t, res.stdout)
	if len(items) != 1 || len(failures) != 0 {
		t.Fatalf("items %v failures %v", items, failures)
	}
	r := items[0]
	if r["storefront"] != "us" || s2aNum(t, r["ratingCount"]) != 42719156 || s2aNum(t, r["averageRating"]) < 4 || r["version"] != "9.1.92" {
		t.Fatalf("row = %v", r)
	}
	if _, err := time.Parse(time.RFC3339, r["fetchedAt"].(string)); err != nil {
		t.Fatalf("fetchedAt = %v", r["fetchedAt"])
	}
	if _, ok := r["histogram"]; ok {
		t.Fatal("histogram must only appear with --histogram")
	}
	if meta["app"] != "324684580" || meta["histogram"] != false {
		t.Fatalf("meta = %v", meta)
	}
	// The cached repeat is not a new capture.
	if res := f.mustOK("ratings", "324684580", "--json"); res.requests != 0 {
		t.Fatalf("repeat sent %d requests", res.requests)
	}
	series, err := store.RatingsSeries(context.Background(), f.openStore(), "324684580", "us", store.SourceLookup)
	if err != nil || len(series) != 1 || series[0].RatingCount != 42719156 {
		t.Fatalf("lookup ratings series = %+v (%v), want one snapshot", series, err)
	}
}

func TestRatingsHistogramAndStorefronts(t *testing.T) {
	f := newS2AFake(t)
	res := f.mustOK("ratings", "324684580", "--countries", "us,gb,cn", "--histogram", "--json")
	items, meta, _ := s2aItems(t, res.stdout)
	if len(items) != 2 || items[0]["storefront"] != "us" || items[1]["storefront"] != "gb" {
		t.Fatalf("items = %v", items)
	}
	if s2aNum(t, items[1]["ratingCount"]) != 1876543 {
		t.Fatalf("gb count = %v", items[1]["ratingCount"])
	}
	hist, _ := items[0]["histogram"].([]any)
	total := s2aNum(t, items[0]["histogramTotal"])
	var sum float64
	for _, h := range hist {
		sum += s2aNum(t, h)
	}
	if len(hist) != 5 || total <= 0 || sum < total*0.99 || sum > total*1.01 || s2aNum(t, hist[0]) <= s2aNum(t, hist[4]) {
		t.Fatalf("us histogram %v (sum %.0f) total %.0f: want 5 counts, 5-star first, summing to the page total", hist, sum, total)
	}
	unavailable, _ := meta["unavailable"].([]any)
	if len(unavailable) != 1 || unavailable[0] != "cn" {
		t.Fatalf("unavailable = %v", meta["unavailable"])
	}
	st := f.openStore()
	ctx := context.Background()
	for _, cc := range []string{"us", "gb"} {
		lookup, _ := store.RatingsSeries(ctx, st, "324684580", cc, store.SourceLookup)
		page, _ := store.RatingsSeries(ctx, st, "324684580", cc, store.SourcePage)
		if len(lookup) != 1 || len(page) != 1 || len(page[0].Histogram) != 5 {
			t.Fatalf("%s: lookup %d page %d snapshots", cc, len(lookup), len(page))
		}
	}
	if cn, _ := store.RatingsSeries(ctx, st, "324684580", "cn", ""); len(cn) != 0 {
		t.Fatalf("an unavailable storefront must not get a zero snapshot: %+v", cn)
	}
}

func TestRatingsFailuresNeverBecomeRows(t *testing.T) {
	f := newS2AFake(t)
	f.fail = func(host, path string, q url.Values) int {
		if q.Get("country") == "gb" {
			return http.StatusInternalServerError
		}
		return 0
	}
	res := f.mustOK("ratings", "324684580", "--countries", "us,gb", "--json")
	items, _, failures := s2aItems(t, res.stdout)
	if len(items) != 1 || items[0]["storefront"] != "us" || len(failures) != 1 || failures[0]["storefront"] != "gb" {
		t.Fatalf("items %v failures %v", items, failures)
	}
	if !strings.Contains(res.stderr, "warning: 1 of 2 fetches failed") {
		t.Fatalf("stderr = %q", res.stderr)
	}

	f.fail = func(host, path string, q url.Values) int { return http.StatusInternalServerError }
	if res := f.run("ratings", "553834731", "--countries", "us,gb", "--json"); res.code != 5 {
		t.Fatalf("every storefront failing exit = %d, want 5", res.code)
	}
	f.fail = func(host, path string, q url.Values) int { return http.StatusForbidden }
	if res := f.run("ratings", "553834731", "--json"); res.code != 7 {
		t.Fatalf("403 exit = %d, want 7", res.code)
	}
}

func TestRatingsNotFound(t *testing.T) {
	f := newS2AFake(t)
	if res := f.run("ratings", "1", "--json"); res.code != 3 {
		t.Fatalf("ratings 1 exit = %d, want 3", res.code)
	}
	if res := f.run("ratings", "324684580", "--countries", "cn", "--json"); res.code != 3 {
		t.Fatalf("not sold in the only storefront exit = %d, want 3", res.code)
	}
}

// A storefront whose product page fails keeps its Lookup row (exact counts
// and their snapshot); only the histogram is missing, named in fetch_failures.
func TestRatingsKeepsTheLookupRowWhenThePageFails(t *testing.T) {
	f := newS2AFake(t)
	f.fail = func(host, path string, q url.Values) int {
		if host == "apps.apple.com" && strings.HasPrefix(path, "/gb/") {
			return http.StatusBadGateway
		}
		return 0
	}
	res := f.run("ratings", "324684580", "--countries", "us,gb", "--histogram", "--json")
	if res.code != 0 {
		t.Fatalf("exit %d (%v), want 0", res.code, res.err)
	}
	if strings.Count(res.stdout, `"ratingCount"`) != 2 || !strings.Contains(res.stdout, `"error": "histogram: `) {
		t.Fatalf("want both storefronts' rows and a histogram failure for gb:\n%s", res.stdout)
	}
}
