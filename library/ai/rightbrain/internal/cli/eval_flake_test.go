// Copyright 2026 Farouk Umar and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/rightbrain/internal/store"
)

// evalRun is a compact constructor for a synthetic eval run: one run against
// one candidate revision, carrying one verdict per named test case.
func evalRun(id, created, revision string, verdicts map[string]string) evalRunRecord {
	run := evalRunRecord{
		ID:                  id,
		TaskID:              "task-1",
		EvalSetID:           "set-1",
		CandidateRevisionID: revision,
		Status:              "completed",
		Created:             created,
	}
	refs := make([]string, 0, len(verdicts))
	for ref := range verdicts {
		refs = append(refs, ref)
	}
	// Deterministic ordering keeps per-case assertions stable across runs.
	sort.Strings(refs)
	for _, ref := range refs {
		verdict := verdicts[ref]
		if verdict == "pass" {
			run.PassCount++
		} else {
			run.FailCount++
		}
		run.Results = append(run.Results, evalResultRecord{
			ID:             id + ":" + ref,
			EvalRunID:      id,
			ReferenceRunID: ref,
			CandidateRunID: id + ":cand:" + ref,
			Verdict:        verdict,
			Created:        created,
		})
	}
	return run
}

// evalErrorRun is an eval run in which every named case errored out. This is
// the shape the API actually returns for an errored result: EvalVerdict is
// nullable and is_error is a separate required boolean, so the verdict is
// absent entirely and only is_error says what happened.
func evalErrorRun(id, created, revision string, refs []string, message string) evalRunRecord {
	run := evalRunRecord{
		ID:                  id,
		TaskID:              "task-1",
		EvalSetID:           "set-1",
		CandidateRevisionID: revision,
		Status:              "completed",
		Created:             created,
		ErrorCount:          len(refs),
	}
	sorted := append([]string(nil), refs...)
	sort.Strings(sorted)
	for _, ref := range sorted {
		run.Results = append(run.Results, evalResultRecord{
			ID:             id + ":" + ref,
			EvalRunID:      id,
			ReferenceRunID: ref,
			CandidateRunID: id + ":cand:" + ref,
			Verdict:        "", // verdict is null on an errored result
			IsError:        true,
			ErrorMessage:   message,
			Created:        created,
		})
	}
	return run
}

func findEvalCase(t *testing.T, report evalFlakeReport, ref string) evalFlakeCase {
	t.Helper()
	for _, c := range report.Cases {
		if c.ReferenceRunID == ref {
			return c
		}
	}
	t.Fatalf("case %q not present in report; got %d case(s): %+v", ref, len(report.Cases), report.Cases)
	return evalFlakeCase{}
}

// TestEvalFlakeSameRevisionBothVerdictsIsFlaky covers the core rule: a case
// that both passed and failed while running against the SAME candidate
// revision is genuine nondeterminism.
func TestEvalFlakeSameRevisionBothVerdictsIsFlaky(t *testing.T) {
	runs := []evalRunRecord{
		evalRun("run-1", "2026-07-01T10:00:00Z", "rev-a", map[string]string{"case-flaky": "pass"}),
		evalRun("run-2", "2026-07-01T11:00:00Z", "rev-a", map[string]string{"case-flaky": "fail"}),
		evalRun("run-3", "2026-07-01T12:00:00Z", "rev-a", map[string]string{"case-flaky": "pass"}),
	}
	report := buildEvalFlakeReport(runs, 10)

	if report.EvalRunsExamined != 3 {
		t.Fatalf("eval_runs_examined = %d, want 3", report.EvalRunsExamined)
	}
	c := findEvalCase(t, report, "case-flaky")
	if !c.Flaky {
		t.Errorf("flaky = false, want true (pass and fail on the same revision rev-a)")
	}
	if c.Classification != "flaky" {
		t.Errorf("classification = %q, want %q", c.Classification, "flaky")
	}
	if c.ConsistentFailure {
		t.Errorf("consistent_failure = true, want false (the case passed twice)")
	}
	if c.Runs != 3 || c.Fails != 1 || c.Passes != 2 {
		t.Errorf("runs/fails/passes = %d/%d/%d, want 3/1/2", c.Runs, c.Fails, c.Passes)
	}
	if len(c.RevisionsSeen) != 1 || c.RevisionsSeen[0] != "rev-a" {
		t.Errorf("revisions_seen = %v, want [rev-a]", c.RevisionsSeen)
	}
	if report.FlakyCount != 1 {
		t.Errorf("flaky_count = %d, want 1", report.FlakyCount)
	}
	if report.Cases[0].ReferenceRunID != "case-flaky" {
		t.Errorf("flaky case should rank first, got %q", report.Cases[0].ReferenceRunID)
	}
}

