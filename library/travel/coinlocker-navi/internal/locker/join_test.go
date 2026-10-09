// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package locker

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestDistanceM(t *testing.T) {
	// Tokyo Station to Shinjuku Station is about 6.1 km.
	d := DistanceM(35.6812, 139.7671, 35.6896, 139.7006)
	if math.Abs(d-6100) > 200 {
		t.Fatalf("distance = %.0f", d)
	}
}

func f(v float64) *float64 { return &v }

func TestAttachEkicubeGateRules(t *testing.T) {
	inside, outside := "inside", "outside"
	banks := []EkicubeLocation{
		{ID: 1, Lat: 35.68000, Lon: 139.76000, Gate: &inside},
		{ID: 2, Lat: 35.68020, Lon: 139.76000, Gate: &outside}, // ~22 m north of bank 1
	}
	rows := []Locker{
		{ID: "a", Lat: f(35.68001), Lon: f(139.76000)}, // 1 m from bank 1, 21 m from bank 2
		{ID: "b", Lat: f(35.68010), Lon: f(139.76000)}, // 11 m from both banks
		{ID: "c", Lat: f(35.69000), Lon: f(139.76000)}, // far away
		{ID: "d"}, // no coordinates
	}
	AttachEkicube(rows, banks, "2026-10-09T11:00:00+09:00", true)
	if rows[0].Gate == nil || *rows[0].Gate != "inside" || *rows[0].GateSource != GateSourceBank {
		t.Fatalf("a: gate = %v", rows[0].Gate)
	}
	if rows[0].Live == nil || rows[0].Live.Ekicube.ID != 1 || rows[0].Live.CandidatesInRad != 2 || rows[0].Live.Ekicube.Caveat != "" {
		t.Fatalf("a: live = %+v", rows[0].Live)
	}
	if rows[1].Gate != nil {
		t.Fatalf("b: two banks within 15 m must leave gate unknown, got %v", *rows[1].Gate)
	}
	if rows[1].Live == nil {
		t.Fatal("b: live data should still attach within 30 m")
	}
	if rows[2].Gate != nil || rows[2].Live != nil || rows[3].Live != nil {
		t.Fatal("c/d: no match expected")
	}
}

func TestAttachMaihamaOverridesProximity(t *testing.T) {
	id := "2669"
	board := MaihamaBoard{Blocks: []MaihamaBlock{{GID: "0001", LockerID: &id}}}
	rows := []Locker{{ID: "2669", Live: &LiveMatch{Source: "multiecube"}}, {ID: "1"}}
	AttachMaihama(rows, board, "now")
	if rows[0].Live.Source != "coinlocker-navi-maihama" || rows[0].Live.MatchBasis != "locker_id" {
		t.Fatalf("exact id must win: %+v", rows[0].Live)
	}
	if rows[1].Live != nil {
		t.Fatal("unmatched row must stay without live data")
	}
}

func TestGateFor(t *testing.T) {
	inside, outside := "inside", "outside"
	one := []EkicubeLocation{{ID: 1, Lat: 35.68, Lon: 139.76, Gate: &inside}}
	two := append(one, EkicubeLocation{ID: 2, Lat: 35.68005, Lon: 139.76, Gate: &outside})
	noGate := []EkicubeLocation{{ID: 3, Lat: 35.68, Lon: 139.76}}
	cases := []struct {
		name  string
		banks []EkicubeLocation
		want  string
	}{
		{"single bank", one, "inside"},
		{"two banks within 15 m", two, ""},
		{"bank without gate field", noGate, ""},
		{"no banks", nil, ""},
	}
	for _, c := range cases {
		got := GateFor(35.68001, 139.76, c.banks)
		if (got == nil) != (c.want == "") || (got != nil && *got != c.want) {
			t.Errorf("%s: got %v, want %q", c.name, got, c.want)
		}
	}
}

func TestCoveredM(t *testing.T) {
	read := []EkicubeLocation{{Lat: 35.68, Lon: 139.76}, {Lat: 35.681, Lon: 139.76}}
	if c := CoveredM(35.68, 139.76, read, false); c < 100 || c > 120 {
		t.Fatalf("covered = %.0f, want about 111", c)
	}
	if c := CoveredM(35.68, 139.76, read, true); !math.IsInf(c, 1) {
		t.Fatalf("last page must cover everything, got %.0f", c)
	}
	if c := CoveredM(35.68, 139.76, nil, false); c != 0 {
		t.Fatalf("no banks: %.0f", c)
	}
}

