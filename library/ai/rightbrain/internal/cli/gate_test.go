// Copyright 2026 Farouk Umar and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// gateRate builds the nullable pass_rate the API returns.
func gateRate(v float64) *float64 { return &v }

// TestGateHelpWires smoke-tests that the gate command resolves at runtime and
// renders useful --help output. Catches wiring regressions (missing
// AddCommand, panicking RunE on --help) before review.
func TestGateHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"gate", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gate --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{
		"Usage:", "gate <task_id>",
		"--min-pass-rate", "--revision-id", "--revision-tag", "--eval-set", "--wait", "--timeout-secs",
		"exit 3",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("gate --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestGatePasses covers the promote path: a completed run that clears both the
// absolute floor and the baseline must pass, expose a real delta, and produce
// no reasons and no exit error.
func TestGatePasses(t *testing.T) {
	tests := []struct {
		name          string
		run           gateEvalRun
		baseline      gateBaseline
		minPassRate   float64
		minSet        bool
		wantDelta     float64
		wantHasDelta  bool
		wantBaseFound bool
	}{
		{
			name: "above floor and above baseline",
			run: gateEvalRun{
				ID: "run-new", TaskID: "task-1", CandidateRevisionID: "rev-new",
				Status: "completed", PassCount: 19, FailCount: 1, PassRate: gateRate(0.95),
			},
			baseline: gateBaseline{
				Found: true, EvalRunID: "run-old", RevisionID: "rev-old",
				PassRate: 0.90, HasPassRate: true,
			},
			minPassRate: 0.9, minSet: true,
			wantDelta: 0.05, wantHasDelta: true, wantBaseFound: true,
		},
		{
			name: "exactly at the floor and exactly at the baseline still promotes",
			run: gateEvalRun{
				ID: "run-new", TaskID: "task-1", CandidateRevisionID: "rev-new",
				Status: "completed", PassCount: 9, FailCount: 1, PassRate: gateRate(0.9),
			},
			baseline: gateBaseline{
				Found: true, EvalRunID: "run-old", RevisionID: "rev-old",
				PassRate: 0.9, HasPassRate: true,
			},
			minPassRate: 0.9, minSet: true,
			wantDelta: 0, wantHasDelta: true, wantBaseFound: true,
		},
		{
			name: "no floor given, improvement over baseline promotes",
			run: gateEvalRun{
				ID: "run-new", TaskID: "task-1", CandidateRevisionID: "rev-new",
				Status: "completed", PassCount: 5, FailCount: 5, PassRate: gateRate(0.5),
			},
			baseline: gateBaseline{
				Found: true, EvalRunID: "run-old", RevisionID: "rev-old",
				PassRate: 0.4, HasPassRate: true,
			},
			wantDelta: 0.1, wantHasDelta: true, wantBaseFound: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			report := evaluateGate(tc.run, tc.baseline, tc.minPassRate, tc.minSet)
			if !report.Passed {
				t.Fatalf("Passed = false, want true; reasons = %v", report.Reasons)
			}
			if len(report.Reasons) != 0 {
				t.Fatalf("Reasons = %v, want empty", report.Reasons)
			}
			if report.BaselineFound != tc.wantBaseFound {
				t.Fatalf("BaselineFound = %v, want %v", report.BaselineFound, tc.wantBaseFound)
			}
			if tc.wantHasDelta {
				if report.Delta == nil {
					t.Fatal("Delta = nil, want a computed delta")
				}
				if diff := *report.Delta - tc.wantDelta; diff > 1e-9 || diff < -1e-9 {
					t.Fatalf("Delta = %v, want %v", *report.Delta, tc.wantDelta)
				}
			}
			if report.BaselineEvalRunID != tc.baseline.EvalRunID {
				t.Fatalf("BaselineEvalRunID = %q, want %q", report.BaselineEvalRunID, tc.baseline.EvalRunID)
			}
			if err := gateExitStatus(report); err != nil {
				t.Fatalf("gateExitStatus = %v, want nil for a passing gate", err)
			}
		})
	}
}

// TestGateFailsBelowMinPassRate covers the absolute-floor path: below
// --min-pass-rate the gate must fail, say so in reasons, and surface exit 3.
func TestGateFailsBelowMinPassRate(t *testing.T) {
	tests := []struct {
		name        string
		run         gateEvalRun
		baseline    gateBaseline
		minPassRate float64
		wantReason  string
		wantReasons int
	}{
		{
			name: "below floor with no baseline",
			run: gateEvalRun{
				ID: "run-new", TaskID: "task-1", CandidateRevisionID: "rev-new",
				Status: "completed", PassCount: 7, FailCount: 3, PassRate: gateRate(0.7),
			},
			minPassRate: 0.9,
			wantReason:  "below the required --min-pass-rate",
			wantReasons: 1,
		},
		{
			name: "below floor and below baseline reports both reasons",
			run: gateEvalRun{
				ID: "run-new", TaskID: "task-1", CandidateRevisionID: "rev-new",
				Status: "completed", PassCount: 7, FailCount: 3, PassRate: gateRate(0.7),
			},
			baseline: gateBaseline{
				Found: true, EvalRunID: "run-old", RevisionID: "rev-old",
				PassRate: 0.95, HasPassRate: true,
			},
			minPassRate: 0.9,
			wantReason:  "below the required --min-pass-rate",
			wantReasons: 2,
		},
		{
			name: "eval run that failed outright is never promotable",
			run: gateEvalRun{
				ID: "run-new", TaskID: "task-1", CandidateRevisionID: "rev-new",
				Status: "failed", ErrorMessage: "judge model unavailable",
			},
			minPassRate: 0.9,
			wantReason:  "judge model unavailable",
			wantReasons: 2, // failed status + absent pass_rate
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			report := evaluateGate(tc.run, tc.baseline, tc.minPassRate, true)
			if report.Passed {
				t.Fatal("Passed = true, want false")
			}
			if len(report.Reasons) != tc.wantReasons {
				t.Fatalf("len(Reasons) = %d, want %d: %v", len(report.Reasons), tc.wantReasons, report.Reasons)
			}
			if !strings.Contains(strings.Join(report.Reasons, " | "), tc.wantReason) {
				t.Fatalf("Reasons = %v, want one containing %q", report.Reasons, tc.wantReason)
			}
			if report.MinPassRate == nil || *report.MinPassRate != tc.minPassRate {
				t.Fatalf("MinPassRate = %v, want %v echoed back", report.MinPassRate, tc.minPassRate)
			}
			err := gateExitStatus(report)
			if err == nil {
				t.Fatal("gateExitStatus = nil, want a typed exit-3 error")
			}
			var typed *cliError
			if !errors.As(err, &typed) {
				t.Fatalf("gateExitStatus returned %T, want *cliError", err)
			}
			if typed.code != 3 {
				t.Fatalf("exit code = %d, want 3", typed.code)
			}
		})
	}
}

