// Copyright 2026 Farouk Umar and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/rightbrain/internal/store"
)

// rolloutRowByID finds one revision row in a report, failing the test when the
// revision was dropped from the output entirely.
func rolloutRowByID(t *testing.T, report rolloutReport, id string) rolloutRevision {
	t.Helper()
	for _, rev := range report.Revisions {
		if rev.RevisionID == id {
			return rev
		}
	}
	t.Fatalf("revision %q missing from report; got %+v", id, report.Revisions)
	return rolloutRevision{}
}

// rolloutPtr is the "this weight is known" constructor for the nullable weight
// map buildRolloutReport takes.
func rolloutPtr(v float64) *float64 { return &v }

// rolloutWeights lifts a plain id->weight map into the nullable form. Every
// weight built this way is a known one; an unknown weight is a nil entry.
func rolloutWeights(pairs map[string]float64) map[string]*float64 {
	out := make(map[string]*float64, len(pairs))
	for id, w := range pairs {
		out[id] = rolloutPtr(w)
	}
	return out
}

// rolloutShare dereferences a nullable share or drift, failing the test when
// the report left it unknown.
func rolloutShare(t *testing.T, label string, p *float64) float64 {
	t.Helper()
	if p == nil {
		t.Fatalf("%s = null, want a number", label)
	}
	return *p
}

// TestNovelRolloutHelpWires smoke-tests that the rollout command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review.
func TestNovelRolloutHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"rollout", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("rollout --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "rollout", "--since", "--db"} {
		if !strings.Contains(help, want) {
			t.Fatalf("rollout --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestRolloutObservedShare checks the core arithmetic: observed share is a
// revision's runs over the task's total runs in the window, and drift is the
// gap from the configured share in percentage points. Both percent-style
// (sum 100) and fraction-style (sum 1) configured weights must normalize to the
// same share, since the API uses both.
func TestRolloutObservedShare(t *testing.T) {
	runs := make([]rolloutRun, 0)
	for i := 0; i < 8; i++ {
		runs = append(runs, rolloutRun{RevisionID: "rev-stable"})
	}
	for i := 0; i < 2; i++ {
		runs = append(runs, rolloutRun{RevisionID: "rev-canary"})
	}

	cases := []struct {
		name       string
		configured map[string]*float64
	}{
		{"percent weights", rolloutWeights(map[string]float64{"rev-stable": 90, "rev-canary": 10})},
		{"fraction weights", rolloutWeights(map[string]float64{"rev-stable": 0.9, "rev-canary": 0.1})},
		{"unnormalized weights", rolloutWeights(map[string]float64{"rev-stable": 9, "rev-canary": 1})},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := buildRolloutReport(tc.configured, runs, "7d")

			if report.TotalRuns != 10 {
				t.Fatalf("total_runs = %d, want 10", report.TotalRuns)
			}
			if !report.ConfiguredFound {
				t.Fatalf("configured_found = false, want true")
			}
			if report.Window != "7d" {
				t.Fatalf("window = %q, want %q", report.Window, "7d")
			}
			if len(report.Revisions) != 2 {
				t.Fatalf("got %d revisions, want 2: %+v", len(report.Revisions), report.Revisions)
			}
			// Busiest revision first.
			if report.Revisions[0].RevisionID != "rev-stable" {
				t.Fatalf("revisions[0] = %q, want rev-stable (sorted by observed share desc)", report.Revisions[0].RevisionID)
			}

			stable := rolloutRowByID(t, report, "rev-stable")
			if stable.ObservedShare != 0.8 {
				t.Errorf("rev-stable observed_share = %v, want 0.8", stable.ObservedShare)
			}
			if got := rolloutShare(t, "rev-stable configured_share", stable.ConfiguredShare); got != 0.9 {
				t.Errorf("rev-stable configured_share = %v, want 0.9", got)
			}
			if got := rolloutShare(t, "rev-stable drift_pct", stable.DriftPct); got != -10 {
				t.Errorf("rev-stable drift_pct = %v, want -10", got)
			}
			if stable.Runs != 8 {
				t.Errorf("rev-stable runs = %d, want 8", stable.Runs)
			}

			canary := rolloutRowByID(t, report, "rev-canary")
			if canary.ObservedShare != 0.2 {
				t.Errorf("rev-canary observed_share = %v, want 0.2", canary.ObservedShare)
			}
			if got := rolloutShare(t, "rev-canary configured_share", canary.ConfiguredShare); got != 0.1 {
				t.Errorf("rev-canary configured_share = %v, want 0.1", got)
			}
			if got := rolloutShare(t, "rev-canary drift_pct", canary.DriftPct); got != 10 {
				t.Errorf("rev-canary drift_pct = %v, want +10", got)
			}
		})
	}
}

// TestRolloutPercentiles pins the latency math to known samples. p50/p95 use
// nearest-rank, so every reported value is a latency some run actually
// produced.
func TestRolloutPercentiles(t *testing.T) {
	t.Run("percentile helper", func(t *testing.T) {
		cases := []struct {
			name     string
			values   []float64
			p        float64
			expected float64
		}{
			{"ten samples p50", []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 0.50, 5},
			{"ten samples p95", []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 0.95, 10},
			{"unsorted input p50", []float64{9, 1, 7, 3, 5}, 0.50, 5},
			{"unsorted input p95", []float64{9, 1, 7, 3, 5}, 0.95, 9},
			{"single sample", []float64{4.25}, 0.95, 4.25},
			{"empty sample", []float64{}, 0.50, 0},
		}
		for _, tc := range cases {
			if got := rolloutPercentile(tc.values, tc.p); got != tc.expected {
				t.Errorf("%s: rolloutPercentile(%v, %v) = %v, want %v", tc.name, tc.values, tc.p, got, tc.expected)
			}
		}
	})

	t.Run("report latencies", func(t *testing.T) {
		runs := make([]rolloutRun, 0)
		for _, secs := range []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10} {
			runs = append(runs, rolloutRun{RevisionID: "rev-a", LatencySecs: secs, HasLatency: true})
		}
		// A run with no timing at all must not be folded in as a 0.0 sample,
		// which would drag p50 down and hide a slow canary.
		runs = append(runs, rolloutRun{RevisionID: "rev-a"})

		report := buildRolloutReport(nil, runs, "24h")
		row := rolloutRowByID(t, report, "rev-a")
		if row.P50LatencySecs != 5 {
			t.Errorf("p50_latency_secs = %v, want 5", row.P50LatencySecs)
		}
		if row.P95LatencySecs != 10 {
			t.Errorf("p95_latency_secs = %v, want 10", row.P95LatencySecs)
		}
		if row.Runs != 11 {
			t.Errorf("runs = %d, want 11 (the untimed run still counts as traffic)", row.Runs)
		}
		if row.LatencySamples != 10 {
			t.Errorf("latency_samples = %d, want 10 (the untimed run is not a 0.0 sample)", row.LatencySamples)
		}
		if report.ConfiguredFound {
			t.Errorf("configured_found = true, want false when no weights were supplied")
		}
		if report.Note == "" {
			t.Errorf("note is empty, want an explanation that active_revisions was missing")
		}
	})
}

