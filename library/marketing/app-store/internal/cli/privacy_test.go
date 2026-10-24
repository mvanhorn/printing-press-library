// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

func TestPrivacySpotifyLabels(t *testing.T) {
	f := newS2AFake(t)
	res := f.mustOK("privacy", "324684580", "--json")
	v := s2aJSONObject(t, res.stdout)
	if v["app"] != "324684580" || v["storefront"] != "us" || !strings.HasPrefix(v["title"].(string), "Spotify") {
		t.Fatalf("privacy header = %v", v)
	}
	if u, _ := v["policyUrl"].(string); !strings.HasPrefix(u, "http") {
		t.Fatalf("policyUrl = %q", u)
	}
	types, _ := v["privacyTypes"].([]any)
	if len(types) == 0 {
		t.Fatalf("privacyTypes empty: %s", res.stdout)
	}
	dataTypes := 0
	for _, raw := range types {
		pt := raw.(map[string]any)
		if pt["identifier"] == "" || pt["title"] == "" {
			t.Fatalf("privacy type without identifier/title: %v", pt)
		}
		cats, _ := pt["categories"].([]any)
		for _, c := range cats {
			cat := c.(map[string]any)
			if cat["title"] == "" {
				t.Fatalf("category without title: %v", cat)
			}
			dts, _ := cat["dataTypes"].([]any)
			dataTypes += len(dts)
		}
	}
	if dataTypes == 0 {
		t.Fatal("no data types under any category")
	}
	if _, ok := v["note"]; ok {
		t.Fatalf("labels present: no note expected, got %v", v["note"])
	}
	// A cached repeat records nothing new; the privacy history has one version.
	if res := f.mustOK("privacy", "324684580", "--json"); res.requests != 0 {
		t.Fatalf("repeat sent %d requests", res.requests)
	}
	hist, err := store.PrivacyHistory(context.Background(), f.openStore(), "324684580", "us")
	if err != nil || len(hist) != 1 {
		t.Fatalf("privacy history = %d (%v), want 1", len(hist), err)
	}
}

func TestPrivacyNoLabelsIsHonestEmpty(t *testing.T) {
	f := newS2AFake(t)
	f.setPage("324684580", s2aMutatePage(t, "product_spotify_us.html", func(sm map[string]any) {
		delete(sm, "privacyHeader")
		delete(sm, "privacyTypes")
	}))
	res := f.mustOK("privacy", "324684580", "--json")
	v := s2aJSONObject(t, res.stdout)
	types, ok := v["privacyTypes"].([]any)
	if !ok || len(types) != 0 {
		t.Fatalf("privacyTypes = %v, want []", v["privacyTypes"])
	}
	if note, _ := v["note"].(string); !strings.Contains(note, "no privacy labels") || !strings.Contains(res.stderr, "note:") {
		t.Fatalf("missing honest note: %v / %s", v["note"], res.stderr)
	}
	hist, _ := store.PrivacyHistory(context.Background(), f.openStore(), "324684580", "us")
	if len(hist) != 0 {
		t.Fatalf("an empty label set must not be stored as a privacy version: %d", len(hist))
	}
}

func TestPrivacyNegative(t *testing.T) {
	f := newS2AFake(t)
	if res := f.run("privacy", "1", "--json"); res.code != 3 {
		t.Fatalf("privacy 1 exit = %d, want 3", res.code)
	}
	if res := f.run("privacy", "--json"); res.code != 2 || res.requests != 0 {
		t.Fatalf("privacy without app exit = %d requests %d, want 2/0", res.code, res.requests)
	}
}
