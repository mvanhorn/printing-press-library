// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/coinlocker-navi/internal/locker"
)

func sp(s string) *string   { return &s }
func bp(b bool) *bool       { return &b }
func fp(f float64) *float64 { return &f }

func sizeL() []locker.Size { return []locker.Size{locker.NewSize("大", "700円/3個")} }

func TestApplyCheapFilters(t *testing.T) {
	win, _ := locker.ParseWindow("10:00-20:00")
	rows := []locker.Locker{
		{ID: "ecbo", WalkUp: bp(false)},
		{ID: "nosize", WalkUp: bp(true)},
		{ID: "unknownhours", WalkUp: bp(true), Sizes: sizeL(), SizesKnown: true, ICCard: bp(true), Hours: locker.ParseHours("情報なし")},
		{ID: "train", WalkUp: bp(true), Sizes: sizeL(), SizesKnown: true, ICCard: bp(true), Hours: locker.ParseHours("始発から終電")},
		{ID: "open", WalkUp: bp(true), Sizes: sizeL(), SizesKnown: true, ICCard: bp(true), Hours: locker.ParseHours("06時00分～22時00分")},
		{ID: "closed", WalkUp: bp(true), Sizes: sizeL(), SizesKnown: true, ICCard: bp(true), Hours: locker.ParseHours("09時00分～18時00分")},
		{ID: "cash", WalkUp: bp(true), Sizes: sizeL(), SizesKnown: true, ICCard: bp(false)},
	}
	ex := map[string]int{}
	got := applyCheapFilters(rows, nearFilters{size: "L", ic: true, walkUpOnly: true, window: &win}, ex)
	ids := []string{}
	for _, r := range got {
		ids = append(ids, r.ID)
	}
	want := []string{"open", "train", "unknownhours"}
	if len(ids) != 3 || ids[0] != want[0] || ids[1] != want[1] || ids[2] != want[2] {
		t.Fatalf("order = %v, want %v", ids, want)
	}
	for k, v := range map[string]int{"not_walk_up": 1, "size_unknown": 1, "closed_during_window": 1, "no_ic_card": 1} {
		if ex[k] != v {
			t.Errorf("excluded[%s] = %d, want %d (all: %v)", k, ex[k], v, ex)
		}
	}
	ex = map[string]int{}
	got = applyCheapFilters(rows[2:5], nearFilters{window: &win, strict: true}, ex)
	if len(got) != 1 || got[0].ID != "open" || ex["hours_not_confirmed"] != 2 {
		t.Fatalf("strict: got %d rows, excluded %v", len(got), ex)
	}
}

func TestFilterByGate(t *testing.T) {
	j := &liveJoin{checked: map[string]bool{"in": true, "out": true, "unk": true, "conf": true}}
	rows := []locker.Locker{
		{ID: "out", Gate: sp("outside"), Lat: fp(1)},
		{ID: "both", Gate: sp("both"), Lat: fp(1)},
		{ID: "in", Gate: sp("inside"), Lat: fp(1)},
		{ID: "unk", Lat: fp(1)},
		{ID: "far", Lat: fp(1)},
		{ID: "nocoord"},
		{ID: "conf", Lat: fp(1), GateConflict: sp("x")},
	}
	ex := map[string]int{}
	got := filterByGate(rows, "outside", j, ex)
	if len(got) != 2 || got[0].ID != "out" || got[1].ID != "both" {
		t.Fatalf("got %+v", got)
	}
	for k, v := range map[string]int{"other_gate": 1, "gate_unknown": 2, "gate_not_checked": 1, "gate_conflict": 1} {
		if ex[k] != v {
			t.Errorf("excluded[%s] = %d, want %d (all: %v)", k, ex[k], v, ex)
		}
	}
}

