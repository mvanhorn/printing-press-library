// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

func TestVersionsSpotify25NewestFirst(t *testing.T) {
	f := newS2AFake(t)
	rows := s2aJSONArray(t, f.mustOK("versions", "324684580", "--limit", "25", "--json").stdout)
	if len(rows) != 25 {
		t.Fatalf("versions = %d rows, want 25", len(rows))
	}
	var prev time.Time
	for i, r := range rows {
		ts, _ := r["releasedAt"].(string)
		at, err := time.Parse(time.RFC3339, ts)
		if err != nil || !strings.HasSuffix(ts, "Z") {
			t.Fatalf("row %d releasedAt %q is not RFC3339 UTC: %v", i, ts, err)
		}
		if i > 0 && at.After(prev) {
			t.Fatalf("row %d (%s) is newer than row %d: not descending", i, ts, i-1)
		}
		prev = at
		if v, _ := r["version"].(string); v == "" {
			t.Fatalf("row %d has no version", i)
		}
	}
	if rows[0]["version"] != "9.1.92" || rows[0]["releasedAt"] != "2026-10-19T13:57:56Z" {
		t.Fatalf("newest = %v", rows[0])
	}
	stored, err := store.VersionEntries(context.Background(), f.openStore(), "324684580")
	if err != nil || len(stored) != 25 {
		t.Fatalf("stored versions = %d (%v), want 25", len(stored), err)
	}
}

func TestVersionsLimitAndBounds(t *testing.T) {
	f := newS2AFake(t)
	all := s2aJSONArray(t, f.mustOK("versions", "324684580", "--json").stdout)
	five := s2aJSONArray(t, f.mustOK("versions", "324684580", "--limit", "5", "--json").stdout)
	if len(all) != 25 || len(five) != 5 {
		t.Fatalf("default %d rows, --limit 5 %d rows", len(all), len(five))
	}
	for i := range five {
		if five[i]["version"] != all[i]["version"] {
			t.Fatalf("--limit 5 must be the 5 newest: %v vs %v", five[i], all[i])
		}
	}
	for _, bad := range []string{"26", "-1"} {
		if res := f.run("versions", "324684580", "--limit", bad); res.code != 2 || res.requests != 0 {
			t.Fatalf("--limit %s exit %d requests %d, want 2/0", bad, res.code, res.requests)
		}
	}
}

func TestVersionsNoHistoryIsEmptyWithNote(t *testing.T) {
	f := newS2AFake(t)
	f.setPage("324684580", s2aMutatePage(t, "product_spotify_us.html", func(sm map[string]any) {
		delete(sm, "mostRecentVersion")
	}))
	res := f.mustOK("versions", "324684580", "--json")
	if strings.TrimSpace(res.stdout) != "[]" {
		t.Fatalf("no version shelf should print [], got %s", res.stdout)
	}
	if !strings.Contains(res.stderr, "note: the us product page lists no version history") {
		t.Fatalf("missing note on stderr: %q", res.stderr)
	}
	if res := f.run("versions", "1"); res.code != 3 {
		t.Fatalf("versions 1 exit = %d, want 3", res.code)
	}
}
