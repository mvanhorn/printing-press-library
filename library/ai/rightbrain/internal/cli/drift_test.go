// Copyright 2026 Farouk Umar and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/rightbrain/internal/store"
)

// TestNovelDriftHelpWires smoke-tests that the drift command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelDriftHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"drift", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("drift --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "drift"} {
		if !strings.Contains(help, want) {
			t.Fatalf("drift --help missing %q in output:\n%s", want, help)
		}
	}
}

// driftTestNow is a fixed clock so window boundaries are exact in tests.
var driftTestNow = time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)

// driftRunAt builds a run positioned `ago` before the fixed test clock.
func driftRunAt(taskID string, ago time.Duration, credits float64, hasCredits bool, latency float64, isErr bool) driftRun {
	return driftRun{
		TaskID:      taskID,
		TaskName:    taskID + " name",
		Created:     driftTestNow.Add(-ago),
		IsError:     isErr,
		TotalTokens: 100,
		Credits:     credits,
		HasCredits:  hasCredits,
		LatencySecs: latency,
		HasLatency:  true,
	}
}

// driftRunsAt returns n identical runs placed in the same window.
func driftRunsAt(taskID string, ago time.Duration, n int, credits, latency float64) []driftRun {
	out := make([]driftRun, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, driftRunAt(taskID, ago, credits, true, latency, false))
	}
	return out
}

// findDriftMover locates a mover by key so assertions do not depend on the
// sort order that is under test elsewhere.
func findDriftMover(t *testing.T, report driftReport, key string) driftMover {
	t.Helper()
	for _, m := range report.Movers {
		if m.Key == key {
			return m
		}
	}
	t.Fatalf("mover %q not found in report; movers = %+v", key, report.Movers)
	return driftMover{}
}

// TestDriftCreditsDeltaPct checks that a current window costing more than the
// previous one produces a positive credits_delta_pct of the right size, and
// that a cheaper window produces a negative one.
func TestDriftCreditsDeltaPct(t *testing.T) {
	window := 7 * 24 * time.Hour
	prevAgo := 10 * 24 * time.Hour // inside [now-14d, now-7d)
	curAgo := 2 * 24 * time.Hour   // inside [now-7d, now]

	cases := []struct {
		name        string
		prevCredits float64
		curCredits  float64
		wantPct     float64
	}{
		{"got more expensive", 10, 15, 50},
		{"doubled", 4, 8, 100},
		{"got cheaper", 10, 7.5, -25},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := append(
				driftRunsAt("task-a", prevAgo, 25, tc.prevCredits, 1),
				driftRunsAt("task-a", curAgo, 25, tc.curCredits, 1)...,
			)
			report := buildDriftReport(rows, driftTestNow, window, "task", 20)
			if len(report.Movers) != 1 {
				t.Fatalf("movers = %d, want 1: %+v", len(report.Movers), report.Movers)
			}
			m := report.Movers[0]
			if m.CreditsDeltaPct == nil {
				t.Fatalf("credits_delta_pct = nil, want %v", tc.wantPct)
			}
			if *m.CreditsDeltaPct != tc.wantPct {
				t.Fatalf("credits_delta_pct = %v, want %v (mean %v -> %v)",
					*m.CreditsDeltaPct, tc.wantPct, m.MeanCreditsPrevious, m.MeanCreditsCurrent)
			}
			if m.MeanCreditsCurrent != tc.curCredits || m.MeanCreditsPrevious != tc.prevCredits {
				t.Fatalf("mean credits = %v/%v, want %v/%v",
					m.MeanCreditsCurrent, m.MeanCreditsPrevious, tc.curCredits, tc.prevCredits)
			}
			if m.RunsCurrent != 25 || m.RunsPrevious != 25 {
				t.Fatalf("runs = %d/%d, want 25/25", m.RunsCurrent, m.RunsPrevious)
			}
			if m.New {
				t.Fatal("mover flagged new despite a populated previous window")
			}
		})
	}
}

