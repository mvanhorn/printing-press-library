package cli

import (
	"testing"
	"time"
)

func TestSplitNegate(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"furnished", []string{"furnished"}},
		{"furnished, sublet ,STUDIO", []string{"furnished", "sublet", "studio"}},
		{",,,", nil},
	}
	for _, tc := range cases {
		got := splitNegate(tc.in)
		if !equalStringSlices(got, tc.want) {
			t.Errorf("splitNegate(%q): got %v want %v", tc.in, got, tc.want)
		}
	}
}

func TestApplyNegate_dropsMatches(t *testing.T) {
	hits := []searchHit{
		{Title: "1BR Furnished Apartment"},
		{Title: "Studio in SOMA"},
		{Title: "Plain 1BR"},
	}
	out := applyNegate(hits, "furnished,studio")
	if len(out) != 1 {
		t.Fatalf("expected 1 hit, got %d: %+v", len(out), out)
	}
	if out[0].Title != "Plain 1BR" {
		t.Errorf("expected 'Plain 1BR', got %+v", out)
	}
}

func TestApplyNegate_emptyNoOp(t *testing.T) {
	hits := []searchHit{{Title: "anything"}}
	out := applyNegate(hits, "")
	if len(out) != 1 {
		t.Errorf("empty negate should not drop anything; got %d", len(out))
	}
}

func TestApplyPostedSince(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	hits := []searchHit{
		{Title: "old", PostedAt: 999_000},   // 1000s old
		{Title: "fresh", PostedAt: 999_990}, // 10s old
		{Title: "no-time", PostedAt: 0},     // unset → kept
	}
	out := applyPostedSince(hits, 60*time.Second, now)
	titles := map[string]bool{}
	for _, h := range out {
		titles[h.Title] = true
	}
	if titles["old"] {
		t.Errorf("old listing should be dropped past 60s cutoff")
	}
	if !titles["fresh"] {
		t.Errorf("fresh listing should be kept")
	}
	if !titles["no-time"] {
		t.Errorf("listing with unknown postedAt should be kept (cannot prove stale)")
	}
}

func TestSiteList_sitesWinsOverSite(t *testing.T) {
	if got := siteList("sfbay", "nyc,la"); !equalStringSlices(got, []string{"nyc", "la"}) {
		t.Errorf("--sites should win, got %v", got)
	}
	if got := siteList("sfbay", ""); !equalStringSlices(got, []string{"sfbay"}) {
		t.Errorf("--site fallback, got %v", got)
	}
	if got := siteList("", ""); got != nil {
		t.Errorf("both empty should return nil, got %v", got)
	}
}

func TestParseDuration_daySuffix(t *testing.T) {
	d, err := parseDuration("3d")
	if err != nil {
		t.Fatalf("parse 3d: %v", err)
	}
	if d != 3*24*time.Hour {
		t.Errorf("3d: got %v want 72h", d)
	}
	if d, _ := parseDuration("24h"); d != 24*time.Hour {
		t.Errorf("24h: got %v want 24h", d)
	}
	if d, _ := parseDuration(""); d != 0 {
		t.Errorf("empty: got %v want 0", d)
	}
	if _, err := parseDuration("abc"); err == nil {
		t.Errorf("invalid duration should error")
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
