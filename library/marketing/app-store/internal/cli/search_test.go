// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

func TestSearchHelpWires(t *testing.T) {
	r := s3aRun(t, "search", "--help")
	if r.code != 0 || !strings.Contains(r.stdout, "search <query> [flags]") || !strings.Contains(r.stdout, "--type") {
		t.Fatalf("search --help: exit %d\n%s", r.code, r.stdout)
	}
}

func TestSearchReturnsOnlyMatchingRows(t *testing.T) {
	db := s3aSeededDB(t)
	var hits []searchHit
	s3aDecode(t, s3aRun(t, "search", "crash", "--db", db, "--json"), &hits)
	// Fixture: three us reviews and one gb review mention "crash"; no
	// listing title or subtitle does (Candy Crush is "crush").
	if len(hits) != 4 {
		t.Fatalf("hits = %d, want 4: %+v", len(hits), hits)
	}
	storefronts := map[string]int{}
	for _, h := range hits {
		text := strings.ToLower(h.Title + " " + h.Body)
		if h.Type != "review" || !strings.Contains(text, "crash") || h.AppID != "6448311069" || h.Rating == nil || *h.Rating > 2 {
			t.Fatalf("non-matching or wrong hit: %+v", h)
		}
		storefronts[h.Storefront]++
	}
	if storefronts["us"] != 3 || storefronts["gb"] != 1 {
		t.Fatalf("storefronts = %v", storefronts)
	}

	hits = nil
	s3aDecode(t, s3aRun(t, "search", "spotify", "--db", db, "--json"), &hits)
	if len(hits) != 1 || hits[0].Type != "app" || hits[0].AppID != "324684580" || hits[0].Storefront != "us" || hits[0].CapturedAt == nil {
		t.Fatalf("listing search = %+v", hits)
	}
	if hits[0].Version != "9.1.92" {
		t.Fatalf("listing hit must come from the latest snapshot, got version %s", hits[0].Version)
	}
}

func TestSearchFilters(t *testing.T) {
	db := s3aSeededDB(t)
	var hits []searchHit
	s3aDecode(t, s3aRun(t, "search", "crash", "--storefront", "gb", "--db", db, "--json"), &hits)
	if len(hits) != 1 || hits[0].Storefront != "gb" {
		t.Fatalf("--storefront gb = %+v", hits)
	}
	hits = nil
	s3aDecode(t, s3aRun(t, "search", "login", "--type", "reviews", "--app", "6448311069", "--db", db, "--json"), &hits)
	if len(hits) != 3 {
		t.Fatalf("login reviews = %+v", hits)
	}
	for _, h := range hits {
		if !strings.Contains(strings.ToLower(h.Title+" "+h.Body), "login") {
			t.Fatalf("hit without the query: %+v", h)
		}
	}
	hits = nil
	s3aDecode(t, s3aRun(t, "search", "crash", "--type", "apps", "--db", db, "--json"), &hits)
	if len(hits) != 0 {
		t.Fatalf("--type apps must not return reviews: %+v", hits)
	}
	hits = nil
	s3aDecode(t, s3aRun(t, "search", "crash", "--app", "324684580", "--db", db, "--json"), &hits)
	if len(hits) != 0 {
		t.Fatalf("--app filter leaked other apps: %+v", hits)
	}
	hits = nil
	s3aDecode(t, s3aRun(t, "search", "candy", "--app", "553834731", "--db", db, "--json"), &hits)
	if len(hits) != 1 || hits[0].AppID != "553834731" {
		t.Fatalf("--app on listings = %+v", hits)
	}
	hits = nil
	s3aDecode(t, s3aRun(t, "search", "crash", "--limit", "2", "--db", db, "--json"), &hits)
	if len(hits) != 2 {
		t.Fatalf("--limit 2 = %d hits", len(hits))
	}
}

func TestSearchNoMatchAndEmpty(t *testing.T) {
	db := s3aSeededDB(t)
	r := s3aRun(t, "search", "zzqxv", "--db", db, "--json")
	if r.code != 0 || strings.TrimSpace(r.stdout) != "[]" || !strings.Contains(r.stderr, `no stored app listings or reviews match "zzqxv"`) {
		t.Fatalf("no match: exit %d stdout %q stderr %q", r.code, r.stdout, r.stderr)
	}
	r = s3aRun(t, "search", "crash", "--db", t.TempDir()+"/absent.db", "--json")
	if r.code != 0 || strings.TrimSpace(r.stdout) != "[]" || !strings.Contains(r.stderr, "no local mirror at") {
		t.Fatalf("missing mirror: exit %d stdout %q stderr %q", r.code, r.stdout, r.stderr)
	}
	r = s3aRun(t, "search", "crash", "--db", s3aEmptyDB(t), "--json")
	if r.code != 0 || strings.TrimSpace(r.stdout) != "[]" {
		t.Fatalf("empty store: exit %d stdout %q", r.code, r.stdout)
	}
	for _, args := range [][]string{
		{"search", "--json"},
		{"search", "crash", "--type", "users"},
		{"search", "crash", "--storefront", "zz"},
		{"search", "crash", "--limit", "0"},
	} {
		if r := s3aRun(t, args...); r.code != 2 {
			t.Errorf("%v: exit %d, want 2 (%v)", args, r.code, r.err)
		}
	}
}

func TestSearchSnippet(t *testing.T) {
	long := strings.Repeat("filler words here ", 20) + "the checkout CRASHES every time " + strings.Repeat("more filler ", 20)
	s := s3aSnippet(long, "crashes", 60)
	if !strings.Contains(s, "CRASHES") || !strings.HasPrefix(s, "…") || !strings.HasSuffix(s, "…") {
		t.Fatalf("snippet = %q", s)
	}
	if got := s3aSnippet("short body", "x", 60); got != "short body" {
		t.Fatalf("short snippet = %q", got)
	}
}