// TestDriftCreditsMeanExcludesUnparseableValues pins the rule that a run whose
// charged_credits is missing or unparseable is dropped from the mean rather
// than folded in as 0.00, which would fake a cost improvement.
func TestDriftCreditsMeanExcludesUnparseableValues(t *testing.T) {
	window := 7 * 24 * time.Hour
	rows := []driftRun{
		driftRunAt("task-a", 10*24*time.Hour, 10, true, 1, false),
		driftRunAt("task-a", 10*24*time.Hour, 10, true, 1, false),
		driftRunAt("task-a", 2*24*time.Hour, 10, true, 1, false),
		driftRunAt("task-a", 2*24*time.Hour, 0, false, 1, false), // unparseable credits
	}
	report := buildDriftReport(rows, driftTestNow, window, "task", 1)
	m := findDriftMover(t, report, "task-a")
	if m.MeanCreditsCurrent != 10 {
		t.Fatalf("mean_credits_current = %v, want 10 (the credit-less run must be excluded, not counted as 0)",
			m.MeanCreditsCurrent)
	}
	if m.RunsCurrent != 2 {
		t.Fatalf("runs_current = %d, want 2 (the credit-less run still counts as a run)", m.RunsCurrent)
	}
	if m.CreditsDeltaPct == nil || *m.CreditsDeltaPct != 0 {
		t.Fatalf("credits_delta_pct = %v, want 0", m.CreditsDeltaPct)
	}
	if _, err := json.Marshal(report); err != nil {
		t.Fatalf("report does not marshal: %v", err)
	}
}

// TestDriftP95 asserts the exact nearest-rank p95 for known latency sets, both
// through the helper and through the assembled report.
func TestDriftP95(t *testing.T) {
	cases := []struct {
		name   string
		values []float64
		want   float64
	}{
		{"empty set is zero", nil, 0},
		{"single value", []float64{4.2}, 4.2},
		{"1..20 -> 19th value", []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}, 19},
		{"unsorted input", []float64{20, 3, 1, 19, 2}, 20},
		{"ten values", []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 50}, 50},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := driftP95(tc.values); got != tc.want {
				t.Fatalf("driftP95(%v) = %v, want %v", tc.values, got, tc.want)
			}
		})
	}

	// The same set routed through buildDriftReport: 20 current-window runs with
	// latencies 1..20 must report p95 = 19, against a previous window of 20
	// runs at a flat 10s, which is a +90% p95 drift.
	window := 7 * 24 * time.Hour
	rows := make([]driftRun, 0, 40)
	for i := 1; i <= 20; i++ {
		rows = append(rows, driftRunAt("task-a", 2*24*time.Hour, 1, true, float64(i), false))
		rows = append(rows, driftRunAt("task-a", 10*24*time.Hour, 1, true, 10, false))
	}
	report := buildDriftReport(rows, driftTestNow, window, "task", 20)
	m := findDriftMover(t, report, "task-a")
	if m.P95Current != 19 {
		t.Fatalf("p95_current = %v, want 19", m.P95Current)
	}
	if m.P95Previous != 10 {
		t.Fatalf("p95_previous = %v, want 10", m.P95Previous)
	}
	if m.P95DeltaPct == nil || *m.P95DeltaPct != 90 {
		t.Fatalf("p95_delta_pct = %v, want 90", m.P95DeltaPct)
	}
}

// TestDriftMinRunsFiltering checks that thin groups are excluded from movers
// and counted in filtered_out, so the cap is never silent.
func TestDriftMinRunsFiltering(t *testing.T) {
	window := 7 * 24 * time.Hour
	prevAgo := 10 * 24 * time.Hour
	curAgo := 2 * 24 * time.Hour

	rows := make([]driftRun, 0)
	rows = append(rows, driftRunsAt("fat", prevAgo, 30, 1, 1)...)
	rows = append(rows, driftRunsAt("fat", curAgo, 30, 2, 1)...)
	rows = append(rows, driftRunsAt("thin", prevAgo, 30, 1, 1)...)
	rows = append(rows, driftRunsAt("thin", curAgo, 5, 9, 1)...) // huge move, too few runs
	rows = append(rows, driftRunsAt("thinner", curAgo, 1, 1, 1)...)

	cases := []struct {
		name            string
		minRuns         int
		wantKeys        []string
		wantFilteredOut int
	}{
		{"default threshold drops both thin groups", 20, []string{"fat"}, 2},
		{"lower threshold admits the medium group", 5, []string{"fat", "thin"}, 1},
		{"zero threshold admits everything", 0, []string{"fat", "thin", "thinner"}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := buildDriftReport(rows, driftTestNow, window, "task", tc.minRuns)
			got := make([]string, 0, len(report.Movers))
			for _, m := range report.Movers {
				got = append(got, m.Key)
			}
			if len(got) != len(tc.wantKeys) {
				t.Fatalf("movers = %v, want %v", got, tc.wantKeys)
			}
			for _, want := range tc.wantKeys {
				found := false
				for _, k := range got {
					if k == want {
						found = true
					}
				}
				if !found {
					t.Fatalf("movers = %v, missing %q", got, want)
				}
			}
			if report.FilteredOut != tc.wantFilteredOut {
				t.Fatalf("filtered_out = %d, want %d", report.FilteredOut, tc.wantFilteredOut)
			}
			if report.MinRuns != tc.minRuns {
				t.Fatalf("min_runs = %d, want %d", report.MinRuns, tc.minRuns)
			}
		})
	}

	// When the threshold excludes everything, the report must say so rather
	// than presenting an empty movers list as "nothing moved".
	report := buildDriftReport(rows, driftTestNow, window, "task", 500)
	if len(report.Movers) != 0 {
		t.Fatalf("movers = %d, want 0", len(report.Movers))
	}
	if report.FilteredOut != 3 {
		t.Fatalf("filtered_out = %d, want 3", report.FilteredOut)
	}
	if !strings.Contains(report.Note, "--min-runs") {
		t.Fatalf("note = %q, want it to point at --min-runs", report.Note)
	}
}