func TestLiveJoinCoverageCappedByRadius(t *testing.T) {
	j := &liveJoin{lat: 35.68, lon: 139.76, radiusM: 500, needM: 15, coveredM: math.Inf(1)}
	near := locker.Locker{Lat: fp(35.681), Lon: fp(139.76)} // about 111 m
	far := locker.Locker{Lat: fp(35.69), Lon: fp(139.76)}   // about 1.1 km, beyond the query radius
	if !j.covers(near) || j.covers(far) || j.covers(locker.Locker{}) {
		t.Fatal("coverage must stop at the query radius even after the last page")
	}
}

func TestLiveJoinDoneSkipsTextConflicts(t *testing.T) {
	outside := "outside"
	bank := []locker.EkicubeLocation{{ID: 1, Lat: 35.68, Lon: 139.76, Gate: &outside}}
	kept := []locker.Locker{{ID: "a", Name: "改札内ロッカー", Lat: fp(35.68001), Lon: fp(139.76)}}
	j := &liveJoin{lat: 35.68, lon: 139.76, radiusM: 1000, needM: 15}
	if j.done(kept, "outside", 1)(bank, math.Inf(1)) {
		t.Fatal("a record whose text conflicts must not count as a confirmed match")
	}
	kept[0].Name = "東口ロッカー"
	if !j.done(kept, "outside", 1)(bank, math.Inf(1)) {
		t.Fatal("a plain record with a single outside bank should count")
	}
}

func TestGateMatches(t *testing.T) {
	if gateMatches(nil, "inside") || !gateMatches(sp("both"), "inside") || gateMatches(sp("outside"), "inside") || !gateMatches(sp("inside"), "inside") {
		t.Fatal("gateMatches rules wrong")
	}
}

func TestBankSizesKnown(t *testing.T) {
	if bankSizesKnown(locker.EkicubeLocation{}) {
		t.Fatal("no boxes means sizes unknown")
	}
	if !bankSizesKnown(locker.EkicubeLocation{Boxes: []locker.EkicubeBox{{SizeClass: sp("L")}}}) {
		t.Fatal("a box with a size class means sizes known")
	}
}

// TestRadiusTextMatchesConstants guards hand-written help text and
// generated docs (README, SKILL, which index) that name the radii.
func TestRadiusTextMatchesConstants(t *testing.T) {
	if locker.GateRadiusM != 15 || locker.MatchRadiusM != 30 {
		t.Fatal("radius changed: update near Long help, README Data Honesty, SKILL and research.json descriptions that say 15 m / 30 m")
	}
	cmd := newNovelNearCmd(&rootFlags{})
	if !strings.Contains(cmd.Long, fmt.Sprintf("within %d m", locker.GateRadiusM)) {
		t.Fatal("near Long help must state the gate radius")
	}
	if !strings.Contains(cmd.Flags().Lookup("live").Usage, fmt.Sprintf("%d m", locker.MatchRadiusM)) {
		t.Fatal("--live help must state the match radius")
	}
}

func TestValidateLimitPages(t *testing.T) {
	cases := []struct {
		limit, pages int
		ok           bool
	}{{20, 3, true}, {0, 3, false}, {101, 3, false}, {5, 0, false}, {5, 11, false}, {100, 10, true}}
	for _, c := range cases {
		if err := validateLimitPages(c.limit, c.pages); (err == nil) != c.ok {
			t.Errorf("validateLimitPages(%d, %d) = %v", c.limit, c.pages, err)
		}
	}
}

func TestLockerIDArg(t *testing.T) {
	cases := []struct {
		args []string
		flag string
		want string
		ok   bool
	}{
		{[]string{"2685"}, "", "2685", true},
		{nil, "2685", "2685", true},
		{[]string{"2685"}, "2685", "2685", true},
		{[]string{"2685"}, "3366", "", false},
		{[]string{"1", "2"}, "", "", false},
		{nil, "", "", false},
	}
	for _, c := range cases {
		got, err := lockerIDArg(c.args, c.flag)
		if (err == nil) != c.ok || got != c.want {
			t.Errorf("lockerIDArg(%v, %q) = %q, %v", c.args, c.flag, got, err)
		}
	}
}