// TestRolloutConfiguredButZeroRuns is the absence case this command exists for:
// a revision configured to take traffic that received none must still be a row.
// Dropping it would render the exact failure the command is meant to catch —
// "configured 20%, observed 0%" — indistinguishable from a healthy split.
func TestRolloutConfiguredButZeroRuns(t *testing.T) {
	configured := rolloutWeights(map[string]float64{"rev-stable": 80, "rev-canary": 20})
	runs := make([]rolloutRun, 0)
	for i := 0; i < 5; i++ {
		runs = append(runs, rolloutRun{RevisionID: "rev-stable"})
	}

	report := buildRolloutReport(configured, runs, "7d")

	if len(report.Revisions) != 2 {
		t.Fatalf("got %d revisions, want 2 (the idle canary must not be dropped): %+v",
			len(report.Revisions), report.Revisions)
	}
	canary := rolloutRowByID(t, report, "rev-canary")
	if canary.Runs != 0 {
		t.Errorf("rev-canary runs = %d, want 0", canary.Runs)
	}
	if canary.ObservedShare != 0 {
		t.Errorf("rev-canary observed_share = %v, want 0", canary.ObservedShare)
	}
	if got := rolloutShare(t, "rev-canary configured_share", canary.ConfiguredShare); got != 0.2 {
		t.Errorf("rev-canary configured_share = %v, want 0.2", got)
	}
	if got := rolloutShare(t, "rev-canary drift_pct", canary.DriftPct); got != -20 {
		t.Errorf("rev-canary drift_pct = %v, want -20 (configured 20%%, observed 0%%)", got)
	}
	if !canary.Configured {
		t.Errorf("rev-canary configured = false, want true")
	}
	if canary.FailureRate != 0 || canary.MeanCredits != 0 || canary.P95LatencySecs != 0 {
		t.Errorf("rev-canary should report zeroed stats with no runs, got %+v", canary)
	}
	// The sample counts are what tell a caller those zeros mean "no data"
	// rather than "free and instant".
	if canary.CreditSamples != 0 || canary.LatencySamples != 0 {
		t.Errorf("rev-canary sample counts = %d credits / %d latencies, want 0 / 0",
			canary.CreditSamples, canary.LatencySamples)
	}
	// The idle revision sorts last, but it is present.
	if report.Revisions[len(report.Revisions)-1].RevisionID != "rev-canary" {
		t.Errorf("revisions last = %q, want rev-canary", report.Revisions[len(report.Revisions)-1].RevisionID)
	}
}