// TestDriftNoChangeFabricatesNothing is the absence-of-correctness case: when
// the two windows hold identical data, every delta must be exactly 0. A report
// that invents movement here is worse than useless, because the point of the
// command is to tell an operator when to stop looking.
func TestDriftNoChangeFabricatesNothing(t *testing.T) {
	window := 7 * 24 * time.Hour
	rows := make([]driftRun, 0)
	for _, task := range []string{"task-a", "task-b"} {
		for i := 1; i <= 25; i++ {
			latency := float64(i)
			rows = append(rows, driftRunAt(task, 2*24*time.Hour, 3.5, true, latency, false))
			rows = append(rows, driftRunAt(task, 10*24*time.Hour, 3.5, true, latency, false))
		}
	}
	report := buildDriftReport(rows, driftTestNow, window, "task", 20)
	if len(report.Movers) != 2 {
		t.Fatalf("movers = %d, want 2", len(report.Movers))
	}
	for _, m := range report.Movers {
		if m.New {
			t.Fatalf("%s flagged new despite an identical previous window", m.Key)
		}
		deltas := map[string]*float64{
			"credits_delta_pct":      m.CreditsDeltaPct,
			"p95_delta_pct":          m.P95DeltaPct,
			"failure_rate_delta_pct": m.FailureRateDeltaPct,
			"runs_delta_pct":         m.RunsDeltaPct,
		}
		for name, d := range deltas {
			if d == nil {
				t.Fatalf("%s: %s = null, want an explicit 0 when nothing changed", m.Key, name)
			}
			if *d != 0 {
				t.Fatalf("%s: %s = %v, want exactly 0 (no movement was present in the data)", m.Key, name, *d)
			}
		}
		if m.MeanCreditsCurrent != m.MeanCreditsPrevious {
			t.Fatalf("%s: mean credits %v != %v", m.Key, m.MeanCreditsCurrent, m.MeanCreditsPrevious)
		}
		if m.P95Current != m.P95Previous {
			t.Fatalf("%s: p95 %v != %v", m.Key, m.P95Current, m.P95Previous)
		}
		if driftMagnitude(m) != 0 {
			t.Fatalf("%s: ranking magnitude = %v, want 0", m.Key, driftMagnitude(m))
		}
	}
}

