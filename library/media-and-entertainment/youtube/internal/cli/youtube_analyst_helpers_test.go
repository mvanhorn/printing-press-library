// Copyright 2026 Justin and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"testing"
)

func TestParseISODurationSeconds(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"PT1H2M3S", 3723},
		{"PT15M33S", 933},
		{"PT45S", 45},
		{"PT2H", 7200},
		{"P1DT2H", 93600},
		{"PT0S", 0},
		{"", 0},
		{"garbage", 0},
	}
	for _, c := range cases {
		if got := parseISODurationSeconds(c.in); got != c.want {
			t.Errorf("parseISODurationSeconds(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestIsShortDuration(t *testing.T) {
	cases := []struct {
		in   int64
		want bool
	}{{60, true}, {180, true}, {181, false}, {0, false}, {600, false}}
	for _, c := range cases {
		if got := isShortDuration(c.in); got != c.want {
			t.Errorf("isShortDuration(%d) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseCount(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{{"12345", 12345}, {" 7 ", 7}, {"", 0}, {"n/a", 0}}
	for _, c := range cases {
		if got := parseCount(c.in); got != c.want {
			t.Errorf("parseCount(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestNextRingKey(t *testing.T) {
	ring := &keyRing{Active: "b", Keys: map[string]string{"a": "1", "b": "2", "c": "3"}, Order: []string{"a", "b", "c"}}
	if got := nextRingKey(ring); got != "c" {
		t.Errorf("nextRingKey after b = %q, want c", got)
	}
	ring.Active = "c"
	if got := nextRingKey(ring); got != "a" {
		t.Errorf("nextRingKey wraps to %q, want a", got)
	}
	single := &keyRing{Active: "only", Keys: map[string]string{"only": "1"}}
	if got := nextRingKey(single); got != "" {
		t.Errorf("nextRingKey single-key ring = %q, want empty", got)
	}
}

func TestMaskKey(t *testing.T) {
	if got := maskKey("AIzaSyABCDEFGH1234"); got != "AIza…1234" {
		t.Errorf("maskKey long = %q", got)
	}
	if got := maskKey("short"); got != "****" {
		t.Errorf("maskKey short = %q, want ****", got)
	}
}

func TestIsQuotaExhausted(t *testing.T) {
	if !isQuotaExhausted(errors.New(`GET /youtube/v3/search returned HTTP 403: {"error":{"errors":[{"reason":"quotaExceeded"}]}}`)) {
		t.Error("quotaExceeded not detected")
	}
	if isQuotaExhausted(errors.New("HTTP 404 not found")) {
		t.Error("404 misdetected as quota exhaustion")
	}
	if isQuotaExhausted(nil) {
		t.Error("nil misdetected")
	}
}

func TestDedupeStrings(t *testing.T) {
	got := dedupeStrings([]string{"a", "b", "a", "c", "b"})
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("dedupeStrings = %v", got)
	}
}

func TestSortVelocityDesc(t *testing.T) {
	items := []velocityRow{{ViewsPerDay: 1}, {ViewsPerDay: 9}, {ViewsPerDay: 5}}
	sortVelocityDesc(items)
	if items[0].ViewsPerDay != 9 || items[1].ViewsPerDay != 5 || items[2].ViewsPerDay != 1 {
		t.Errorf("sortVelocityDesc order = %v", items)
	}
}

func TestHookFromSegments(t *testing.T) {
	tr := &transcriptResult{Segments: []transcriptSegment{
		{StartMs: 0, Text: "welcome back"},
		{StartMs: 20000, Text: "today we test"},
		{StartMs: 60000, Text: "later material"},
	}}
	got := hookFromSegments(tr, 45000)
	if got != "welcome back today we test" {
		t.Errorf("hookFromSegments = %q", got)
	}
	if hookFromSegments(nil, 45000) != "" {
		t.Error("nil transcript should produce empty hook")
	}
}

func TestTruncateStr(t *testing.T) {
	if got := truncateStr("hello world", 5); len([]rune(got)) > 5+1 {
		t.Errorf("truncateStr too long: %q", got)
	}
	if got := truncateStr("hi", 10); got != "hi" {
		t.Errorf("truncateStr short = %q", got)
	}
}

func TestBestThumbURL(t *testing.T) {
	thumbs := map[string]struct {
		URL string `json:"url"`
	}{
		"default": {URL: "d"},
		"high":    {URL: "h"},
	}
	if got := bestThumbURL(thumbs); got != "h" {
		t.Errorf("bestThumbURL = %q, want h", got)
	}
	if got := bestThumbURL(nil); got != "" {
		t.Errorf("bestThumbURL(nil) = %q", got)
	}
}