// TestEvalFlakeAlwaysFailsIsConsistentFailure covers the opposite pole: a case
// that fails in every single run is a real defect, not flake.
func TestEvalFlakeAlwaysFailsIsConsistentFailure(t *testing.T) {
	runs := []evalRunRecord{
		evalRun("run-1", "2026-07-01T10:00:00Z", "rev-a", map[string]string{"case-broken": "fail"}),
		evalRun("run-2", "2026-07-01T11:00:00Z", "rev-a", map[string]string{"case-broken": "fail"}),
		evalRun("run-3", "2026-07-01T12:00:00Z", "rev-b", map[string]string{"case-broken": "fail"}),
	}
	report := buildEvalFlakeReport(runs, 10)

	c := findEvalCase(t, report, "case-broken")
	if !c.ConsistentFailure {
		t.Errorf("consistent_failure = false, want true (fails in all %d runs)", c.Runs)
	}
	if c.Classification != "consistent-failure" {
		t.Errorf("classification = %q, want %q", c.Classification, "consistent-failure")
	}
	if c.FailRate != 1.0 {
		t.Errorf("fail_rate = %v, want 1.0", c.FailRate)
	}
	if c.Flaky {
		t.Errorf("flaky = true, want false: a case that never passes shows no nondeterminism")
	}
	if report.ConsistentFailureCount != 1 {
		t.Errorf("consistent_failure_count = %d, want 1", report.ConsistentFailureCount)
	}
	if report.FlakyCount != 0 {
		t.Errorf("flaky_count = %d, want 0", report.FlakyCount)
	}
}

// TestEvalFlakeAlwaysPassesIsStablePass asserts healthy cases are neither
// flagged nor counted.
func TestEvalFlakeAlwaysPassesIsStablePass(t *testing.T) {
	runs := []evalRunRecord{
		evalRun("run-1", "2026-07-01T10:00:00Z", "rev-a", map[string]string{"case-good": "pass"}),
		evalRun("run-2", "2026-07-01T11:00:00Z", "rev-a", map[string]string{"case-good": "pass"}),
		evalRun("run-3", "2026-07-01T12:00:00Z", "rev-b", map[string]string{"case-good": "pass"}),
	}
	report := buildEvalFlakeReport(runs, 10)

	c := findEvalCase(t, report, "case-good")
	if c.Classification != "stable-pass" {
		t.Errorf("classification = %q, want %q", c.Classification, "stable-pass")
	}
	if c.Flaky || c.ConsistentFailure {
		t.Errorf("flaky=%v consistent_failure=%v, want both false", c.Flaky, c.ConsistentFailure)
	}
	if c.FailRate != 0 || c.Fails != 0 {
		t.Errorf("fail_rate=%v fails=%d, want 0/0", c.FailRate, c.Fails)
	}
	if report.FlakyCount != 0 {
		t.Errorf("flaky_count = %d, want 0: a case that always passes must not be counted as flaky", report.FlakyCount)
	}
	if report.ConsistentFailureCount != 0 {
		t.Errorf("consistent_failure_count = %d, want 0", report.ConsistentFailureCount)
	}
	if report.TotalCases != 1 {
		t.Errorf("total_cases = %d, want 1", report.TotalCases)
	}
}