// TestDriftNewGroupHasNoInfiniteDelta is the second absence-of-correctness
// case: a group that exists only in the current window has no baseline, so its
// deltas must be reported as unknown (null, flagged new) and never as +Inf or
// NaN — values that would sort straight to the top of the movers list and send
// an operator chasing a task that simply did not exist last week.
func TestDriftNewGroupHasNoInfiniteDelta(t *testing.T) {
	window := 7 * 24 * time.Hour
	rows := make([]driftRun, 0)
	// An established group with a real, modest regression.
	rows = append(rows, driftRunsAt("established", 10*24*time.Hour, 25, 10, 2)...)
	rows = append(rows, driftRunsAt("established", 2*24*time.Hour, 25, 11, 2)...)
	// A group that appears for the first time in the current window.
	rows = append(rows, driftRunsAt("brand-new", 2*24*time.Hour, 25, 99, 40)...)

	report := buildDriftReport(rows, driftTestNow, window, "task", 20)
	fresh := findDriftMover(t, report, "brand-new")

	if !fresh.New {
		t.Fatal("brand-new group not flagged new")
	}
	if fresh.RunsPrevious != 0 {
		t.Fatalf("runs_previous = %d, want 0", fresh.RunsPrevious)
	}
	for name, d := range map[string]*float64{
		"credits_delta_pct": fresh.CreditsDeltaPct,
		"p95_delta_pct":     fresh.P95DeltaPct,
		"runs_delta_pct":    fresh.RunsDeltaPct,
	} {
		if d == nil {
			continue
		}
		if math.IsInf(*d, 0) || math.IsNaN(*d) {
			t.Fatalf("%s = %v; a missing baseline must never produce Inf/NaN", name, *d)
		}
		t.Fatalf("%s = %v, want null for a group with no previous-window baseline", name, *d)
	}
	// Both windows saw zero failures, so that one ratio is a genuine 0.
	if fresh.FailureRateDeltaPct == nil || *fresh.FailureRateDeltaPct != 0 {
		t.Fatalf("failure_rate_delta_pct = %v, want 0", fresh.FailureRateDeltaPct)
	}

	// A null delta must marshal as JSON null, not as a number or an omitted key.
	raw, err := json.Marshal(fresh)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"credits_delta_pct":null`) {
		t.Fatalf("credits_delta_pct did not serialize as null: %s", raw)
	}
	for _, bad := range []string{"Inf", "NaN"} {
		if strings.Contains(string(raw), bad) {
			t.Fatalf("mover JSON contains %q: %s", bad, raw)
		}
	}

	// The measurable regression must outrank the unmeasurable new group.
	if report.Movers[0].Key != "established" {
		t.Fatalf("movers[0] = %q, want the group with a real measured delta ranked first",
			report.Movers[0].Key)
	}
}

// TestDriftWindowBoundaries checks that each run lands in exactly one window
// and that runs older than two windows are ignored entirely.
func TestDriftWindowBoundaries(t *testing.T) {
	window := 7 * 24 * time.Hour
	rows := []driftRun{
		driftRunAt("task-a", 1*time.Hour, 1, true, 1, false),      // current
		driftRunAt("task-a", 6*24*time.Hour, 1, true, 1, false),   // current
		driftRunAt("task-a", 8*24*time.Hour, 1, true, 1, false),   // previous
		driftRunAt("task-a", 13*24*time.Hour, 1, true, 1, false),  // previous
		driftRunAt("task-a", 30*24*time.Hour, 1, true, 1, false),  // too old
		driftRunAt("task-a", 365*24*time.Hour, 1, true, 1, false), // too old
	}
	report := buildDriftReport(rows, driftTestNow, window, "task", 1)
	if report.TotalRunsCurrent != 2 {
		t.Fatalf("total_runs_current = %d, want 2", report.TotalRunsCurrent)
	}
	if report.TotalRunsPrevious != 2 {
		t.Fatalf("total_runs_previous = %d, want 2", report.TotalRunsPrevious)
	}
	if report.Window != "7d" {
		t.Fatalf("window = %q, want %q", report.Window, "7d")
	}
	if report.CurrentWindowEnd != driftTestNow.Format(time.RFC3339) {
		t.Fatalf("current_window_end = %q", report.CurrentWindowEnd)
	}
	if report.CurrentWindowStart != report.PreviousWindowEnd {
		t.Fatalf("windows are not contiguous: %q vs %q", report.CurrentWindowStart, report.PreviousWindowEnd)
	}

	// A store with no runs at all in either window says so plainly.
	empty := buildDriftReport(nil, driftTestNow, window, "task", 20)
	if len(empty.Movers) != 0 {
		t.Fatalf("movers = %d, want 0", len(empty.Movers))
	}
	if empty.Note == "" {
		t.Fatal("empty report carries no note")
	}
	raw, err := json.Marshal(empty)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"movers":[]`) {
		t.Fatalf("empty movers marshalled as null, not []: %s", raw)
	}
}

