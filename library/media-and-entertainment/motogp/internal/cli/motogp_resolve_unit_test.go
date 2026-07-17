// Copyright 2026 waterpig and contributors. Licensed under Apache-2.0.

package cli

import "testing"

func TestNormClass(t *testing.T) {
	cases := map[string]string{
		"":          "motogp",
		"MotoGP":    "motogp",
		"MotoGP™":   "motogp",
		"moto gp":   "motogp",
		"premier":   "motogp",
		"Moto2":     "moto2",
		"moto-2":    "moto2",
		"m3":        "moto3",
		"Moto3™":    "moto3",
		"unknownxx": "unknownxx",
	}
	for in, want := range cases {
		if got := normClass(in); got != want {
			t.Errorf("normClass(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSessionTypeMatch(t *testing.T) {
	cases := []struct {
		in       string
		wantType string
		wantNum  int
	}{
		{"", "RAC", 0},
		{"race", "RAC", 0},
		{"sprint", "SPR", 0},
		{"SPR", "SPR", 0},
		{"q", "Q", 0},
		{"qualifying", "Q", 0},
		{"q2", "Q", 2},
		{"fp1", "FP", 1},
		{"warmup", "WUP", 0},
	}
	for _, c := range cases {
		gotType, gotNum := sessionTypeMatch(c.in)
		if gotType != c.wantType || gotNum != c.wantNum {
			t.Errorf("sessionTypeMatch(%q) = (%q,%d), want (%q,%d)", c.in, gotType, gotNum, c.wantType, c.wantNum)
		}
	}
}

func TestParseYearArg(t *testing.T) {
	if y, err := parseYearArg("2024"); err != nil || y != 2024 {
		t.Errorf("parseYearArg(2024) = %d,%v", y, err)
	}
	for _, bad := range []string{"abc", "12", "1800", "3000", ""} {
		if _, err := parseYearArg(bad); err == nil {
			t.Errorf("parseYearArg(%q) expected error", bad)
		}
	}
}

func TestICSDate(t *testing.T) {
	if got := icsDate("2025-11-18T08:00:00+01:00"); got != "20251118" {
		t.Errorf("icsDate = %q, want 20251118", got)
	}
	if got := icsDate("short"); got != "" {
		t.Errorf("icsDate(short) = %q, want empty", got)
	}
}

func TestFullNamePrefersFlat(t *testing.T) {
	r := mgpRider{Name: "Marc", Surname: "Marquez", FullNameFlat: "Flat Name"}
	if got := r.fullName(); got != "Flat Name" {
		t.Errorf("fullName() = %q, want flat", got)
	}
	r2 := mgpRider{Name: "Marc", Surname: "Marquez"}
	if got := r2.fullName(); got != "Marc Marquez" {
		t.Errorf("fullName() = %q, want composed", got)
	}
}

func TestLeaderOf(t *testing.T) {
	pts := map[string]int{"A": 10, "B": 25, "C": 25}
	name, p := leaderOf(pts)
	if p != 25 || (name != "B" && name != "C") {
		t.Errorf("leaderOf = %q,%d", name, p)
	}
	// deterministic tie-break: alphabetical first among equal leaders
	if name != "B" {
		t.Errorf("leaderOf tie-break = %q, want B", name)
	}
	if n, p := leaderOf(map[string]int{}); n != "" || p != 0 {
		t.Errorf("leaderOf(empty) = %q,%d", n, p)
	}
}

func TestICSEscape(t *testing.T) {
	if got := icsEscape("a,b;c"); got != "a\\,b\\;c" {
		t.Errorf("icsEscape = %q", got)
	}
}

func TestBroadcastEventKind(t *testing.T) {
	gp := mgpBroadcastEvent{Kind: "GP"}
	if !gp.isGP() || gp.isTest() {
		t.Errorf("GP classification wrong")
	}
	test := mgpBroadcastEvent{Kind: "TEST"}
	if test.isGP() || !test.isTest() {
		t.Errorf("TEST classification wrong")
	}
	media := mgpBroadcastEvent{Kind: "MEDIA", Name: "TEAM PRESENTATION"}
	if media.isGP() || media.isTest() {
		t.Errorf("MEDIA should be neither GP nor TEST")
	}
}