// TestGateFailsOnBaselineRegression covers the regression path: a candidate
// that clears the absolute floor but scores below the recorded baseline must
// still be held, with the baseline named in the reason and a negative delta.
func TestGateFailsOnBaselineRegression(t *testing.T) {
	tests := []struct {
		name      string
		candidate float64
		baseline  float64
		minSet    bool
		min       float64
		wantDelta float64
	}{
		{name: "clears floor but regresses", candidate: 0.91, baseline: 0.97, minSet: true, min: 0.9, wantDelta: -0.06},
		{name: "no floor set, regression alone holds", candidate: 0.80, baseline: 0.85, wantDelta: -0.05},
		{name: "hairline regression still holds", candidate: 0.8999, baseline: 0.9, wantDelta: -0.0001},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			run := gateEvalRun{
				ID: "run-new", TaskID: "task-1", CandidateRevisionID: "rev-new",
				Status: "completed", PassRate: gateRate(tc.candidate),
			}
			baseline := gateBaseline{
				Found: true, EvalRunID: "run-old", RevisionID: "rev-old",
				PassRate: tc.baseline, HasPassRate: true,
			}
			report := evaluateGate(run, baseline, tc.min, tc.minSet)
			if report.Passed {
				t.Fatalf("Passed = true, want false (candidate %v vs baseline %v)", tc.candidate, tc.baseline)
			}
			joined := strings.Join(report.Reasons, " | ")
			if !strings.Contains(joined, "regressed against baseline") {
				t.Fatalf("Reasons = %v, want one naming the baseline regression", report.Reasons)
			}
			if !strings.Contains(joined, "rev-old") {
				t.Fatalf("Reasons = %v, want the baseline revision id named", report.Reasons)
			}
			if report.Delta == nil {
				t.Fatal("Delta = nil, want a negative delta")
			}
			if diff := *report.Delta - tc.wantDelta; diff > 1e-9 || diff < -1e-9 {
				t.Fatalf("Delta = %v, want %v", *report.Delta, tc.wantDelta)
			}
			if report.BaselinePassRate == nil || *report.BaselinePassRate != tc.baseline {
				t.Fatalf("BaselinePassRate = %v, want %v", report.BaselinePassRate, tc.baseline)
			}
		})
	}
}

