// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"math"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

func TestAppSpotifyMergesLookupAndPage(t *testing.T) {
	f := newS2AFake(t)
	res := f.mustOK("app", "324684580", "--json")
	if res.requests != 2 {
		t.Fatalf("app sent %d requests, want 2 (Lookup + canonical page): %v", res.requests, f.requestLog())
	}
	v := s2aJSONObject(t, res.stdout)
	if name, _ := v["trackName"].(string); !strings.HasPrefix(name, "Spotify") {
		t.Fatalf("trackName = %q", name)
	}
	if n := s2aNum(t, v["ratingCount"]); n <= 1_000_000 || n != s2aNum(t, v["userRatingCount"]) {
		t.Fatalf("ratingCount = %v userRatingCount = %v", v["ratingCount"], v["userRatingCount"])
	}
	hist, _ := v["histogram"].([]any)
	if len(hist) != 5 {
		t.Fatalf("histogram = %v, want 5 ints", v["histogram"])
	}
	var sum float64
	for _, h := range hist {
		sum += s2aNum(t, h)
	}
	total := s2aNum(t, v["totalRatings"])
	if total <= 0 || math.Abs(sum-total)/total > 0.01 {
		t.Fatalf("histogram sum %.0f not within 1%% of totalRatings %.0f", sum, total)
	}
	if s2aNum(t, hist[0]) <= s2aNum(t, hist[4]) {
		t.Fatalf("histogram not 5-star first: %v", hist)
	}
	if sub, _ := v["subtitle"].(string); sub == "" {
		t.Fatal("subtitle missing")
	}
	cp, _ := v["chartPosition"].(map[string]any)
	if cp == nil || cp["chart"] != "top-free" || cp["genreId"] != "6011" || s2aNum(t, cp["position"]) < 1 {
		t.Fatalf("chartPosition = %v", v["chartPosition"])
	}
	if u, _ := v["url"].(string); u != "https://apps.apple.com/us/app/spotify-music-and-podcasts/id324684580" {
		t.Fatalf("canonical url = %q", u)
	}
	if v["storefront"] != "us" || v["bundleId"] != "com.spotify.client" {
		t.Fatalf("storefront/bundle = %v/%v", v["storefront"], v["bundleId"])
	}
	for _, key := range []string{"description", "versions", "privacy", "accessibility", "screenshots", "screenshotUrls"} {
		if _, ok := v[key]; ok {
			t.Fatalf("%q must only appear with --full", key)
		}
	}
	if _, ok := v["editorsChoice"].(bool); !ok {
		t.Fatalf("editorsChoice = %v", v["editorsChoice"])
	}

	st := f.openStore()
	ctx := context.Background()
	snaps, err := store.AppSnapshots(ctx, st, "324684580", "us", s2aFixedNow.AddDate(-10, 0, 0))
	if err != nil || len(snaps) != 1 || snaps[0].Source != store.SourcePageLookup || snaps[0].Subtitle == "" {
		t.Fatalf("app snapshots = %+v %v", snaps, err)
	}
	versions, _ := store.VersionEntries(ctx, st, "324684580")
	if len(versions) != 25 {
		t.Fatalf("version entries = %d, want 25", len(versions))
	}
	caps, _ := store.Captures(ctx, st, store.CaptureFilter{})
	if len(caps) != 2 {
		t.Fatalf("capture log rows = %d, want 2", len(caps))
	}
	for _, c := range caps {
		if c.Outcome != "ok" || c.Status != 200 {
			t.Fatalf("capture = %+v", c)
		}
	}
}