// TestDriftGroupByDimensions checks that each --group-by dimension keys on the
// right field, resolves a display name, and computes failure rate per group.
func TestDriftGroupByDimensions(t *testing.T) {
	window := 7 * 24 * time.Hour
	base := func(ago time.Duration, isErr bool) driftRun {
		return driftRun{
			TaskID:         "t1",
			TaskName:       "Summarize",
			TaskRevisionID: "rev-9",
			ModelName:      "gpt-4o",
			TaskAgentID:    "agent-3",
			AgentName:      "Researcher",
			Created:        driftTestNow.Add(-ago),
			IsError:        isErr,
			Credits:        2,
			HasCredits:     true,
			LatencySecs:    1,
			HasLatency:     true,
		}
	}
	rows := []driftRun{
		base(2*24*time.Hour, false),
		base(2*24*time.Hour, true),
		base(10*24*time.Hour, false),
		base(10*24*time.Hour, false),
	}

	cases := []struct {
		groupBy  string
		wantKey  string
		wantName string
	}{
		{"task", "t1", "Summarize"},
		{"agent", "agent-3", "Researcher"},
		{"model", "gpt-4o", "gpt-4o"},
		{"revision", "rev-9", "Summarize"},
	}
	for _, tc := range cases {
		t.Run(tc.groupBy, func(t *testing.T) {
			report := buildDriftReport(rows, driftTestNow, window, tc.groupBy, 1)
			if report.GroupBy != tc.groupBy {
				t.Fatalf("group_by = %q, want %q", report.GroupBy, tc.groupBy)
			}
			m := findDriftMover(t, report, tc.wantKey)
			if m.Name != tc.wantName {
				t.Fatalf("name = %q, want %q", m.Name, tc.wantName)
			}
			if m.FailureRateCurrent != 0.5 {
				t.Fatalf("failure_rate_current = %v, want 0.5", m.FailureRateCurrent)
			}
			if m.FailureRatePrevious != 0 {
				t.Fatalf("failure_rate_previous = %v, want 0", m.FailureRatePrevious)
			}
			// 0 -> 0.5 has no baseline, so the ratio is undefined, not infinite.
			if m.FailureRateDeltaPct != nil {
				t.Fatalf("failure_rate_delta_pct = %v, want null (previous rate was 0)", *m.FailureRateDeltaPct)
			}
		})
	}

	// A run carrying no value for the requested dimension is attributed to no
	// group, rather than collapsed into "". It is counted under its own name
	// so the header can never claim runs no mover accounts for.
	unattributed := buildDriftReport(
		[]driftRun{{TaskID: "t1", Created: driftTestNow.Add(-time.Hour)}},
		driftTestNow, window, "agent", 1)
	if unattributed.TotalRunsCurrent != 0 {
		t.Fatalf("total_runs_current = %d, want 0: no mover can account for a run with no agent",
			unattributed.TotalRunsCurrent)
	}
	if unattributed.UnattributedRuns != 1 {
		t.Fatalf("unattributed_runs = %d, want 1", unattributed.UnattributedRuns)
	}
	if len(unattributed.Movers) != 0 {
		t.Fatalf("movers = %+v, want none", unattributed.Movers)
	}
}

// TestDriftDecodeRun covers the loose decoding of a mirrored run row:
// charged_credits arrives as a quoted string, and unreadable values must be
// excluded rather than scored as zero.
func TestDriftDecodeRun(t *testing.T) {
	cases := []struct {
		name           string
		data           string
		wantOK         bool
		wantCredits    float64
		wantHasCredits bool
		wantErr        bool
		wantTokens     int64
		wantLatency    float64
	}{
		{
			name:           "string credits",
			data:           `{"task_id":"t1","created":"2026-07-30T10:00:00Z","charged_credits":"9.00","total_tokens":1234,"llm_call_timing":2.5,"is_error":false}`,
			wantOK:         true,
			wantCredits:    9,
			wantHasCredits: true,
			wantTokens:     1234,
			wantLatency:    2.5,
		},
		{
			name:           "numeric credits",
			data:           `{"task_id":"t1","created":"2026-07-30T10:00:00Z","charged_credits":3.5,"is_error":true}`,
			wantOK:         true,
			wantCredits:    3.5,
			wantHasCredits: true,
			wantErr:        true,
		},
		{
			name:           "null credits are excluded, not zeroed",
			data:           `{"task_id":"t1","created":"2026-07-30T10:00:00Z","charged_credits":null}`,
			wantOK:         true,
			wantHasCredits: false,
		},
		{
			name:           "unparseable credits are excluded",
			data:           `{"task_id":"t1","created":"2026-07-30T10:00:00Z","charged_credits":"n/a"}`,
			wantOK:         true,
			wantHasCredits: false,
		},
		{
			name:   "missing created is dropped",
			data:   `{"task_id":"t1","charged_credits":"1.00"}`,
			wantOK: false,
		},
		{
			name:   "malformed json is dropped",
			data:   `{not json`,
			wantOK: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run, ok := driftDecodeRun(tc.data)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if run.HasCredits != tc.wantHasCredits {
				t.Fatalf("has_credits = %v, want %v", run.HasCredits, tc.wantHasCredits)
			}
			if run.Credits != tc.wantCredits {
				t.Fatalf("credits = %v, want %v", run.Credits, tc.wantCredits)
			}
			if run.IsError != tc.wantErr {
				t.Fatalf("is_error = %v, want %v", run.IsError, tc.wantErr)
			}
			if run.TotalTokens != tc.wantTokens {
				t.Fatalf("total_tokens = %d, want %d", run.TotalTokens, tc.wantTokens)
			}
			if run.LatencySecs != tc.wantLatency {
				t.Fatalf("latency = %v, want %v", run.LatencySecs, tc.wantLatency)
			}
		})
	}
}

