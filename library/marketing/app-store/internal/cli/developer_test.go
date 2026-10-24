// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

func TestDeveloperByDeveloperID(t *testing.T) {
	f := newS2AFake(t)
	res := f.mustOK("developer", "324684583", "--json")
	if res.requests != 2 {
		t.Fatalf("developer id sent %d requests, want 2 (developer Lookup + page): %v", res.requests, f.requestLog())
	}
	v := s2aJSONObject(t, res.stdout)
	dev, _ := v["developer"].(map[string]any)
	if dev["id"] != "324684583" || dev["name"] != "Spotify" || dev["url"] != "https://apps.apple.com/us/developer/spotify/id324684583" {
		t.Fatalf("developer = %v", dev)
	}
	apps, _ := v["apps"].([]any)
	if len(apps) != 4 || s2aNum(t, v["appCount"]) != 4 {
		t.Fatalf("apps %d appCount %v, want 4", len(apps), v["appCount"])
	}
	first := apps[0].(map[string]any)
	if first["appId"] != "324684580" || s2aNum(t, first["ratingCount"]) != 42719156 {
		t.Fatalf("first app = %v", first)
	}
	shelves, _ := v["shelves"].([]any)
	if len(shelves) != 4 {
		t.Fatalf("shelves = %v", shelves)
	}
	titles := map[string]string{}
	for _, s := range shelves {
		sh := s.(map[string]any)
		titles[sh["title"].(string)] = sh["platform"].(string)
		ids, _ := sh["appIds"].([]any)
		if s2aNum(t, sh["count"]) != float64(len(ids)) || len(ids) == 0 {
			t.Fatalf("shelf count/ids mismatch: %v", sh)
		}
	}
	if titles["iPad & iPhone"] != "iphone" || titles["Apple TV"] != "appletv" || titles["Apple Watch"] != "watch" {
		t.Fatalf("shelf platforms = %v", titles)
	}
	lookupIDs := map[string]bool{}
	for _, a := range apps {
		lookupIDs[a.(map[string]any)["appId"].(string)] = true
	}
	pageOnly, _ := v["pageOnlyAppIds"].([]any)
	for _, id := range pageOnly {
		if lookupIDs[id.(string)] {
			t.Fatalf("pageOnlyAppIds lists %v, which the Lookup returned", id)
		}
	}
	// developer writes nothing but the capture log.
	st := f.openStore()
	if snaps, _ := store.AppSnapshots(context.Background(), st, "324684580", "", s2aFixedNow.AddDate(-10, 0, 0)); len(snaps) != 0 {
		t.Fatalf("developer wrote %d snapshots", len(snaps))
	}
}

func TestDeveloperFromAppReferences(t *testing.T) {
	f := newS2AFake(t)
	for _, arg := range []string{"324684580", "com.spotify.client", "https://apps.apple.com/us/app/spotify-music-and-podcasts/id324684580", "https://apps.apple.com/us/developer/spotify/id324684583"} {
		v := s2aJSONObject(t, f.mustOK("developer", arg, "--json", "--no-cache").stdout)
		if dev, _ := v["developer"].(map[string]any); dev["id"] != "324684583" {
			t.Fatalf("%s resolved to developer %v", arg, v["developer"])
		}
	}
	v := s2aJSONObject(t, f.mustOK("developer", "324684583", "--limit", "2", "--json").stdout)
	if apps, _ := v["apps"].([]any); len(apps) != 2 || s2aNum(t, v["appCount"]) != 4 {
		t.Fatalf("--limit 2: apps %v appCount %v", len(apps), v["appCount"])
	}
}

func TestDeveloperPlatformFilterAndNegative(t *testing.T) {
	f := newS2AFake(t)
	res := f.mustOK("developer", "324684583", "--platform", "mac", "--json")
	v := s2aJSONObject(t, res.stdout)
	apps, _ := v["apps"].([]any)
	shelves, _ := v["shelves"].([]any)
	if len(apps) != 0 || len(shelves) != 0 || v["note"] == nil {
		t.Fatalf("Spotify has no Mac apps in the fixtures: apps %v shelves %v note %v", apps, shelves, v["note"])
	}
	if res := f.run("developer", "324684583", "--platform", "watch"); res.code != 2 || res.requests != 0 {
		t.Fatalf("--platform watch exit %d requests %d", res.code, res.requests)
	}
	if res := f.run("developer", "1", "--json"); res.code != 3 {
		t.Fatalf("developer 1 exit = %d, want 3", res.code)
	}
	if res := f.run("developer", "324684583", "553834731"); res.code != 2 {
		t.Fatalf("two arguments exit = %d, want 2", res.code)
	}
	if res := f.run("developer", "not a developer"); res.code != 2 || res.requests != 0 {
		t.Fatalf("unparseable argument exit %d requests %d", res.code, res.requests)
	}
}
