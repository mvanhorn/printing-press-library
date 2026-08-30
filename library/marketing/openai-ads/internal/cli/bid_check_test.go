// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestNovelBidCheckHelpWires smoke-tests that the bid-check command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelBidCheckHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"bid-check", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("bid-check --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "bid-check"} {
		if !strings.Contains(help, want) {
			t.Fatalf("bid-check --help missing %q in output:\n%s", want, help)
		}
	}
}

// --- Phase 3 behavior tests ---

func TestBidImpliedClicksPerDay(t *testing.T) {
	cases := []struct {
		name   string
		budget int64
		maxBid int64
		want   float64
	}{
		{"normal", 100_000_000, 10_000_000, 10},
		{"zero bid yields zero", 100_000_000, 0, 0},
		{"negative bid zero", 50_000_000, -5, 0},
		{"budget smaller than bid", 5_000_000, 10_000_000, 0.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := bidImpliedClicksPerDay(tc.budget, tc.maxBid)
			if got != tc.want {
				t.Fatalf("bidImpliedClicksPerDay(%d,%d)=%v want %v", tc.budget, tc.maxBid, got, tc.want)
			}
		})
	}
}

func TestBidIsFlagged(t *testing.T) {
	cases := []struct {
		implied float64
		min     int
		want    bool
	}{
		{5, 10, true},
		{10, 10, false},
		{9.99, 10, true},
		{30, 20, false},
	}
	for _, tc := range cases {
		if got := bidIsFlagged(tc.implied, tc.min); got != tc.want {
			t.Fatalf("bidIsFlagged(%v,%d)=%v want %v", tc.implied, tc.min, got, tc.want)
		}
	}
}

func TestNovelBidCheck_EmptyStoreJSON(t *testing.T) {
	novelEmptyStore(t)
	out, err := runNovelCmd(t, "bid-check", "--json")
	if err != nil {
		t.Fatalf("bid-check empty: %v", err)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("bid-check empty expected [], got %q", out)
	}
}

func TestNovelBidCheck_HappyPath(t *testing.T) {
	novelSeedStore(t)
	out, err := runNovelCmd(t, "bid-check", "--json")
	if err != nil {
		t.Fatalf("bid-check: %v", err)
	}
	var rows []bidCheckRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("bid-check json: %v\n%s", err, out)
	}
	if len(rows) != 1 {
		t.Fatalf("expected only adgrp_1 flagged (implied 5 < 10), got %d: %s", len(rows), out)
	}
	if rows[0].AdGroupID != "adgrp_1" {
		t.Fatalf("unexpected flagged row: %+v", rows[0])
	}
	if rows[0].ImpliedClicksPerDay != 5 {
		t.Fatalf("expected implied 5, got %v", rows[0].ImpliedClicksPerDay)
	}
}
