// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for the ftp-history feature.

package cli

import (
	"testing"
	"time"
)

func TestComputeFTPHistory(t *testing.T) {
	mk := func(ftp, cp float64, date string) powerZoneRecord {
		tt, _ := time.Parse("2006-01-02", date)
		return powerZoneRecord{FTP: ftp, CriticalPower: cp, Updated: tt}
	}
	recs := []powerZoneRecord{
		mk(250, 280, "2026-03-01"),
		mk(240, 270, "2026-01-01"),
		mk(0, 0, "2026-02-01"), // no FTP -> excluded
		mk(260, 290, "2026-05-01"),
	}
	pts := computeFTPHistory(recs, 70) // 70 kg
	if len(pts) != 3 {
		t.Fatalf("points=%d want 3 (zero-FTP record excluded)", len(pts))
	}
	if pts[0].FTP != 240 || pts[1].FTP != 250 || pts[2].FTP != 260 {
		t.Errorf("not sorted ascending by date: %v %v %v", pts[0].FTP, pts[1].FTP, pts[2].FTP)
	}
	if pts[0].ChangeW != 0 {
		t.Errorf("first change=%v want 0", pts[0].ChangeW)
	}
	if pts[1].ChangeW != 10 {
		t.Errorf("second change=%v want 10", pts[1].ChangeW)
	}
	if pts[0].WattsPerKg < 3.42 || pts[0].WattsPerKg > 3.44 { // 240/70 = 3.43
		t.Errorf("watts/kg=%v want ~3.43", pts[0].WattsPerKg)
	}
	// No weight -> no watts/kg.
	if noW := computeFTPHistory(recs, 0); noW[0].WattsPerKg != 0 {
		t.Errorf("expected no watts/kg without weight, got %v", noW[0].WattsPerKg)
	}
	// No records with FTP -> empty.
	if empty := computeFTPHistory([]powerZoneRecord{mk(0, 0, "2026-01-01")}, 70); len(empty) != 0 {
		t.Errorf("expected empty, got %d", len(empty))
	}
}
