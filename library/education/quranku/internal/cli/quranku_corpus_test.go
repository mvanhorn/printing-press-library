// Copyright 2026 erikgunawans and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the hand-authored QuranKu corpus helpers.

package cli

import (
	"fmt"
	"testing"
)

func TestQkParseRef(t *testing.T) {
	cases := []struct {
		in           string
		surah, verse int
		ok           bool
	}{
		{"2:255", 2, 255, true},
		{"1:1", 1, 1, true},
		{" 114:6 ", 114, 6, true},
		{"0:1", 0, 0, false},
		{"115:1", 0, 0, false},
		{"2:0", 0, 0, false},
		{"2", 0, 0, false},
		{"a:b", 0, 0, false},
		{"", 0, 0, false},
	}
	for _, c := range cases {
		s, v, ok := qkParseRef(c.in)
		if ok != c.ok || (ok && (s != c.surah || v != c.verse)) {
			t.Errorf("qkParseRef(%q) = (%d,%d,%v), want (%d,%d,%v)", c.in, s, v, ok, c.surah, c.verse, c.ok)
		}
	}
}

func TestQkGlobalToRefAndTotal(t *testing.T) {
	metas := []qkSurahMeta{
		{ID: 1, Name: "Al-Fatihah", NumberOfVerses: 7},
		{ID: 2, Name: "Al-Baqarah", NumberOfVerses: 286},
		{ID: 3, Name: "Ali 'Imran", NumberOfVerses: 200},
	}
	if got := qkTotalVerses(metas); got != 493 {
		t.Fatalf("qkTotalVerses = %d, want 493", got)
	}
	cases := []struct {
		global int
		want   string
	}{
		{1, "1:1"},
		{7, "1:7"},
		{8, "2:1"},
		{293, "2:286"},
		{294, "3:1"},
		{493, "3:200"},
		{494, ""}, // beyond corpus
	}
	for _, c := range cases {
		if got := qkGlobalToRef(metas, c.global); got != c.want {
			t.Errorf("qkGlobalToRef(%d) = %q, want %q", c.global, got, c.want)
		}
	}
}

func TestSortVerses(t *testing.T) {
	vs := []qkVerse{
		{Surah: 2, Verse: 5},
		{Surah: 1, Verse: 3},
		{Surah: 1, Verse: 1},
		{Surah: 2, Verse: 1},
	}
	sortVerses(vs)
	want := []string{"1:1", "1:3", "2:1", "2:5"}
	for i, v := range vs {
		got := fmt.Sprintf("%d:%d", v.Surah, v.Verse)
		if got != want[i] {
			t.Errorf("index %d = %q, want %q", i, got, want[i])
		}
	}
}
