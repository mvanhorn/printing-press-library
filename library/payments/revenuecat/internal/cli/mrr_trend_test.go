// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

func mkChart(rows ...string) chartData {
	cd := chartData{Resolution: "week", YAxisCurrency: "USD"}
	for _, r := range rows {
		cd.Values = append(cd.Values, json.RawMessage(r))
	}
	return cd
}

func TestJoinMrrSeries(t *testing.T) {
	// Two periods (cohort = Unix seconds); movement keyed by the same cohorts.
	mrr := mkChart(
		`{"cohort":1640995200,"measure":0,"value":100}`, // 2022-01-01
		`{"cohort":1641600000,"measure":0,"value":130}`, // 2022-01-08
	)
	move := mkChart(
		`{"cohort":1640995200,"measure":0,"value":100}`,
		`{"cohort":1641600000,"measure":0,"value":30}`,
	)
	pts := joinMrrSeries(mrr, move)
	if len(pts) != 2 {
		t.Fatalf("points = %d, want 2", len(pts))
	}
	// First point: no prior, delta 0, movement 100.
	if pts[0].MRR != 100 || pts[0].Delta != 0 || pts[0].Movement != 100 {
		t.Fatalf("point0 = %+v", pts[0])
	}
	// Second point: delta = 130-100 = 30, movement 30.
	if pts[1].MRR != 130 || pts[1].Delta != 30 || pts[1].Movement != 30 {
		t.Fatalf("point1 = %+v", pts[1])
	}
	// Chronological order preserved.
	if pts[0].PeriodStart >= pts[1].PeriodStart {
		t.Fatalf("expected chronological order, got %q then %q", pts[0].PeriodStart, pts[1].PeriodStart)
	}
}

func TestJoinMrrSeriesEmpty(t *testing.T) {
	if got := joinMrrSeries(chartData{}, chartData{}); len(got) != 0 {
		t.Fatalf("expected empty join, got %d points", len(got))
	}
}