// TestDriftCommandGuards exercises the command surface: --data-source live is
// refused outright, bad flag values are usage errors, and a missing mirror
// produces the sync instruction plus an empty JSON body instead of a crash.
func TestDriftCommandGuards(t *testing.T) {
	missingDB := filepath.Join(t.TempDir(), "absent.db")

	cases := []struct {
		name        string
		args        []string
		wantErr     string
		wantStdout  string
		wantStderr  string
		wantSuccess bool
	}{
		{
			name:    "live data source is refused",
			args:    []string{"drift", "--data-source", "live", "--no-learn"},
			wantErr: "no live equivalent",
		},
		{
			name:    "unknown group-by is a usage error",
			args:    []string{"drift", "--group-by", "tasks", "--db", missingDB, "--no-learn"},
			wantErr: "invalid --group-by",
		},
		{
			name:    "unparseable since is a usage error",
			args:    []string{"drift", "--since", "banana", "--db", missingDB, "--no-learn"},
			wantErr: "invalid --since",
		},
		{
			name:        "dry run does no work",
			args:        []string{"drift", "--dry-run", "--no-learn"},
			wantStdout:  "would compare the current window",
			wantSuccess: true,
		},
		{
			name:        "missing mirror explains how to sync and emits an empty body",
			args:        []string{"drift", "--db", missingDB, "--json", "--no-learn"},
			wantStdout:  "[]",
			wantStderr:  "no local mirror at",
			wantSuccess: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := RootCmd()
			cmd.SetArgs(tc.args)
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			err := cmd.Execute()

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil (stdout=%q)", tc.wantErr, stdout.String())
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %q, want it to contain %q", err.Error(), tc.wantErr)
				}
				if ExitCode(err) != 2 {
					t.Fatalf("exit code = %d, want 2 for a usage error", ExitCode(err))
				}
				return
			}
			if tc.wantSuccess && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantStdout != "" && !strings.Contains(stdout.String(), tc.wantStdout) {
				t.Fatalf("stdout = %q, want it to contain %q", stdout.String(), tc.wantStdout)
			}
			if tc.wantStderr != "" && !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr.String(), tc.wantStderr)
			}
		})
	}
}

