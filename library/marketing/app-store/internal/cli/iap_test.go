// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

func TestIAPCandyCrushTenDollarRows(t *testing.T) {
	f := newS2AFake(t)
	res := f.mustOK("iap", "553834731", "--json")
	items, meta, failures := s2aItems(t, res.stdout)
	if len(items) != 10 {
		t.Fatalf("iap rows = %d, want 10", len(items))
	}
	for i, it := range items {
		price, _ := it["price"].(string)
		if !strings.HasPrefix(price, "$") || it["storefront"] != "us" || it["currency"] != "USD" || s2aNum(t, it["position"]) != float64(i+1) {
			t.Fatalf("row %d = %v", i, it)
		}
		if name, _ := it["name"].(string); name == "" {
			t.Fatalf("row %d has no name", i)
		}
	}
	if len(failures) != 0 || meta["app"] != "553834731" || meta["title"] != "Candy Crush Saga" {
		t.Fatalf("meta %v failures %v", meta, failures)
	}
	sfs, _ := meta["storefronts"].([]any)
	if len(sfs) != 1 || sfs[0].(map[string]any)["iapCount"].(float64) != 10 {
		t.Fatalf("meta.storefronts = %v", meta["storefronts"])
	}
	// The unchanged list is stored once even when fetched again.
	f.mustOK("iap", "553834731", "--json", "--no-cache")
	hist, err := store.IAPHistory(context.Background(), f.openStore(), "553834731", "us")
	if err != nil || len(hist) != 1 || len(hist[0].Items) != 10 {
		t.Fatalf("IAP history = %+v (%v), want one 10-row list", hist, err)
	}
}

func TestIAPNoListAndUnavailableStorefront(t *testing.T) {
	f := newS2AFake(t)
	res := f.mustOK("iap", "324684580", "--json")
	items, meta, _ := s2aItems(t, res.stdout)
	notes, _ := meta["notes"].([]any)
	if len(items) != 0 || len(notes) == 0 || !strings.Contains(notes[0].(string), "lists no in-app purchase prices") {
		t.Fatalf("Spotify (no IAP rows) items %v notes %v", items, notes)
	}
	res = f.mustOK("iap", "553834731", "--countries", "us,cn", "--json")
	items, meta, failures := s2aItems(t, res.stdout)
	unavailable, _ := meta["unavailable"].([]any)
	if len(items) != 10 || len(unavailable) != 1 || unavailable[0] != "cn" || len(failures) != 0 {
		t.Fatalf("us,cn: items %d unavailable %v failures %v", len(items), unavailable, failures)
	}
	for _, it := range items {
		if it["storefront"] != "us" {
			t.Fatalf("a storefront where the app is not sold produced a row: %v", it)
		}
	}
}

func TestIAPPartialFailureAndThrottle(t *testing.T) {
	f := newS2AFake(t)
	f.fail = func(host, path string, q url.Values) int {
		if host == "apps.apple.com" && strings.HasPrefix(path, "/gb/") {
			return http.StatusInternalServerError
		}
		return 0
	}
	res := f.mustOK("iap", "553834731", "--countries", "us,gb", "--json")
	items, _, failures := s2aItems(t, res.stdout)
	if len(items) != 10 || len(failures) != 1 || failures[0]["storefront"] != "gb" {
		t.Fatalf("partial: items %d failures %v", len(items), failures)
	}
	if !strings.Contains(res.stderr, "warning: 1 of 2 fetches failed") {
		t.Fatalf("missing partial-failure warning: %q", res.stderr)
	}

	f2 := newS2AFake(t)
	f2.fail = func(host, path string, q url.Values) int {
		if q.Get("country") == "gb" {
			return http.StatusTooManyRequests
		}
		return 0
	}
	res = f2.run("iap", "553834731", "--countries", "us,gb,jp", "--json")
	// The us rows gathered before the throttle still print; gb carries the
	// throttle and jp is listed as not attempted; exit 7 says it is partial.
	if res.code != 7 || !strings.Contains(res.stdout, `"storefront": "us"`) ||
		!strings.Contains(res.stdout, "rate limited by Apple") || !strings.Contains(res.stdout, `not attempted: the run stopped at gb`) {
		t.Fatalf("throttle mid-fan-out exit %d stdout %q, want 7 with the us rows and the gb/jp failures", res.code, res.stdout)
	}
	for _, r := range f2.requestLog() {
		if strings.Contains(r, "country=jp") {
			t.Fatalf("the run continued after the throttle: %v", f2.requestLog())
		}
	}
}

func TestIAPNotFoundAndDogfoodCap(t *testing.T) {
	f := newS2AFake(t)
	if res := f.run("iap", "1", "--json"); res.code != 3 {
		t.Fatalf("iap 1 exit = %d, want 3", res.code)
	}
	if res := f.run("iap", "1", "--countries", "us,gb", "--json"); res.code != 3 {
		t.Fatalf("iap 1 in us,gb exit = %d, want 3", res.code)
	}
	if res := f.run("iap", "553834731", "--countries", "us,zz"); res.code != 2 || res.requests != 0 {
		t.Fatalf("bad --countries exit %d requests %d", res.code, res.requests)
	}
	t.Setenv(cliutil.DogfoodEnvVar, "1")
	res := f.mustOK("iap", "553834731", "--countries", "us,gb,cn", "--json")
	_, meta, _ := s2aItems(t, res.stdout)
	if sfs, _ := meta["storefronts"].([]any); len(sfs) != dogfoodMaxCountries {
		t.Fatalf("dogfood storefronts = %v, want %d", meta["storefronts"], dogfoodMaxCountries)
	}
}