// TestGateNoBaselineIsNotARegression is the absence-of-correctness case: a
// first-ever gate has nothing to compare against, so merely meeting
// --min-pass-rate must PASS, and the report must not fabricate a baseline or a
// delta out of the zero value.
func TestGateNoBaselineIsNotARegression(t *testing.T) {
	tests := []struct {
		name       string
		passRate   float64
		min        float64
		minSet     bool
		wantPassed bool
	}{
		{name: "meets the floor exactly", passRate: 0.9, min: 0.9, minSet: true, wantPassed: true},
		{name: "well above the floor", passRate: 0.99, min: 0.9, minSet: true, wantPassed: true},
		{name: "low absolute rate with no floor demanded", passRate: 0.10, wantPassed: true},
		{name: "below the floor still fails without a baseline", passRate: 0.5, min: 0.9, minSet: true, wantPassed: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			run := gateEvalRun{
				ID: "run-first", TaskID: "task-1", CandidateRevisionID: "rev-first",
				Status: "completed", PassCount: 9, FailCount: 1, PassRate: gateRate(tc.passRate),
			}
			report := evaluateGate(run, gateBaseline{}, tc.min, tc.minSet)

			if report.BaselineFound {
				t.Fatal("BaselineFound = true, want false")
			}
			if report.Passed != tc.wantPassed {
				t.Fatalf("Passed = %v, want %v; reasons = %v", report.Passed, tc.wantPassed, report.Reasons)
			}
			// The whole point: absence of a baseline must never be reported as
			// a regression, and must never be materialized as 0.0.
			if report.BaselinePassRate != nil {
				t.Fatalf("BaselinePassRate = %v, want nil (no baseline exists)", *report.BaselinePassRate)
			}
			if report.Delta != nil {
				t.Fatalf("Delta = %v, want nil (nothing to subtract from)", *report.Delta)
			}
			if report.BaselineRevisionID != "" || report.BaselineEvalRunID != "" {
				t.Fatalf("baseline ids = (%q, %q), want empty", report.BaselineRevisionID, report.BaselineEvalRunID)
			}
			for _, r := range report.Reasons {
				if strings.Contains(r, "regressed") {
					t.Fatalf("reason %q treats a missing baseline as a regression", r)
				}
			}

			// The JSON contract callers parse: nulls, not zeros, and reasons
			// always an array.
			raw, err := json.Marshal(report)
			if err != nil {
				t.Fatalf("marshal report: %v", err)
			}
			body := string(raw)
			for _, want := range []string{`"baseline_pass_rate":null`, `"delta":null`, `"baseline_found":false`} {
				if !strings.Contains(body, want) {
					t.Fatalf("report JSON missing %s:\n%s", want, body)
				}
			}
			if tc.wantPassed && !strings.Contains(body, `"reasons":[]`) {
				t.Fatalf("report JSON should carry an empty reasons array, got:\n%s", body)
			}
		})
	}
}