// TestRolloutCreditsExcludeUnparseable is the second absence case:
// charged_credits arrives as a string and is sometimes empty or malformed.
// Those runs must be excluded from the mean, not counted as 0.0 — a zero would
// pull the average down and report a revision as cheaper than it is.
func TestRolloutCreditsExcludeUnparseable(t *testing.T) {
	cases := []struct {
		name        string
		credits     []string
		wantMean    float64
		wantSamples int
	}{
		{"empty string excluded", []string{"1.00", "3.00", ""}, 2.0, 2},
		{"malformed value excluded", []string{"1.00", "3.00", "n/a"}, 2.0, 2},
		{"whitespace excluded", []string{"1.00", "3.00", "   "}, 2.0, 2},
		{"all parseable", []string{"1.00", "3.00"}, 2.0, 2},
		{"explicit zero counts", []string{"1.00", "3.00", "0.00"}, 1.3333, 3},
		{"none parseable", []string{"", "bogus"}, 0, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runs := make([]rolloutRun, 0, len(tc.credits))
			for _, credit := range tc.credits {
				runs = append(runs, rolloutRun{RevisionID: "rev-a", ChargedCredits: credit})
			}
			report := buildRolloutReport(nil, runs, "7d")
			row := rolloutRowByID(t, report, "rev-a")

			if row.MeanCredits != tc.wantMean {
				t.Errorf("mean_credits = %v, want %v (unparseable values must be excluded, not treated as 0)",
					row.MeanCredits, tc.wantMean)
			}
			if row.CreditSamples != tc.wantSamples {
				t.Errorf("credit_samples = %d, want %d", row.CreditSamples, tc.wantSamples)
			}
			if row.Runs != len(tc.credits) {
				t.Errorf("runs = %d, want %d (excluded credits must not drop the run)", row.Runs, len(tc.credits))
			}
		})
	}
}

// TestRolloutFailureRateAndUnattributed covers the remaining per-row math and
// the fallback bucket for runs whose record carries no revision id — they are
// surfaced, not discarded, because a task whose traffic is mostly unattributed
// cannot have its split verified at all.
func TestRolloutFailureRateAndUnattributed(t *testing.T) {
	runs := []rolloutRun{
		{RevisionID: "rev-a", IsError: true},
		{RevisionID: "rev-a"},
		{RevisionID: "rev-a"},
		{RevisionID: "rev-a"},
		{RevisionID: ""},
	}

	report := buildRolloutReport(rolloutWeights(map[string]float64{"rev-a": 1}), runs, "7d")

	rowA := rolloutRowByID(t, report, "rev-a")
	if rowA.FailureRate != 0.25 {
		t.Errorf("rev-a failure_rate = %v, want 0.25", rowA.FailureRate)
	}
	if rowA.ObservedShare != 0.8 {
		t.Errorf("rev-a observed_share = %v, want 0.8", rowA.ObservedShare)
	}
	if got := rolloutShare(t, "rev-a configured_share", rowA.ConfiguredShare); got != 1 {
		t.Errorf("rev-a configured_share = %v, want 1 (a lone weight normalizes to the whole split)", got)
	}

	unattributed := rolloutRowByID(t, report, rolloutUnattributed)
	if unattributed.Runs != 1 {
		t.Errorf("unattributed runs = %d, want 1", unattributed.Runs)
	}
	if unattributed.Configured {
		t.Errorf("unattributed configured = true, want false")
	}
}