// TestEvalFlakeSingleRunProvesNothing is the absence-of-correctness case: one
// eval run is a single observation, and a single observation cannot
// distinguish a nondeterministic judge from a genuine defect. Nothing may be
// classified flaky, and the report must say why.
func TestEvalFlakeSingleRunProvesNothing(t *testing.T) {
	cases := []struct {
		name     string
		verdicts map[string]string
	}{
		{"single run, one failure", map[string]string{"case-a": "fail"}},
		{"single run, mixed verdicts", map[string]string{"case-a": "fail", "case-b": "pass"}},
		{"single run, all passing", map[string]string{"case-a": "pass", "case-b": "pass"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runs := []evalRunRecord{
				evalRun("run-1", "2026-07-01T10:00:00Z", "rev-a", tc.verdicts),
			}
			report := buildEvalFlakeReport(runs, 10)

			if report.EvalRunsExamined != 1 {
				t.Fatalf("eval_runs_examined = %d, want 1", report.EvalRunsExamined)
			}
			if report.FlakyCount != 0 {
				t.Errorf("flaky_count = %d, want 0: flake is unprovable from a single eval run", report.FlakyCount)
			}
			for _, c := range report.Cases {
				if c.Flaky {
					t.Errorf("case %q flaky = true, want false with only one observation", c.ReferenceRunID)
				}
				if c.Classification == "flaky" {
					t.Errorf("case %q classification = %q, want anything but %q", c.ReferenceRunID, c.Classification, "flaky")
				}
				if c.ConsistentFailure {
					t.Errorf("case %q consistent_failure = true, want false: one run is not a repeated failure", c.ReferenceRunID)
				}
			}
			if report.Note == "" {
				t.Fatal("note is empty; the report must state that the history is insufficient")
			}
			lower := strings.ToLower(report.Note)
			if !strings.Contains(lower, "insufficient history") {
				t.Errorf("note = %q, want it to name the insufficient history", report.Note)
			}
			if !strings.Contains(lower, "repeated runs") {
				t.Errorf("note = %q, want it to explain that repeated runs are required", report.Note)
			}
		})
	}
}

// TestEvalFlakeDifferentRevisionIsIntermittentNotFlaky is the second
// absence-of-correctness case: a case that passed on one revision and failed
// on a different one is a regression at a revision boundary. The inputs
// changed, so the differing verdicts are not evidence of nondeterminism and
// must never be reported as flake.
func TestEvalFlakeDifferentRevisionIsIntermittentNotFlaky(t *testing.T) {
	runs := []evalRunRecord{
		evalRun("run-1", "2026-07-01T10:00:00Z", "rev-old", map[string]string{"case-regressed": "pass"}),
		evalRun("run-2", "2026-07-01T11:00:00Z", "rev-old", map[string]string{"case-regressed": "pass"}),
		evalRun("run-3", "2026-07-01T12:00:00Z", "rev-new", map[string]string{"case-regressed": "fail"}),
	}
	report := buildEvalFlakeReport(runs, 10)

	c := findEvalCase(t, report, "case-regressed")
	if c.Flaky {
		t.Errorf("flaky = true, want false: the verdict changed only when the candidate revision changed")
	}
	if c.Classification != "intermittent" {
		t.Errorf("classification = %q, want %q (a revision-boundary regression, not nondeterminism)", c.Classification, "intermittent")
	}
	if c.ConsistentFailure {
		t.Errorf("consistent_failure = true, want false: the case passed on rev-old")
	}
	if report.FlakyCount != 0 {
		t.Errorf("flaky_count = %d, want 0", report.FlakyCount)
	}
	if len(c.RevisionsSeen) != 2 {
		t.Errorf("revisions_seen = %v, want two distinct revisions", c.RevisionsSeen)
	}
	if c.Fails != 1 || c.Passes != 2 || c.Runs != 3 {
		t.Errorf("fails/passes/runs = %d/%d/%d, want 1/2/3", c.Fails, c.Passes, c.Runs)
	}

	// The contrast that makes the distinction load-bearing: the identical
	// verdict history, but all against ONE revision, IS flaky.
	sameRev := []evalRunRecord{
		evalRun("run-1", "2026-07-01T10:00:00Z", "rev-old", map[string]string{"case-regressed": "pass"}),
		evalRun("run-2", "2026-07-01T11:00:00Z", "rev-old", map[string]string{"case-regressed": "pass"}),
		evalRun("run-3", "2026-07-01T12:00:00Z", "rev-old", map[string]string{"case-regressed": "fail"}),
	}
	sameReport := buildEvalFlakeReport(sameRev, 10)
	sameCase := findEvalCase(t, sameReport, "case-regressed")
	if !sameCase.Flaky || sameCase.Classification != "flaky" {
		t.Errorf("same-revision history: flaky=%v classification=%q, want true/%q — otherwise the revision check is doing nothing",
			sameCase.Flaky, sameCase.Classification, "flaky")
	}
}