// TestDriftEndToEndAgainstLocalMirror seeds a real SQLite mirror and drives the
// command through RootCmd, so the SQL read, the JSON decode of the mirrored
// `data` column, and the report assembly are exercised together. Rows with
// missing fields are included deliberately: the loop must skip them, not
// abandon the query and report "no data".
func TestDriftEndToEndAgainstLocalMirror(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	insert := func(id, resourceType, data string) {
		t.Helper()
		if _, err := s.DB().Exec(
			`INSERT INTO resources (id, resource_type, data) VALUES (?, ?, ?)`,
			id, resourceType, data); err != nil {
			t.Fatalf("insert %s: %v", id, err)
		}
	}

	now := time.Now().UTC()
	n := 0
	seed := func(ago time.Duration, credits string, latency float64, isErr bool) {
		n++
		insert(fmt.Sprintf("run-%03d", n), "project_task_run", fmt.Sprintf(
			`{"task_id":"t-1","task_name":"Summarize","task_revision_id":"rev-1","model_name":"gpt-4o",`+
				`"created":%q,"is_error":%t,"total_tokens":500,"charged_credits":%q,"llm_call_timing":%v}`,
			now.Add(-ago).Format(time.RFC3339), isErr, credits, latency))
	}
	for i := 0; i < 25; i++ {
		seed(48*time.Hour, "9.00", 4, i == 0) // current window
		seed(10*24*time.Hour, "3.00", 2, false)
	}
	// Junk rows that must be skipped without aborting the scan.
	insert("junk-1", "project_task_run", `{"task_id":"t-1"}`)
	insert("junk-2", "project_task_run", `{"task_id":"t-2","created":"not-a-date"}`)
	insert("junk-3", "project_task_run", `{}`)
	if err := s.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	cmd := RootCmd()
	cmd.SetArgs([]string{"drift", "--db", dbPath, "--since", "7d", "--min-runs", "20", "--json", "--no-learn"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("drift: %v (stderr=%s)", err, stderr.String())
	}

	var report driftReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decoding drift output %q: %v", stdout.String(), err)
	}
	if report.TotalRunsCurrent != 25 || report.TotalRunsPrevious != 25 {
		t.Fatalf("totals = %d/%d, want 25/25 (junk rows must be skipped, not fatal)",
			report.TotalRunsCurrent, report.TotalRunsPrevious)
	}
	m := findDriftMover(t, report, "t-1")
	if m.Name != "Summarize" {
		t.Fatalf("name = %q, want %q", m.Name, "Summarize")
	}
	if m.MeanCreditsCurrent != 9 || m.MeanCreditsPrevious != 3 {
		t.Fatalf("mean credits = %v/%v, want 9/3 (string charged_credits must parse)",
			m.MeanCreditsCurrent, m.MeanCreditsPrevious)
	}
	if m.CreditsDeltaPct == nil || *m.CreditsDeltaPct != 200 {
		t.Fatalf("credits_delta_pct = %v, want 200", m.CreditsDeltaPct)
	}
	if m.P95Current != 4 || m.P95Previous != 2 {
		t.Fatalf("p95 = %v/%v, want 4/2", m.P95Current, m.P95Previous)
	}
	if m.P95DeltaPct == nil || *m.P95DeltaPct != 100 {
		t.Fatalf("p95_delta_pct = %v, want 100", m.P95DeltaPct)
	}
	if m.FailureRateCurrent != 0.04 {
		t.Fatalf("failure_rate_current = %v, want 0.04", m.FailureRateCurrent)
	}
	if m.TotalTokensCurrent != 12500 {
		t.Fatalf("total_tokens_current = %d, want 12500", m.TotalTokensCurrent)
	}
	if m.New {
		t.Fatal("group flagged new despite a populated previous window")
	}
	if report.FilteredOut != 0 {
		t.Fatalf("filtered_out = %d, want 0", report.FilteredOut)
	}
}

