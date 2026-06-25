// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"
)

func TestSchedulePaceFraction(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC) // 10-day flight
	tests := []struct {
		name string
		now  time.Time
		want float64
	}{
		{"before start clamps to 0", start.Add(-24 * time.Hour), 0},
		{"exactly at start", start, 0},
		{"halfway", time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC), 0.5},
		{"after end clamps to 1", end.Add(24 * time.Hour), 1},
		{"exactly at end clamps to 1", end, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := schedulePaceFraction(start, end, tt.now); got != tt.want {
				t.Fatalf("schedulePaceFraction(now=%v) = %v, want %v", tt.now, got, tt.want)
			}
		})
	}
}

func TestSchedulePaceFractionDegenerateFlight(t *testing.T) {
	// end <= start must not divide by zero; returns 0.
	d := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if got := schedulePaceFraction(d, d, d); got != 0 {
		t.Fatalf("zero-duration flight = %v, want 0", got)
	}
	if got := schedulePaceFraction(d.Add(time.Hour), d, d); got != 0 {
		t.Fatalf("inverted flight = %v, want 0", got)
	}
}

func TestClassifyLineItems(t *testing.T) {
	now := time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC)
	items := []rawLineItem{
		{
			Name:         "networks/123/lineItems/111",
			DisplayName:  "Half done",
			StartTime:    "2026-06-01T00:00:00Z",
			EndTime:      "2026-06-11T00:00:00Z",
			LineItemType: "STANDARD",
			Goal:         lineItemGoal{Units: "1000", GoalType: "LIFETIME", UnitType: "IMPRESSIONS"},
		},
		{
			Name:        "networks/123/lineItems/222",
			DisplayName: "Almost done",
			StartTime:   "2026-06-01T00:00:00Z",
			EndTime:     "2026-06-07T00:00:00Z",
			Goal:        lineItemGoal{Units: "500"},
		},
		{
			Name:        "networks/123/lineItems/333",
			DisplayName: "Broken dates",
			StartTime:   "not-a-time",
			EndTime:     "2026-06-07T00:00:00Z",
		},
		{
			Name:        "networks/123/lineItems/444",
			DisplayName: "Missing end",
			StartTime:   "2026-06-01T00:00:00Z",
		},
	}

	paces, failures := classifyLineItems(items, now)

	if len(paces) != 2 {
		t.Fatalf("len(paces) = %d, want 2 (%#v)", len(paces), paces)
	}
	if len(failures) != 2 {
		t.Fatalf("len(failures) = %d, want 2 (%#v)", len(failures), failures)
	}

	// Sorted by schedule fraction descending: 222 (~0.833) before 111 (0.5).
	if paces[0].ID != "222" || paces[1].ID != "111" {
		t.Fatalf("ranking = [%s,%s], want [222,111]", paces[0].ID, paces[1].ID)
	}
	if paces[1].ScheduleFraction != 0.5 {
		t.Fatalf("111 schedule_fraction = %v, want 0.5", paces[1].ScheduleFraction)
	}
	if paces[0].GoalUnits != 500 {
		t.Fatalf("222 goal_units = %d, want 500", paces[0].GoalUnits)
	}
	if paces[1].GoalUnits != 1000 || paces[1].LineItemType != "STANDARD" {
		t.Fatalf("111 goal_units/type = %d/%q, want 1000/STANDARD", paces[1].GoalUnits, paces[1].LineItemType)
	}

	// Failures carry the line-item id and a reason, never a phantom zero row.
	failByID := map[string]string{}
	for _, f := range failures {
		failByID[f.ID] = f.Reason
	}
	if _, ok := failByID["333"]; !ok {
		t.Fatalf("expected 333 in failures, got %#v", failures)
	}
	if _, ok := failByID["444"]; !ok {
		t.Fatalf("expected 444 in failures, got %#v", failures)
	}
}

func TestLastPathSegment(t *testing.T) {
	tests := []struct{ in, want string }{
		{"networks/123/lineItems/456", "456"},
		{"456", "456"},
		{"networks/123/lineItems/", "networks/123/lineItems/"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := lastPathSegment(tt.in); got != tt.want {
			t.Fatalf("lastPathSegment(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