// rolloutTestDB builds a throwaway mirror holding the given run payloads and
// returns an open handle to it.
func rolloutTestDB(t *testing.T, runs []map[string]any) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("opening test store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for i, run := range runs {
		blob, err := json.Marshal(run)
		if err != nil {
			t.Fatalf("marshaling run %d: %v", i, err)
		}
		if _, err := db.DB().Exec(
			`INSERT INTO resources (id, resource_type, data, synced_at) VALUES (?, 'project_task_run', ?, ?)`,
			run["id"], string(blob), time.Now().UTC()); err != nil {
			t.Fatalf("inserting run %d: %v", i, err)
		}
	}
	return db
}

// TestRolloutQueryRuns exercises the mirror read against a real SQLite store.
// The fixtures deliberately omit fields (no revision id, no timing, no credits,
// no created stamp) so the NULL-safe scan is proven: a bare scan would error on
// those rows and the loop would drop them, which reads as "this revision got no
// traffic" — the exact wrong answer for a command about traffic.
func TestRolloutQueryRuns(t *testing.T) {
	now := time.Now().UTC()
	stamp := func(d time.Duration) string { return now.Add(-d).Format(time.RFC3339) }

	db := rolloutTestDB(t, []map[string]any{
		{"id": "r1", "task_id": "T", "task_revision_id": "rev-a", "created": stamp(time.Hour), "is_error": false, "charged_credits": "9.00", "llm_call_timing": 1.5},
		{"id": "r2", "task_id": "T", "task_revision_id": "rev-a", "created": stamp(2 * time.Hour), "is_error": true},
		{"id": "r3", "task_id": "T", "created": stamp(3 * time.Hour), "charged_credits": "11.00"},
		{"id": "r4", "task_id": "T", "task_revision_id": "rev-b", "created": stamp(40 * 24 * time.Hour)},
		{"id": "r5", "task_id": "T", "task_revision_id": "rev-b"},
		{"id": "r6", "task_id": "OTHER", "task_revision_id": "rev-z", "created": stamp(time.Hour)},
	})

	runs, undated, err := rolloutQueryRuns(context.Background(), db, "T", now.Add(-7*24*time.Hour))
	if err != nil {
		t.Fatalf("rolloutQueryRuns error = %v", err)
	}
	// r4 is outside the window and r6 belongs to another task; everything else
	// survives, including the rows missing most fields.
	if len(runs) != 4 {
		t.Fatalf("got %d runs, want 4: %+v", len(runs), runs)
	}
	if undated != 1 {
		t.Errorf("undated = %d, want 1 (the run with no created stamp)", undated)
	}

	byRev := map[string]int{}
	for _, run := range runs {
		byRev[run.RevisionID]++
	}
	if byRev["rev-a"] != 2 {
		t.Errorf("rev-a runs = %d, want 2", byRev["rev-a"])
	}
	if byRev["rev-b"] != 1 {
		t.Errorf("rev-b runs = %d, want 1 (the undated run is kept)", byRev["rev-b"])
	}
	if byRev[""] != 1 {
		t.Errorf("runs with no revision id = %d, want 1 (must not be dropped)", byRev[""])
	}

	for _, run := range runs {
		if run.RevisionID == "rev-b" && run.HasLatency {
			t.Errorf("missing llm_call_timing must not report as a latency sample: %+v", run)
		}
	}
	if runs[0].RevisionID != "rev-a" || !runs[0].HasLatency || runs[0].LatencySecs != 1.5 {
		t.Errorf("first run = %+v, want rev-a with 1.5s latency", runs[0])
	}
	if runs[1].ChargedCredits != "" {
		t.Errorf("run with no charged_credits = %q, want empty (excluded from the mean later)", runs[1].ChargedCredits)
	}
	if !runs[1].IsError {
		t.Errorf("run r2 is_error = false, want true")
	}

	report := buildRolloutReport(rolloutWeights(map[string]float64{"rev-a": 50, "rev-b": 50}), runs, "7d")
	rowA := rolloutRowByID(t, report, "rev-a")
	if rowA.MeanCredits != 9 || rowA.CreditSamples != 1 {
		t.Errorf("rev-a mean_credits = %v over %d samples, want 9 over 1", rowA.MeanCredits, rowA.CreditSamples)
	}
	if rowA.FailureRate != 0.5 {
		t.Errorf("rev-a failure_rate = %v, want 0.5", rowA.FailureRate)
	}
}

