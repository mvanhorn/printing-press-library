// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
)

func TestPlanPrice(t *testing.T) {
	tests := []struct {
		plan string
		want float64
	}{
		{"starter", 7.00},
		{"standard", 25.00},
		{"pro", 85.00},
		{"basic-1gb", 19.00},
		{"unknown-plan", 0},
		{"", 0},
		{"  STARTER  ", 7.00}, // whitespace + casing tolerated
	}
	for _, tc := range tests {
		t.Run(tc.plan, func(t *testing.T) {
			got := planPrice(tc.plan)
			if got != tc.want {
				t.Errorf("planPrice(%q) = %v, want %v", tc.plan, got, tc.want)
			}
		})
	}
}

func TestGroupCostRows_None(t *testing.T) {
	rows := []costRow{
		{ID: "srv-1", Kind: "service", Plan: "starter", Project: "p1", Monthly: 7},
		{ID: "srv-2", Kind: "service", Plan: "standard", Project: "p1", Monthly: 25},
		{ID: "pg-1", Kind: "postgres", Plan: "basic-1gb", Project: "p2", Monthly: 19},
		{ID: "kv-1", Kind: "key-value", Plan: "starter", Project: "p2", Monthly: 7},
		{ID: "disk-1", Kind: "disk", Project: "p1", Monthly: 0},
	}
	rep := groupCostRows(rows, "none")
	if len(rep.Groups) != 1 {
		t.Fatalf("expected 1 group for 'none', got %d", len(rep.Groups))
	}
	if rep.Total.Services != 2 {
		t.Errorf("total services: got %d, want 2", rep.Total.Services)
	}
	if rep.Total.Postgres != 1 {
		t.Errorf("total postgres: got %d, want 1", rep.Total.Postgres)
	}
	if rep.Total.KeyValue != 1 {
		t.Errorf("total key-value: got %d, want 1", rep.Total.KeyValue)
	}
	if rep.Total.Disks != 1 {
		t.Errorf("total disks: got %d, want 1", rep.Total.Disks)
	}
	wantTotal := 7 + 25 + 19 + 7.0
	if rep.Total.MonthlyUSD != wantTotal {
		t.Errorf("total monthly: got %v, want %v", rep.Total.MonthlyUSD, wantTotal)
	}
}

func TestGroupCostRows_ByProject(t *testing.T) {
	rows := []costRow{
		{ID: "srv-1", Kind: "service", Project: "p1", Monthly: 7},
		{ID: "srv-2", Kind: "service", Project: "p2", Monthly: 25},
		{ID: "srv-3", Kind: "service", Project: "p1", Monthly: 25},
	}
	rep := groupCostRows(rows, "project")
	if len(rep.Groups) != 2 {
		t.Fatalf("expected 2 project groups, got %d", len(rep.Groups))
	}
	for _, g := range rep.Groups {
		if g.Group == "p1" && g.MonthlyUSD != 32 {
			t.Errorf("p1 monthly: got %v, want 32", g.MonthlyUSD)
		}
		if g.Group == "p2" && g.MonthlyUSD != 25 {
			t.Errorf("p2 monthly: got %v, want 25", g.MonthlyUSD)
		}
	}
}

func TestGroupCostRows_UnassignedFallback(t *testing.T) {
	rows := []costRow{
		{ID: "srv-1", Kind: "service", Monthly: 7}, // no project
	}
	rep := groupCostRows(rows, "project")
	if len(rep.Groups) != 1 || rep.Groups[0].Group != "(unassigned)" {
		t.Errorf("expected '(unassigned)' bucket, got %+v", rep.Groups)
	}
}