// TestGatePollAndBaselineSelection covers the two supporting rules: --wait
// stops at a terminal status (and reports a non-terminal run honestly rather
// than as a verdict), and the baseline picker takes the most recent completed
// run for a *different* revision.
func TestGatePollAndBaselineSelection(t *testing.T) {
	t.Run("poll stops at a terminal status", func(t *testing.T) {
		statuses := []string{"running", "completed"}
		calls := 0
		fetch := func(context.Context) (gateEvalRun, error) {
			s := statuses[calls]
			calls++
			return gateEvalRun{ID: "run-1", Status: s, PassRate: gateRate(1)}, nil
		}
		run, terminal, err := pollGateEvalRun(context.Background(),
			gateEvalRun{ID: "run-1", Status: "pending"}, fetch,
			time.Now().Add(time.Second), time.Millisecond, 0)
		if err != nil {
			t.Fatalf("pollGateEvalRun error = %v", err)
		}
		if !terminal || run.Status != "completed" {
			t.Fatalf("terminal = %v, status = %q; want true/completed", terminal, run.Status)
		}
		if calls != 2 {
			t.Fatalf("fetch calls = %d, want 2", calls)
		}
	})

	t.Run("single-poll clamp reports a still-running eval as non-terminal", func(t *testing.T) {
		fetch := func(context.Context) (gateEvalRun, error) {
			return gateEvalRun{ID: "run-1", Status: "running"}, nil
		}
		run, terminal, err := pollGateEvalRun(context.Background(),
			gateEvalRun{ID: "run-1", Status: "pending"}, fetch,
			time.Now().Add(time.Second), time.Millisecond, 1)
		if err != nil {
			t.Fatalf("pollGateEvalRun error = %v", err)
		}
		if terminal {
			t.Fatal("terminal = true, want false for a run that never completed")
		}
		if run.Status != "running" {
			t.Fatalf("status = %q, want the last observed status", run.Status)
		}
	})

	t.Run("baseline picks the most recent completed other-revision run", func(t *testing.T) {
		stored := []gateStoredRun{
			{id: "r1", revisionID: "rev-old", status: "completed", passRate: 0.5, hasPassRate: true, completedAt: "2026-07-01T00:00:00Z"},
			{id: "r2", revisionID: "rev-old", status: "completed", passRate: 0.8, hasPassRate: true, completedAt: "2026-07-20T00:00:00Z"},
			{id: "r3", revisionID: "rev-old", status: "running", hasPassRate: false, created: "2026-07-30T00:00:00Z"},
			{id: "r4", revisionID: "rev-new", status: "completed", passRate: 0.99, hasPassRate: true, completedAt: "2026-07-31T00:00:00Z"},
		}
		best, ok := pickGateBaseline(stored, "rev-new", "r4")
		if !ok {
			t.Fatal("pickGateBaseline found nothing, want r2")
		}
		if best.id != "r2" {
			t.Fatalf("baseline = %q, want r2 (newest completed run of a different revision)", best.id)
		}
	})

	t.Run("no comparable row means no baseline", func(t *testing.T) {
		stored := []gateStoredRun{
			{id: "r4", revisionID: "rev-new", status: "completed", passRate: 0.99, hasPassRate: true},
			{id: "r5", revisionID: "rev-old", status: "running", hasPassRate: false},
		}
		if _, ok := pickGateBaseline(stored, "rev-new", ""); ok {
			t.Fatal("pickGateBaseline found a baseline, want none")
		}
	})
}