func TestAppFullAccessibilityDeclaredVsNot(t *testing.T) {
	f := newS2AFake(t)
	pages := s2aJSONObject(t, f.mustOK("app", "361309726", "--full", "--json").stdout)
	acc, _ := pages["accessibility"].(map[string]any)
	feats, _ := acc["features"].([]any)
	if acc == nil || acc["declared"] != true || len(feats) < 1 || feats[0] != "VoiceOver" {
		t.Fatalf("Pages accessibility = %v", pages["accessibility"])
	}
	spotify := s2aJSONObject(t, f.mustOK("app", "324684580", "--full", "--json").stdout)
	acc, _ = spotify["accessibility"].(map[string]any)
	feats, _ = acc["features"].([]any)
	if acc == nil || acc["declared"] != false || feats == nil || len(feats) != 0 {
		t.Fatalf("Spotify accessibility = %v, want declared=false with features []", spotify["accessibility"])
	}
	if d, _ := spotify["description"].(string); len(d) < 50 {
		t.Fatalf("--full description = %q", d)
	}
	if vs, _ := spotify["versions"].([]any); len(vs) != 25 {
		t.Fatalf("--full versions = %d, want 25", len(vs))
	}
	pv, _ := spotify["privacy"].(map[string]any)
	if types, _ := pv["privacyTypes"].([]any); len(types) == 0 {
		t.Fatalf("--full privacy = %v", spotify["privacy"])
	}
	// Spotify has no events and no IAP rows: the keys stay present as [].
	for _, key := range []string{"events", "inAppPurchases"} {
		list, ok := spotify[key].([]any)
		if !ok || len(list) != 0 {
			t.Fatalf("--full %s = %v, want []", key, spotify[key])
		}
	}
	if sim, _ := spotify["similar"].([]any); len(sim) != 10 {
		t.Fatalf("--full similar = %d, want 10", len(sim))
	}
	if shots, _ := spotify["screenshots"].([]any); len(shots) == 0 {
		t.Fatal("--full screenshots empty")
	}
}

func TestAppUnknownIDExits3(t *testing.T) {
	f := newS2AFake(t)
	res := f.run("app", "1", "--json")
	if res.code != 3 {
		t.Fatalf("app 1 exit = %d (%v), want 3", res.code, res.err)
	}
	if !strings.Contains(res.err.Error(), "no App Store app 1 in storefront us") {
		t.Fatalf("message = %q", res.err.Error())
	}
	if res.requests != 1 || strings.TrimSpace(res.stdout) != "" {
		t.Fatalf("app 1 sent %d requests (want only the Lookup) and printed %q", res.requests, res.stdout)
	}
}

func TestAppUsageErrorsSendNothing(t *testing.T) {
	f := newS2AFake(t)
	for _, args := range [][]string{
		{"app", "--json"},
		{"app", "not an app", "--json"},
		{"app", "324684580", "553834731"},
		{"app", "324684580", "--platform", "toaster"},
		{"app", "324684580", "--country", "zz"},
	} {
		res := f.run(args...)
		if res.code != 2 {
			t.Errorf("%v exit = %d (%v), want 2", args, res.code, res.err)
		}
		if res.requests != 0 {
			t.Errorf("%v sent %d requests", args, res.requests)
		}
	}
}

func TestAppDryRunAndHelp(t *testing.T) {
	f := newS2AFake(t)
	res := f.mustOK("app", "324684580", "--dry-run", "--json")
	if v := s2aJSONObject(t, res.stdout); v["dry_run"] != true || res.requests != 0 {
		t.Fatalf("dry-run = %s (requests %d)", res.stdout, res.requests)
	}
	res = f.mustOK("app")
	if !strings.Contains(res.stdout, "Usage:") || !strings.Contains(res.stdout, "app <app> [flags]") || res.requests != 0 {
		t.Fatalf("bare app should print help: %s", res.stdout)
	}
}

