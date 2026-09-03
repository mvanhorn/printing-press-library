// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestSaveRate(t *testing.T) {
	cases := []struct {
		name    string
		col, dl int
		want    float64
	}{
		{"normal", 100, 200, 0.5},
		{"zero downloads", 50, 0, 0},
		{"more saves than downloads", 300, 100, 3},
	}
	for _, c := range cases {
		if got := saveRate(c.col, c.dl); got != c.want {
			t.Errorf("%s: saveRate(%d,%d)=%v want %v", c.name, c.col, c.dl, got, c.want)
		}
	}
}

func TestLessBySort(t *testing.T) {
	a := designRow{DesignScore: 10, Download: 100, HotScore: 5, Collection: 50}
	b := designRow{DesignScore: 8, Download: 200, HotScore: 9, Collection: 10}
	cases := []struct {
		mode   string
		aFirst bool
	}{
		{"quality", true},  // a.DesignScore 10 > 8
		{"popular", false}, // a.Download 100 < 200
		{"hot", false},     // a.HotScore 5 < 9
		{"saves", true},    // a 50/100=0.5 > b 10/200=0.05
	}
	for _, c := range cases {
		if got := lessBySort(a, b, c.mode); got != c.aFirst {
			t.Errorf("mode %s: lessBySort=%v want %v", c.mode, got, c.aFirst)
		}
	}
}

func TestToDiscoverItem(t *testing.T) {
	r := designRow{ID: "123", Title: "X", CreatorName: "C", Collection: 10, Download: 20}
	it := toDiscoverItem(r)
	if it.ID != "123" || it.Title != "X" || it.Creator != "C" {
		t.Fatalf("unexpected item: %+v", it)
	}
	if it.SaveRate != 0.5 {
		t.Errorf("save rate = %v want 0.5", it.SaveRate)
	}
	if it.URL != "https://makerworld.com/en/models/123" {
		t.Errorf("url = %s", it.URL)
	}
}