// TestDriftTotalsReconcileWithMovers is the arithmetic contract of the header:
// every run counted in total_runs_current must be accounted for by some mover
// row, or by filtered_out, or by unattributed_runs. Under --group-by agent the
// mirror also holds project_task_run rows, which carry no agent at all; they
// used to be added to the totals before the "no key" skip, so the header read
// "5,040 runs" above a movers table summing to 40, with filtered_out=0
// explaining the missing 5,000.
func TestDriftTotalsReconcileWithMovers(t *testing.T) {
	window := 7 * 24 * time.Hour
	rows := make([]driftRun, 0)
	// Plain task runs: no agent id, so nothing can attribute them under
	// --group-by agent.
	for i := 0; i < 500; i++ {
		rows = append(rows, driftRun{
			TaskID:   "t-1",
			TaskName: "Summarize",
			Created:  driftTestNow.Add(-24 * time.Hour),
		})
	}
	// Agent runs: attributable.
	for i := 0; i < 40; i++ {
		rows = append(rows, driftRun{
			TaskID:      "t-1",
			TaskAgentID: "agent-1",
			AgentName:   "Support bot",
			Created:     driftTestNow.Add(-24 * time.Hour),
		})
	}
	for i := 0; i < 10; i++ {
		rows = append(rows, driftRun{
			TaskID:      "t-1",
			TaskAgentID: "agent-1",
			AgentName:   "Support bot",
			Created:     driftTestNow.Add(-9 * 24 * time.Hour),
		})
	}

	report := buildDriftReport(rows, driftTestNow, window, "agent", 1)

	moverRuns := 0
	for _, m := range report.Movers {
		moverRuns += m.RunsCurrent
	}
	if moverRuns != report.TotalRunsCurrent {
		t.Fatalf("movers sum to %d current runs but the header claims %d; the totals must reconcile with what is shown",
			moverRuns, report.TotalRunsCurrent)
	}
	if report.TotalRunsCurrent != 40 {
		t.Errorf("total_runs_current = %d, want 40", report.TotalRunsCurrent)
	}
	if report.TotalRunsPrevious != 10 {
		t.Errorf("total_runs_previous = %d, want 10", report.TotalRunsPrevious)
	}
	if report.UnattributedRuns != 500 {
		t.Errorf("unattributed_runs = %d, want 500: the runs with no agent must be reported, not silently folded into the totals",
			report.UnattributedRuns)
	}
	if report.FilteredOut != 0 {
		t.Errorf("filtered_out = %d, want 0", report.FilteredOut)
	}

	// Grouping by task attributes every single run, so nothing is left over.
	byTask := buildDriftReport(rows, driftTestNow, window, "task", 1)
	if byTask.UnattributedRuns != 0 {
		t.Errorf("unattributed_runs = %d under --group-by task, want 0", byTask.UnattributedRuns)
	}
	if byTask.TotalRunsCurrent != 540 {
		t.Errorf("total_runs_current = %d under --group-by task, want 540", byTask.TotalRunsCurrent)
	}

	// Runs exist, but none can be grouped: the report must say so rather than
	// claim the mirror is empty.
	noneGroupable := buildDriftReport(rows[:500], driftTestNow, window, "agent", 1)
	if len(noneGroupable.Movers) != 0 {
		t.Fatalf("movers = %+v, want none", noneGroupable.Movers)
	}
	if !strings.Contains(noneGroupable.Note, "group by") {
		t.Errorf("note = %q, want it to explain that no run carried an agent to group by", noneGroupable.Note)
	}
	if noneGroupable.UnattributedRuns != 500 {
		t.Errorf("unattributed_runs = %d, want 500", noneGroupable.UnattributedRuns)
	}
}

// TestDriftHumanHeaderReportsUnattributedRuns renders the human table end to
// end against a real mirror. Under --group-by agent the mirror's
// project_task_run rows carry no agent, so the header's run counts and the
// movers beneath it are computed from different sets unless the unattributed
// runs are pulled out and named — which is what made a header of "5,040 runs"
// sit above a table summing to 40 with filtered_out=0.
func TestDriftHumanHeaderReportsUnattributedRuns(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	now := time.Now().UTC()
	insert := func(id, resourceType, data string) {
		t.Helper()
		if _, err := s.DB().Exec(
			`INSERT INTO resources (id, resource_type, data) VALUES (?, ?, ?)`,
			id, resourceType, data); err != nil {
			t.Fatalf("insert %s: %v", id, err)
		}
	}
	for i := 0; i < 30; i++ {
		insert(fmt.Sprintf("task-run-%03d", i), "project_task_run", fmt.Sprintf(
			`{"task_id":"t-1","task_name":"Summarize","created":%q,"is_error":false,"charged_credits":"9.00"}`,
			now.Add(-24*time.Hour).Format(time.RFC3339)))
	}
	for i := 0; i < 4; i++ {
		insert(fmt.Sprintf("agent-run-%03d", i), "project_task_agent_run", fmt.Sprintf(
			`{"task_id":"t-1","task_agent_id":"agent-1","task_agent_name":"Support bot","created":%q,"is_error":false,"charged_credits":"1.00"}`,
			now.Add(-24*time.Hour).Format(time.RFC3339)))
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	cmd := RootCmd()
	cmd.SetArgs([]string{"drift", "--db", dbPath, "--since", "7d", "--group-by", "agent",
		"--min-runs", "1", "--human-friendly", "--no-learn"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("drift: %v (stderr=%s)", err, stderr.String())
	}
	out := stdout.String()

	if !strings.Contains(out, "(4 runs)") {
		t.Errorf("header does not report 4 current runs — only the agent runs can be attributed:\n%s", out)
	}
	if strings.Contains(out, "(34 runs)") {
		t.Fatalf("header counts the 30 agent-less task runs it cannot show in any row:\n%s", out)
	}
	if !strings.Contains(out, "30 run(s)") || !strings.Contains(out, "carry no agent") {
		t.Errorf("output does not account for the 30 unattributable runs:\n%s", out)
	}
	if !strings.Contains(out, "Support bot") {
		t.Errorf("the one attributable group is missing from the table:\n%s", out)
	}
}