func TestGateConflictBlocksBankValue(t *testing.T) {
	outside := "outside"
	banks := []EkicubeLocation{{ID: 1, Lat: 35.68, Lon: 139.76, Gate: &outside}}
	rows := []Locker{
		{ID: "a", Name: "JR新宿駅東口改札内改札手前付近", Lat: f(35.68001), Lon: f(139.76)},
		{ID: "b", Name: "JR新宿駅東口", Lat: f(35.68001), Lon: f(139.76)},
	}
	AttachEkicube(rows, banks, "now", false)
	if rows[0].Gate != nil || rows[0].GateConflict == nil {
		t.Fatalf("a: conflict must leave gate null, got gate=%v conflict=%v", rows[0].Gate, rows[0].GateConflict)
	}
	if rows[1].Gate == nil || *rows[1].Gate != "outside" || rows[1].GateConflict != nil {
		t.Fatalf("b: plain name keeps bank gate, got %v", rows[1].Gate)
	}
}

func TestRadiusInSourceLabels(t *testing.T) {
	if !strings.Contains(GateSourceBank, fmt.Sprintf("within_%dm", GateRadiusM)) {
		t.Fatalf("GateSourceBank %q must name the %d m radius", GateSourceBank, GateRadiusM)
	}
	inside := "inside"
	rows := []Locker{{ID: "a", Lat: f(35.68), Lon: f(139.76)}}
	AttachEkicube(rows, []EkicubeLocation{{ID: 1, Lat: 35.68, Lon: 139.76, Gate: &inside}}, "now", true)
	if rows[0].Live == nil || !strings.Contains(rows[0].Live.MatchBasis, fmt.Sprintf("within_%dm", MatchRadiusM)) {
		t.Fatalf("match basis must name the %d m radius: %+v", MatchRadiusM, rows[0].Live)
	}
}

func TestTextGateConflictBoth(t *testing.T) {
	r := Locker{Name: "改札内ロッカー"}
	if TextGateConflict(r, "both") == "" || TextGateConflict(r, "inside") != "" || TextGateConflict(r, "outside") == "" {
		t.Fatal("conflict rules wrong")
	}
	if TextGateConflict(Locker{Name: "東口"}, "both") != "" {
		t.Fatal("no side named: no conflict")
	}
}

func TestLiveSkipsBankOnOtherGateSide(t *testing.T) {
	inside, outside := "inside", "outside"
	banks := []EkicubeLocation{
		{ID: 1, Lat: 35.68000, Lon: 139.76000, Gate: &inside},  // nearest
		{ID: 2, Lat: 35.68015, Lon: 139.76000, Gate: &outside}, // ~17 m away
	}
	rows := []Locker{
		{ID: "out", Name: "丸の内地下南口改札外ATM付近", Lat: f(35.68001), Lon: f(139.76)},
		{ID: "plain", Name: "丸の内地下南口", Lat: f(35.68001), Lon: f(139.76)},
		{ID: "only-in", Name: "改札外ロッカー", Lat: f(35.69001), Lon: f(139.76)},
	}
	only := []EkicubeLocation{{ID: 3, Lat: 35.69, Lon: 139.76, Gate: &inside}}
	AttachEkicube(rows[:2], banks, "now", true)
	AttachEkicube(rows[2:], only, "now", true)
	if rows[0].Live == nil || rows[0].Live.Ekicube.ID != 2 || rows[0].Live.CandidatesInRad != 2 {
		t.Fatalf("out: want the outside bank 2, got %+v", rows[0].Live)
	}
	if rows[1].Live == nil || rows[1].Live.Ekicube.ID != 1 {
		t.Fatalf("plain: want the nearest bank 1, got %+v", rows[1].Live)
	}
	if rows[2].Live != nil {
		t.Fatalf("only-in: a bank on the other gate side must not attach, got %+v", rows[2].Live)
	}
}