// TestEvalFlakeUnknownRevisionIsNotFlaky guards the conservative rule: when
// candidate_revision_id is absent, two differing verdicts cannot be proven to
// come from identical inputs, so the case must not be called flaky.
func TestEvalFlakeUnknownRevisionIsNotFlaky(t *testing.T) {
	runs := []evalRunRecord{
		evalRun("run-1", "2026-07-01T10:00:00Z", "", map[string]string{"case-x": "pass"}),
		evalRun("run-2", "2026-07-01T11:00:00Z", "", map[string]string{"case-x": "fail"}),
	}
	report := buildEvalFlakeReport(runs, 10)

	c := findEvalCase(t, report, "case-x")
	if c.Flaky {
		t.Errorf("flaky = true, want false: the candidate revision is unknown, so sameness of inputs is unproven")
	}
	if c.Classification != "intermittent" {
		t.Errorf("classification = %q, want %q", c.Classification, "intermittent")
	}
	if len(c.RevisionsSeen) != 0 {
		t.Errorf("revisions_seen = %v, want empty", c.RevisionsSeen)
	}
}

// TestEvalFlakeLastWindowAndRanking checks that --last trims to the newest
// runs by created time and that output ordering is flaky, then consistent
// failure, then intermittent, then stable-pass.
func TestEvalFlakeLastWindowAndRanking(t *testing.T) {
	runs := []evalRunRecord{
		// Deliberately out of chronological order in the input slice.
		evalRun("run-old", "2026-06-01T10:00:00Z", "rev-ancient", map[string]string{
			"case-flaky": "pass", "case-broken": "pass", "case-good": "pass"}),
		evalRun("run-3", "2026-07-01T12:00:00Z", "rev-a", map[string]string{
			"case-flaky": "fail", "case-broken": "fail", "case-good": "pass"}),
		evalRun("run-1", "2026-07-01T10:00:00Z", "rev-a", map[string]string{
			"case-flaky": "pass", "case-broken": "fail", "case-good": "pass"}),
		evalRun("run-2", "2026-07-01T11:00:00Z", "rev-a", map[string]string{
			"case-flaky": "fail", "case-broken": "fail", "case-good": "pass"}),
	}

	report := buildEvalFlakeReport(runs, 3)
	if report.EvalRunsExamined != 3 {
		t.Fatalf("eval_runs_examined = %d, want 3 (--last 3 must trim run-old)", report.EvalRunsExamined)
	}
	broken := findEvalCase(t, report, "case-broken")
	if broken.Runs != 3 || broken.Fails != 3 {
		t.Errorf("case-broken runs/fails = %d/%d, want 3/3 — the older passing run must fall outside the window",
			broken.Runs, broken.Fails)
	}
	if broken.Classification != "consistent-failure" {
		t.Errorf("case-broken classification = %q, want %q", broken.Classification, "consistent-failure")
	}
	if report.TotalCases != 3 {
		t.Errorf("total_cases = %d, want 3", report.TotalCases)
	}

	got := make([]string, 0, len(report.Cases))
	for _, c := range report.Cases {
		got = append(got, c.Classification)
	}
	rank := map[string]int{"flaky": 0, "consistent-failure": 1, "intermittent": 2, "stable-pass": 3}
	for i := 1; i < len(got); i++ {
		if rank[got[i-1]] > rank[got[i]] {
			t.Fatalf("cases out of rank order: %v", got)
		}
	}
	if got[0] != "flaky" {
		t.Errorf("first case classification = %q, want %q; ranking = %v", got[0], "flaky", got)
	}
	if got[len(got)-1] != "stable-pass" {
		t.Errorf("last case classification = %q, want %q; ranking = %v", got[len(got)-1], "stable-pass", got)
	}
}