// TestRolloutConfiguredWeights covers the defensive decode of active_revisions.
// The field is absent on some task payloads and stringly-typed on others; both
// must degrade to a usable answer instead of failing the whole command.
func TestRolloutConfiguredWeights(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		// A nil entry means "the revision is present but its weight is
		// unknown" — the shape a null or unreadable weight must produce.
		want    map[string]*float64
		wantErr bool
	}{
		{
			name:    "percent weights",
			payload: `{"id":"T","active_revisions":[{"task_revision_id":"a","weight":80},{"task_revision_id":"b","weight":20}]}`,
			want:    map[string]*float64{"a": rolloutPtr(80), "b": rolloutPtr(20)},
		},
		{
			name:    "string weights still count",
			payload: `{"active_revisions":[{"task_revision_id":"a","weight":"0.9"},{"task_revision_id":"b","weight":"0.1"}]}`,
			want:    map[string]*float64{"a": rolloutPtr(0.9), "b": rolloutPtr(0.1)},
		},
		{
			name:    "field absent",
			payload: `{"id":"T","name":"summarize"}`,
			want:    map[string]*float64{},
		},
		{
			name:    "field null",
			payload: `{"active_revisions":null}`,
			want:    map[string]*float64{},
		},
		{
			name:    "empty array",
			payload: `{"active_revisions":[]}`,
			want:    map[string]*float64{},
		},
		{
			name:    "entry without an id is skipped",
			payload: `{"active_revisions":[{"weight":50},{"task_revision_id":"b","weight":50}]}`,
			want:    map[string]*float64{"b": rolloutPtr(50)},
		},
		{
			name:    "null weight is unknown, not zero",
			payload: `{"active_revisions":[{"task_revision_id":"a","weight":0.2},{"task_revision_id":"b","weight":null}]}`,
			want:    map[string]*float64{"a": rolloutPtr(0.2), "b": nil},
		},
		{
			name:    "unusable weight is unknown, not a dropped revision",
			payload: `{"active_revisions":[{"task_revision_id":"a","weight":"n/a"}]}`,
			want:    map[string]*float64{"a": nil},
		},
		{
			name:    "malformed payload",
			payload: `not json`,
			want:    map[string]*float64{},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := rolloutConfiguredWeights([]byte(tc.payload))
			if tc.wantErr != (err != nil) {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v keys, want %v", len(got), len(tc.want))
			}
			for id, want := range tc.want {
				gotWeight, present := got[id]
				if !present {
					t.Fatalf("weight[%q] missing; the revision must survive the decode", id)
				}
				switch {
				case want == nil && gotWeight != nil:
					t.Errorf("weight[%q] = %v, want unknown (nil): a weight that cannot be read is not 0", id, *gotWeight)
				case want != nil && gotWeight == nil:
					t.Errorf("weight[%q] = unknown, want %v", id, *want)
				case want != nil && *gotWeight != *want:
					t.Errorf("weight[%q] = %v, want %v", id, *gotWeight, *want)
				}
			}
		})
	}
}

// TestRolloutEmptyReportMarshalsAsList guards the empty-slice contract: a
// report with no revisions must emit [] rather than null, so agents can index
// the result without a nil check.
func TestRolloutEmptyReportMarshalsAsList(t *testing.T) {
	report := buildRolloutReport(nil, nil, "7d")
	if report.Revisions == nil {
		t.Fatalf("revisions is nil; want an initialized empty slice so JSON emits []")
	}
	if len(report.Revisions) != 0 {
		t.Fatalf("revisions = %+v, want empty", report.Revisions)
	}
	if report.TotalRuns != 0 {
		t.Fatalf("total_runs = %d, want 0", report.TotalRuns)
	}
	if !strings.Contains(report.Note, "no local runs") {
		t.Fatalf("note = %q, want an explanation that the mirror held no runs", report.Note)
	}
	if !strings.Contains(report.Note, "7d") {
		t.Fatalf("note = %q, want the window echoed back", report.Note)
	}
}

