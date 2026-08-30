// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

// TestNovelFatigueHelpWires smoke-tests that the fatigue command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelFatigueHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"fatigue", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("fatigue --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "fatigue"} {
		if !strings.Contains(help, want) {
			t.Fatalf("fatigue --help missing %q in output:\n%s", want, help)
		}
	}
}

// --- Phase 3 behavior tests ---

func TestFatigueDecay(t *testing.T) {
	cases := []struct {
		name string
		ctrs []float64
		want float64
	}{
		{"declining", []float64{0.05, 0.02}, 0.03},
		{"climbing is negative", []float64{0.01, 0.04}, -0.03},
		{"flat", []float64{0.03, 0.03}, 0},
		{"single returns zero", []float64{0.02}, 0},
		{"empty returns zero", []float64{}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := fatigueDecay(tc.ctrs)
			if math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("fatigueDecay(%v)=%v want %v", tc.ctrs, got, tc.want)
			}
		})
	}
}

func TestNovelFatigue_NoInsightsNote(t *testing.T) {
	novelSeedStore(t)
	out, errOut, err := runNovelCmdOutErr(t, "fatigue", "--json")
	if err != nil {
		t.Fatalf("fatigue: %v", err)
	}
	// stdout stays a bare array (uniform across every novel command); the
	// empty-state explanation belongs on stderr.
	var rows []fatigueRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("fatigue json: %v\n%s", err, out)
	}
	if len(rows) != 0 {
		t.Fatalf("expected empty results, got %+v", rows)
	}
	if !strings.Contains(errOut, "sync") {
		t.Fatalf("stderr note should name sync, got %q", errOut)
	}
}

func TestNovelFatigue_EmptyStore(t *testing.T) {
	novelEmptyStore(t)
	out, err := runNovelCmd(t, "fatigue", "--json")
	if err != nil {
		t.Fatalf("fatigue: %v", err)
	}
	var rows []fatigueRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("fatigue json: %v\n%s", err, out)
	}
	if len(rows) != 0 {
		t.Fatalf("expected empty results, got %+v", rows)
	}
}

func TestNovelFatigue_HappyPath(t *testing.T) {
	s := novelSeedStore(t)
	now := time.Now().UTC().Format(time.RFC3339)
	older := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	if _, err := s.DB().Exec(`INSERT INTO ads_insights (id, ads_id, data, synced_at, parent_id) VALUES (?,?,?,?,?)`,
		"ins_a1_old", "ad_1", `{"impressions":10000,"clicks":500,"ctr":0.05}`, older, "ad_1"); err != nil {
		t.Fatalf("insert insight old: %v", err)
	}
	if _, err := s.DB().Exec(`INSERT INTO ads_insights (id, ads_id, data, synced_at, parent_id) VALUES (?,?,?,?,?)`,
		"ins_a1_new", "ad_1", `{"impressions":10000,"clicks":200,"ctr":0.02}`, now, "ad_1"); err != nil {
		t.Fatalf("insert insight new: %v", err)
	}
	out, err := runNovelCmd(t, "fatigue", "--json", "--days", "7")
	if err != nil {
		t.Fatalf("fatigue: %v", err)
	}
	var rows []fatigueRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("fatigue json: %v\n%s", err, out)
	}
	if len(rows) != 1 || rows[0].AdID != "ad_1" {
		t.Fatalf("expected ad_1 ranked, got %+v", rows)
	}
	if rows[0].Decay < 2.9 || rows[0].Decay > 3.1 {
		t.Fatalf("expected decay ~3.0, got %v", rows[0].Decay)
	}
}