func TestAppCachedRepeatRecordsOnce(t *testing.T) {
	f := newS2AFake(t)
	f.mustOK("app", "324684580", "--json")
	res := f.mustOK("app", "324684580", "--json")
	if res.requests != 0 {
		t.Fatalf("repeat app sent %d requests, want 0 (local cache)", res.requests)
	}
	st := f.openStore()
	ctx := context.Background()
	snaps, _ := store.AppSnapshots(ctx, st, "324684580", "us", s2aFixedNow.AddDate(-10, 0, 0))
	pageRatings, _ := store.RatingsSeries(ctx, st, "324684580", "us", store.SourcePage)
	if len(snaps) != 1 || len(pageRatings) != 1 {
		t.Fatalf("a cached repeat must not duplicate the capture: snapshots %d, page ratings %d", len(snaps), len(pageRatings))
	}
	// --no-cache is a fresh capture and is recorded.
	res = f.mustOK("app", "324684580", "--json", "--no-cache")
	if res.requests != 2 {
		t.Fatalf("--no-cache sent %d requests, want 2", res.requests)
	}
}

func TestAppThrottleExits7(t *testing.T) {
	f := newS2AFake(t)
	f.fail = func(host, path string, q url.Values) int {
		if host == "apps.apple.com" {
			return http.StatusTooManyRequests
		}
		return 0
	}
	res := f.run("app", "324684580", "--json")
	if res.code != 7 || !strings.Contains(res.err.Error(), "rate limited by Apple") {
		t.Fatalf("throttled page exit = %d (%v), want 7", res.code, res.err)
	}
	// The Lookup fields already in hand still print, with the page error,
	// and the exit code says the run is incomplete.
	if !strings.Contains(res.stdout, `"trackName"`) || !strings.Contains(res.stdout, `"pageError"`) || !strings.Contains(res.stdout, "rate limited") {
		t.Fatalf("a page throttle must keep the Lookup fields and name the page error: %s", res.stdout)
	}
}

func TestAppPageFailureKeepsLookupFields(t *testing.T) {
	f := newS2AFake(t)
	f.fail = func(host, path string, q url.Values) int {
		if host == "apps.apple.com" {
			return http.StatusInternalServerError
		}
		return 0
	}
	res := f.mustOK("app", "324684580", "--json")
	v := s2aJSONObject(t, res.stdout)
	if v["trackName"] == nil || v["pageError"] == nil || !strings.Contains(res.stderr, "product page unavailable") {
		t.Fatalf("page failure should keep Lookup fields with pageError and a warning: %s / %s", res.stdout, res.stderr)
	}
	if hist, _ := v["histogram"].([]any); hist == nil || len(hist) != 0 {
		t.Fatalf("histogram without a page = %v, want [] (never invented)", v["histogram"])
	}
}

func TestAppBundleIDAndStoreURL(t *testing.T) {
	f := newS2AFake(t)
	v := s2aJSONObject(t, f.mustOK("app", "com.spotify.client", "--json").stdout)
	if s2aNum(t, v["trackId"]) != 324684580 {
		t.Fatalf("bundle id resolved to %v", v["trackId"])
	}
	v = s2aJSONObject(t, f.mustOK("app", "https://apps.apple.com/gb/app/spotify-music-and-podcasts/id324684580", "--json").stdout)
	if v["storefront"] != "gb" || v["currency"] != "GBP" {
		t.Fatalf("store URL storefront = %v currency = %v, want gb/GBP", v["storefront"], v["currency"])
	}
	v = s2aJSONObject(t, f.mustOK("app", "https://apps.apple.com/gb/app/spotify-music-and-podcasts/id324684580", "--country", "us", "--json").stdout)
	if v["storefront"] != "us" {
		t.Fatalf("explicit --country must win over the URL storefront: %v", v["storefront"])
	}
}

func TestAppHumanOutput(t *testing.T) {
	f := newS2AFake(t)
	res := f.mustOK("app", "324684580", "--full", "--human-friendly")
	for _, want := range []string{"Spotify: Music and Podcasts", "Histogram:", "5★", "Chart:", "Accessibility:", "not declared", "Recent versions:"} {
		if !strings.Contains(res.stdout, want) {
			t.Fatalf("human output missing %q:\n%s", want, res.stdout)
		}
	}
}