// TestRolloutNullWeightIsUnknownNotZero is the mixed-weight case: the task
// configures {A: 0.2, B: null} and traffic actually splits 20/80, exactly as
// intended. Scoring the unreadable weight as 0 would shrink the normalization
// denominator to 0.2, hand A a configured share of 100%, and print
// "CONFIG 100.0% / OBSERVED 20.0% / DRIFT -80.0pp" — the report screaming that
// the primary revision is starved while the split is correct.
func TestRolloutNullWeightIsUnknownNotZero(t *testing.T) {
	configured := map[string]*float64{"rev-a": rolloutPtr(0.2), "rev-b": nil}

	runs := make([]rolloutRun, 0)
	for i := 0; i < 2; i++ {
		runs = append(runs, rolloutRun{RevisionID: "rev-a"})
	}
	for i := 0; i < 8; i++ {
		runs = append(runs, rolloutRun{RevisionID: "rev-b"})
	}

	report := buildRolloutReport(configured, runs, "7d")

	rowA := rolloutRowByID(t, report, "rev-a")
	// The bug: rev-b's weight was scored as 0, the denominator shrank to 0.2,
	// and rev-a was reported at CONFIG 100.0% / DRIFT -80.0pp.
	if rowA.ConfiguredShare != nil {
		t.Errorf("rev-a configured_share = %v, want null: 0.2 out of an unknown total is not a share, and 1.0 here is the exact number that reads as a starved primary revision",
			*rowA.ConfiguredShare)
	}
	if rowA.DriftPct != nil {
		t.Errorf("rev-a drift_pct = %v, want null: with a sibling weight unreadable there is nothing to drift from", *rowA.DriftPct)
	}
	if rowA.ObservedShare != 0.2 {
		t.Errorf("rev-a observed_share = %v, want 0.2 — the observed half is still measurable", rowA.ObservedShare)
	}

	rowB := rolloutRowByID(t, report, "rev-b")
	if !rowB.Configured {
		t.Errorf("rev-b configured = false, want true: it is listed in active_revisions, only its weight is unknown")
	}
	if rowB.ConfiguredShare != nil {
		t.Errorf("rev-b configured_share = %v, want null: the weight could not be read, so its share is unknown", *rowB.ConfiguredShare)
	}
	if rowB.DriftPct != nil {
		t.Errorf("rev-b drift_pct = %v, want null: drift from an unknown configured share is not a number", *rowB.DriftPct)
	}
	if rowB.ObservedShare != 0.8 {
		t.Errorf("rev-b observed_share = %v, want 0.8 — the observed half is still measurable", rowB.ObservedShare)
	}

	// The JSON contract: null, not 0. An agent thresholding on drift_pct must
	// see "unknown" rather than a fabricated zero.
	blob, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	var decoded struct {
		Revisions []struct {
			RevisionID      string   `json:"revision_id"`
			ConfiguredShare *float64 `json:"configured_share"`
			DriftPct        *float64 `json:"drift_pct"`
		} `json:"revisions"`
	}
	if err := json.Unmarshal(blob, &decoded); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	seen := false
	for _, rev := range decoded.Revisions {
		if rev.RevisionID != "rev-b" {
			continue
		}
		seen = true
		if rev.ConfiguredShare != nil || rev.DriftPct != nil {
			t.Errorf("rev-b JSON = %+v, want configured_share and drift_pct both null", rev)
		}
	}
	if !seen {
		t.Fatalf("rev-b missing from marshalled report: %s", blob)
	}
	if !strings.Contains(string(blob), `"configured_share":null`) {
		t.Errorf("marshalled report has no null configured_share: %s", blob)
	}
	if !strings.Contains(report.Note, "null or unreadable weight") {
		t.Errorf("note = %q, want it to name the unreadable weight rather than leaving a column of nulls unexplained", report.Note)
	}

	// The one case where an unknown weight costs nothing: the readable weights
	// already add up to a whole split, so the unknown one can only be idle.
	whole := buildRolloutReport(
		map[string]*float64{"rev-a": rolloutPtr(50), "rev-b": rolloutPtr(50), "rev-c": nil},
		[]rolloutRun{{RevisionID: "rev-a"}, {RevisionID: "rev-b"}}, "7d")
	wholeA := rolloutRowByID(t, whole, "rev-a")
	if got := rolloutShare(t, "rev-a configured_share", wholeA.ConfiguredShare); got != 0.5 {
		t.Errorf("rev-a configured_share = %v, want 0.5: 50+50 is already the whole split", got)
	}
	if got := rolloutShare(t, "rev-a drift_pct", wholeA.DriftPct); got != 0 {
		t.Errorf("rev-a drift_pct = %v, want 0", got)
	}
	wholeC := rolloutRowByID(t, whole, "rev-c")
	if wholeC.ConfiguredShare != nil || wholeC.DriftPct != nil {
		t.Errorf("rev-c = %+v, want null configured_share and drift_pct", wholeC)
	}
}