// TestEvalFlakeNoRunsReportsEmptyNotFlake asserts an empty mirror produces an
// empty, JSON-safe report rather than a fabricated verdict.
func TestEvalFlakeNoRunsReportsEmptyNotFlake(t *testing.T) {
	report := buildEvalFlakeReport(nil, 10)
	if report.Cases == nil {
		t.Fatal("cases is nil; it must be an empty slice so it marshals as [] not null")
	}
	if report.TotalCases != 0 || report.FlakyCount != 0 || report.ConsistentFailureCount != 0 {
		t.Errorf("counts = %d/%d/%d, want all zero",
			report.TotalCases, report.FlakyCount, report.ConsistentFailureCount)
	}
	if !strings.Contains(report.Note, "no eval runs") {
		t.Errorf("note = %q, want it to state that no eval runs are present", report.Note)
	}
}

// TestEvalFlakeHelpWires smoke-tests that eval-flake resolves at runtime and
// renders the documented usage and disambiguation guidance.
func TestEvalFlakeHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"eval-flake", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("eval-flake --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{
		"Usage:",
		"eval-flake <task_id>",
		"--eval-set",
		"--last",
		"use 'gate' instead",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("eval-flake --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestEvalFlakeErroredCaseIsNeverStablePass is the whole point of tracking
// is_error: an eval result that errored carries no verdict at all (EvalVerdict
// is nullable, is_error is the separate required flag), so a case that errored
// in every run has Fails == 0. Counting only "fail" verdicts made it fall
// through every branch to stable-pass with fail_rate 0.00 and sort to the
// bottom of the table — a permanently broken case reported as the healthiest
// thing in the project.
func TestEvalFlakeErroredCaseIsNeverStablePass(t *testing.T) {
	runs := []evalRunRecord{
		evalErrorRun("run-1", "2026-07-01T10:00:00Z", "rev-a", []string{"case-blown"}, "judge timed out"),
		evalErrorRun("run-2", "2026-07-01T11:00:00Z", "rev-a", []string{"case-blown"}, "judge timed out"),
		evalErrorRun("run-3", "2026-07-01T12:00:00Z", "rev-a", []string{"case-blown"}, "judge timed out"),
	}
	report := buildEvalFlakeReport(runs, 10)

	c := findEvalCase(t, report, "case-blown")
	if c.Runs != 3 || c.Errors != 3 {
		t.Fatalf("runs/errors = %d/%d, want 3/3", c.Runs, c.Errors)
	}
	if c.Passes != 0 {
		t.Errorf("passes = %d, want 0: an errored result is not a pass", c.Passes)
	}
	if c.Classification == "stable-pass" {
		t.Fatalf("classification = %q for a case that errored in all 3 runs; it must never read as healthy", c.Classification)
	}
	if c.Classification != "errored" {
		t.Errorf("classification = %q, want %q", c.Classification, "errored")
	}
	if !c.Errored {
		t.Errorf("errored = false, want true")
	}
	if c.FailRate == 0 {
		t.Fatalf("fail_rate = 0 for a case that never once passed; errored observations must count on the failure side")
	}
	if c.FailRate != 1 {
		t.Errorf("fail_rate = %v, want 1 (3 non-passes over 3 runs)", c.FailRate)
	}
	if c.NonPasses != 3 {
		t.Errorf("non_passes = %d, want 3", c.NonPasses)
	}
	if report.ErroredCount != 1 {
		t.Errorf("errored_count = %d, want 1", report.ErroredCount)
	}
	if c.LastVerdict != "error" {
		t.Errorf("last_verdict = %q, want %q", c.LastVerdict, "error")
	}
	if c.ErrorMessage != "judge timed out" {
		t.Errorf("error_message = %q, want the API's message carried through", c.ErrorMessage)
	}
	if c.Flaky {
		t.Errorf("flaky = true, want false: errors are not evidence of a nondeterministic judge")
	}

	// Ranking: the broken case must outrank the healthy one, not sort beneath it.
	mixed := []evalRunRecord{
		evalRun("run-1", "2026-07-01T10:00:00Z", "rev-a", map[string]string{"case-good": "pass"}),
		evalRun("run-2", "2026-07-01T11:00:00Z", "rev-a", map[string]string{"case-good": "pass"}),
		evalErrorRun("run-1e", "2026-07-01T10:30:00Z", "rev-a", []string{"case-blown"}, "boom"),
		evalErrorRun("run-2e", "2026-07-01T11:30:00Z", "rev-a", []string{"case-blown"}, "boom"),
	}
	mixedReport := buildEvalFlakeReport(mixed, 10)
	if len(mixedReport.Cases) != 2 {
		t.Fatalf("got %d cases, want 2: %+v", len(mixedReport.Cases), mixedReport.Cases)
	}
	if mixedReport.Cases[0].ReferenceRunID != "case-blown" {
		t.Errorf("cases[0] = %q (%s), want case-blown ranked above the stable-pass case",
			mixedReport.Cases[0].ReferenceRunID, mixedReport.Cases[0].Classification)
	}
	if evalFlakeRank["errored"] >= evalFlakeRank["stable-pass"] {
		t.Errorf("errored ranks %d and stable-pass ranks %d; errored must rank above stable-pass",
			evalFlakeRank["errored"], evalFlakeRank["stable-pass"])
	}
}

// TestEvalFlakeErrorsMixedWithVerdicts pins the boundaries between the errored
// classification and its neighbours: an error alongside a real fail verdict is
// still a consistent failure (there is a genuine red assertion to fix), and an
// error alongside a pass is intermittent, not stable.
func TestEvalFlakeErrorsMixedWithVerdicts(t *testing.T) {
	failsAndErrors := []evalRunRecord{
		evalRun("run-1", "2026-07-01T10:00:00Z", "rev-a", map[string]string{"case-x": "fail"}),
		evalRun("run-2", "2026-07-01T11:00:00Z", "rev-a", map[string]string{"case-x": "fail"}),
		evalErrorRun("run-3", "2026-07-01T12:00:00Z", "rev-a", []string{"case-x"}, "boom"),
	}
	c := findEvalCase(t, buildEvalFlakeReport(failsAndErrors, 10), "case-x")
	if c.Classification != "consistent-failure" {
		t.Errorf("classification = %q, want %q: it never passed and it has a real fail verdict", c.Classification, "consistent-failure")
	}
	if c.FailRate != 1 {
		t.Errorf("fail_rate = %v, want 1 (2 fails + 1 error over 3 runs)", c.FailRate)
	}

	passAndError := []evalRunRecord{
		evalRun("run-1", "2026-07-01T10:00:00Z", "rev-a", map[string]string{"case-y": "pass"}),
		evalErrorRun("run-2", "2026-07-01T11:00:00Z", "rev-a", []string{"case-y"}, "boom"),
	}
	y := findEvalCase(t, buildEvalFlakeReport(passAndError, 10), "case-y")
	if y.Classification == "stable-pass" {
		t.Fatalf("classification = %q, want anything but stable-pass: one of the two observations errored", y.Classification)
	}
	if y.Classification != "intermittent" {
		t.Errorf("classification = %q, want %q", y.Classification, "intermittent")
	}
	if y.FailRate != 0.5 {
		t.Errorf("fail_rate = %v, want 0.5 (1 error over 2 runs)", y.FailRate)
	}
	if y.Flaky {
		t.Errorf("flaky = true, want false: an error produced no verdict, so it cannot prove nondeterminism")
	}
}

// TestLoadEvalRunsForTaskHonorsContext proves the mirror read runs under the
// caller's --timeout-bounded context. It previously queried with cmd.Context(),
// so --timeout was silently ignored for the only I/O this command does.
func TestLoadEvalRunsForTaskHonorsContext(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("opening test store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	run := evalRun("run-1", "2026-07-01T10:00:00Z", "rev-a", map[string]string{"case-a": "fail"})
	blob, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("marshaling run: %v", err)
	}
	if _, err := db.DB().Exec(
		`INSERT INTO resources (id, resource_type, data, synced_at) VALUES (?, 'project_task_eval', ?, ?)`,
		"run-1", string(blob), time.Now().UTC()); err != nil {
		t.Fatalf("inserting eval run: %v", err)
	}

	// A live context reads the row.
	runs, err := loadEvalRunsForTask(context.Background(), db, "task-1", "")
	if err != nil {
		t.Fatalf("loadEvalRunsForTask error = %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("got %d eval runs, want 1", len(runs))
	}

	// An expired one must abort the query instead of reading anyway.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := loadEvalRunsForTask(ctx, db, "task-1", ""); err == nil {
		t.Fatal("loadEvalRunsForTask with a cancelled context returned no error; the context is being ignored")
	}
}
