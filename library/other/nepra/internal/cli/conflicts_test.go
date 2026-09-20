// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
//
// Command-level tests for `conflicts`. The invariant every one of them
// protects is the same: both figures reach the caller with their provenance,
// nothing is reconciled, and no absent value ever reads as a zero.

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestNovelConflictsHelpWires smoke-tests that the conflicts command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelConflictsHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"conflicts", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("conflicts --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "conflicts"} {
		if !strings.Contains(help, want) {
			t.Fatalf("conflicts --help missing %q in output:\n%s", want, help)
		}
	}
	// Every flag the signature promises must exist, or a documented
	// invocation is a lie.
	for _, flag := range []string{"--surface", "--kind", "--fy", "--recompute", "--break-ratio",
		"--strict", "--limit"} {
		if !strings.Contains(help, flag) {
			t.Errorf("conflicts --help does not document %s", flag)
		}
	}
}

// runConflicts executes the command against a base URL that FAILS the test on
// any request, and returns stdout, stderr and the error.
//
// Pointing the client at a t.Fatal handler is the whole point: the default
// path is the shipped ledger and it must make no request at all. A command
// that quietly fetched something would still look right in its output.
func runConflicts(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("conflicts %v made an HTTP request to %s; the ledger path must make none", args, r.URL)
		w.WriteHeader(http.StatusTeapot)
	}))
	defer srv.Close()
	t.Setenv("NEPRA_BASE_URL", srv.URL)

	cmd := RootCmd()
	cmd.SetArgs(append([]string{"conflicts"}, args...))
	var so, se bytes.Buffer
	cmd.SetOut(&so)
	cmd.SetErr(&se)
	err = cmd.Execute()
	return so.String(), se.String(), err
}

// conflictsEnvelope is the documented wire shape, declared independently of
// the producing structs so a regression that marshalled nepraper.Conflict
// directly (Go field case, Key/A/B/Class) would fail to unmarshal here.
type conflictsEnvelope struct {
	Meta struct {
		Source                string   `json:"source"`
		LedgerAsOf            string   `json:"ledger_as_of"`
		BreakRatio            float64  `json:"break_ratio"`
		BreakRatioLedgerFloor float64  `json:"break_ratio_ledger_floor"`
		EntriesTotal          int      `json:"entries_total"`
		EntriesReturned       int      `json:"entries_returned"`
		Truncated             bool     `json:"truncated"`
		Recomputed            bool     `json:"recomputed"`
		SurfacesRequested     []string `json:"surfaces_requested"`
		KindsRequested        []string `json:"kinds_requested"`
		Reconciled            bool     `json:"reconciled"`
		BytesFetched          int      `json:"bytes_fetched"`
		Verified              int      `json:"entries_verified"`
		LiveOnly              int      `json:"entries_live_only"`
	} `json:"meta"`
	Results struct {
		Entries []struct {
			ID      string `json:"id"`
			Surface string `json:"surface"`
			Kind    string `json:"kind"`
			Class   string `json:"class"`
			SameKey bool   `json:"same_key"`
			Key     struct {
				PeriodFY string `json:"period_fy"`
				Entity   string `json:"entity"`
				Metric   string `json:"metric"`
			} `json:"key"`
			A                     map[string]any   `json:"a"`
			B                     map[string]any   `json:"b"`
			Also                  []map[string]any `json:"also"`
			Ratio                 *float64         `json:"ratio"`
			AbsDiff               *float64         `json:"abs_diff"`
			Direction             string           `json:"direction"`
			Reconciled            bool             `json:"reconciled"`
			Computable            string           `json:"computable"`
			Derivation            string           `json:"derivation"`
			ReproducedByThisBuild bool             `json:"reproduced_by_this_build"`
			MeasuredFrom          string           `json:"measured_from"`
			RecomputeNeeds        []string         `json:"recompute_needs"`
			Population            map[string]any   `json:"population"`
			ExplainedBy           map[string]any   `json:"explained_by"`
			BreakRatioSeen        *float64         `json:"break_ratio_seen"`
			Registry              *struct {
				Ratio       float64 `json:"ratio"`
				Direction   string  `json:"direction"`
				Description string  `json:"description"`
			} `json:"registry"`
			Note string `json:"note"`
		} `json:"entries"`
		Unverified       []map[string]any `json:"unverified"`
		UnverifiedGapFYs []string         `json:"unverified_gap_fys"`
		Refusals         []struct {
			Surface string `json:"surface"`
			Kind    string `json:"kind"`
			Subject string `json:"subject"`
			Verdict string `json:"verdict"`
			Reason  string `json:"reason"`
			Instead string `json:"instead"`
		} `json:"refusals"`
		DeclaredIncomplete []map[string]any `json:"declared_incomplete"`
		Recompute          []map[string]any `json:"recompute"`
	} `json:"results"`
}

func decodeConflicts(t *testing.T, stdout string) conflictsEnvelope {
	t.Helper()
	var env conflictsEnvelope
	dec := json.NewDecoder(strings.NewReader(stdout))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&env); err != nil {
		t.Fatalf("conflicts --json output does not match the documented shape: %v\n%s", err, stdout)
	}
	return env
}

// TestConflictsLedgerMakesNoRequest is the contract that makes this command
// useful offline: with no selector it prints the whole ledger and touches no
// network.
func TestConflictsLedgerMakesNoRequest(t *testing.T) {
	stdout, stderr, err := runConflicts(t, "--json")
	if err != nil {
		t.Fatalf("conflicts --json: %v\n%s", err, stderr)
	}
	env := decodeConflicts(t, stdout)
	if env.Meta.Source != "ledger" {
		t.Errorf("meta.source = %q, want %q on the offline path", env.Meta.Source, "ledger")
	}
	if env.Meta.BytesFetched != 0 {
		t.Errorf("meta.bytes_fetched = %d, want 0 with no --recompute", env.Meta.BytesFetched)
	}
	if env.Meta.Recomputed {
		t.Error("meta.recomputed is true without --recompute")
	}
	if len(env.Results.Entries) == 0 {
		t.Fatal("the ledger printed no entries; it IS the product, not a fallback")
	}
}

// TestConflictsLedgerSizeIsPinned pins the shipped ledger's shape so a silent
// loss of entries is visible. The counts are MEASURED from this build.
func TestConflictsLedgerSizeIsPinned(t *testing.T) {
	stdout, _, err := runConflicts(t, "--json")
	if err != nil {
		t.Fatalf("conflicts --json: %v", err)
	}
	env := decodeConflicts(t, stdout)
	if got := env.Meta.EntriesTotal; got != 25 {
		t.Errorf("ledger entries = %d, want 25", got)
	}
	if got := env.Meta.Verified; got != 24 {
		t.Errorf("verified entries = %d, want 24", got)
	}
	if got := env.Meta.LiveOnly; got != 1 {
		t.Errorf("live_only entries = %d, want 1 (the IESCO recovery revision)", got)
	}
	bySurface := map[string]int{}
	byKind := map[string]int{}
	for _, e := range env.Results.Entries {
		bySurface[e.Surface]++
		byKind[e.Kind]++
	}
	for surface, want := range map[string]int{"per": 11, "gen": 13, "capacity": 1} {
		if bySurface[surface] != want {
			t.Errorf("%s entries = %d, want %d", surface, bySurface[surface], want)
		}
	}
	for kind, want := range map[string]int{"conflict": 9, "break": 3, "arithmetic": 13} {
		if byKind[kind] != want {
			t.Errorf("%s entries = %d, want %d at the default break ratio", kind, byKind[kind], want)
		}
	}
}

// TestConflictsNeverReconciles is the command's central property. A regression
// that added a resolved value would leave every other test green.
func TestConflictsNeverReconciles(t *testing.T) {
	stdout, _, err := runConflicts(t, "--json")
	if err != nil {
		t.Fatalf("conflicts --json: %v", err)
	}
	// No field anywhere in the document may offer a chosen number.
	for _, banned := range []string{`"resolved`, `"preferred`, `"consensus`, `"best`, `"mean`,
		`"latest`, `"winner`, `"chosen`} {
		if strings.Contains(stdout, banned) {
			t.Errorf("the output carries a %s field; this command must never emit a reconciled value", banned)
		}
	}
	env := decodeConflicts(t, stdout)
	if env.Meta.Reconciled {
		t.Error("meta.reconciled is true")
	}
	for _, e := range env.Results.Entries {
		if e.Reconciled {
			t.Errorf("%s: reconciled is true", e.ID)
		}
		a, aok := e.A["value"].(float64)
		b, bok := e.B["value"].(float64)
		if !aok || !bok {
			continue
		}
		if a == b {
			t.Errorf("%s: both sides are %v; the conflict was collapsed", e.ID, a)
		}
		mid := (a + b) / 2
		if a == mid || b == mid {
			t.Errorf("%s: a side equals the mean of the two figures; it was averaged", e.ID)
		}
	}
}

// TestConflictsAbsentValueNeverReadsAsZero is the null-discipline assertion at
// the JSON boundary. A side that is not a number must carry NO `value` key,
// because a `"value": 0` on an unavailable figure is indistinguishable from a
// measured zero.
func TestConflictsAbsentValueNeverReadsAsZero(t *testing.T) {
	stdout, _, err := runConflicts(t, "--json")
	if err != nil {
		t.Fatalf("conflicts --json: %v", err)
	}
	env := decodeConflicts(t, stdout)
	checked := 0
	for _, e := range env.Results.Entries {
		sides := append([]map[string]any{e.A, e.B}, e.Also...)
		for _, s := range sides {
			kind, _ := s["value_kind"].(string)
			_, hasValue := s["value"]
			switch kind {
			case "numeric", "derived":
				if !hasValue {
					t.Errorf("%s: a %s side carries no value", e.ID, kind)
				}
			default:
				checked++
				if hasValue {
					t.Errorf("%s: a %q side carries value %v; an absent figure must have no value key",
						e.ID, kind, s["value"])
				}
				if basis, _ := s["basis"].(string); strings.TrimSpace(basis) == "" {
					t.Errorf("%s: a %q side carries no basis saying why there is no number", e.ID, kind)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no non-numeric side was exercised; the assertion proved nothing")
	}
}

// TestConflictsRatioSerialisesAsNullNotInfinity is the CRITICAL finding as a
// test. A zero denominator in a plain float64 produced math.Inf, Go's encoder
// refuses non-finite floats, and the ubiquitous `b, _ := json.Marshal(...)`
// then wrote an EMPTY document: a real conflict reported as no conflict.
func TestConflictsRatioSerialisesAsNullNotInfinity(t *testing.T) {
	// A published zero denominator is ordinary: safety_fatalities,
	// load_shedding and pending_connections are integer counts.
	zero := fptr(0)
	three := fptr(3)
	if r := conflictRatio(three, zero); r != nil {
		t.Errorf("conflictRatio(3, 0) = %v, want nil: the ratio does not EXIST", *r)
	}
	if r := conflictRatio(nil, three); r != nil {
		t.Errorf("conflictRatio(nil, 3) = %v, want nil", *r)
	}
	if d := conflictAbsDiff(three, zero); d == nil || *d != 3 {
		t.Errorf("conflictAbsDiff(3, 0) = %v, want 3: the magnitude still exists", d)
	}
	if d := conflictAbsDiff(three, nil); d != nil {
		t.Errorf("conflictAbsDiff(3, nil) = %v, want nil", *d)
	}

	entry := conflictEntry{
		ID: "test-zero-denominator", Surface: conflictSurfacePER, Kind: conflictKindConflict,
		Key: conflictKey{PeriodFY: "FY2018-19", Entity: "PESCO", Metric: "safety_fatalities"},
		A:   conflictSide{Raw: "3", Value: three, ValueKind: "numeric"},
		B:   conflictSide{Raw: "0", Value: zero, ValueKind: "numeric"},
	}
	entry.Ratio = conflictRatio(entry.A.Value, entry.B.Value)
	entry.AbsDiff = conflictAbsDiff(entry.A.Value, entry.B.Value)
	// The error MUST be returned, not swallowed into an empty document.
	out, err := json.Marshal([]conflictEntry{entry})
	if err != nil {
		t.Fatalf("marshalling a conflict with a zero denominator failed: %v", err)
	}
	if !strings.Contains(string(out), `"ratio":null`) {
		t.Errorf("an undefined ratio did not serialise as null:\n%s", out)
	}
	if !strings.Contains(string(out), `"abs_diff":3`) {
		t.Errorf("the magnitude was lost along with the ratio:\n%s", out)
	}

	// The shipped ledger must marshal too, and every ratio it does emit must
	// be finite.
	payload, err := json.Marshal(conflictLedger())
	if err != nil {
		t.Fatalf("the shipped ledger does not marshal: %v", err)
	}
	if len(payload) < 100 {
		t.Fatalf("the ledger marshalled to %d bytes; an empty document is how this defect presents",
			len(payload))
	}
	for _, e := range conflictLedger() {
		if e.Ratio != nil && (math.IsInf(*e.Ratio, 0) || math.IsNaN(*e.Ratio)) {
			t.Errorf("%s: ratio is %v, which json.Marshal refuses", e.ID, *e.Ratio)
		}
	}
}

// TestConflictsRefusalMatrix table-drives all nine (surface, kind) pairs.
//
// An unsupported pair must exit 2 with its measured reason and a working
// alternative. Returning an empty list would let an agent read a refusal as a
// finding of zero, which is the worst outcome available to this command.
func TestConflictsRefusalMatrix(t *testing.T) {
	want := map[string]bool{ // true == supported
		"per+conflict": true, "per+break": true, "per+arithmetic": false,
		"gen+conflict": false, "gen+break": false, "gen+arithmetic": true,
		"capacity+conflict": true, "capacity+break": false, "capacity+arithmetic": false,
	}
	supported := 0
	for _, surface := range conflictSurfaces {
		for _, kind := range conflictKinds {
			pair := surface + "+" + kind
			expectOK, known := want[pair]
			if !known {
				t.Fatalf("the matrix does not cover %s", pair)
			}
			stdout, stderr, err := runConflicts(t, "--surface", surface, "--kind", kind, "--json")
			if expectOK {
				supported++
				if err != nil {
					t.Errorf("%s: unexpected error %v\n%s", pair, err, stderr)
					continue
				}
				env := decodeConflicts(t, stdout)
				if len(env.Results.Entries) == 0 {
					t.Errorf("%s is supported but returned no entries", pair)
				}
				continue
			}
			if err == nil {
				t.Errorf("%s must be refused, but it succeeded with:\n%s", pair, stdout)
				continue
			}
			var typed *cliError
			if !errors.As(err, &typed) || typed.code != 2 {
				t.Errorf("%s: exit code = %v, want 2", pair, err)
			}
			msg := err.Error()
			if !strings.Contains(msg, "cannot be produced") {
				t.Errorf("%s: refusal does not say the pair cannot be produced: %s", pair, msg)
			}
			if !strings.Contains(msg, "instead:") {
				t.Errorf("%s: refusal names no working alternative: %s", pair, msg)
			}
			if len(msg) < 200 {
				t.Errorf("%s: refusal reason is %d chars; it must carry the MEASURED reason: %s",
					pair, len(msg), msg)
			}
		}
	}
	if supported != 4 {
		t.Errorf("supported pairs = %d, want 4", supported)
	}
	if len(conflictRefusals) != 5 {
		t.Errorf("the refusal table holds %d pairs, want 5 of the 9", len(conflictRefusals))
	}
	// The two refusals whose exact reason is load-bearing.
	perArith, _ := conflictSupported(conflictSurfacePER, conflictKindArithmetic)
	if perArith {
		t.Error("per+arithmetic must be refused: no shipped code checks a PER figure against its arithmetic")
	}
	for _, tc := range []struct{ surface, kind, phrase string }{
		{conflictSurfacePER, conflictKindArithmetic, "no shipped code checks a PER figure against its own arithmetic"},
		{conflictSurfaceGen, conflictKindConflict, "one vintage of each fiscal year"},
		{conflictSurfaceGen, conflictKindBreak, "one workbook covers one fiscal year"},
	} {
		_, refusal := conflictSupported(tc.surface, tc.kind)
		if !strings.Contains(refusal.Reason, tc.phrase) {
			t.Errorf("%s+%s reason does not carry %q: %s", tc.surface, tc.kind, tc.phrase, refusal.Reason)
		}
	}
}

// TestConflictsUsageContracts pins every exit-2 path. A validation that
// silently ignored a bad flag would answer a question the caller did not ask.
func TestConflictsUsageContracts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		args   []string
		phrase string
	}{
		{"unknown surface", []string{"--surface", "bogus"}, "accepted values are per, gen, capacity"},
		{"unknown kind", []string{"--kind", "bogus"}, "conflict, break, arithmetic"},
		{"break ratio of one", []string{"--break-ratio", "1"}, "would flag every ordinary year-on-year change"},
		{"break ratio below the ledger floor", []string{"--break-ratio", "3"}, "below the shipped ledger's measured floor"},
		{"negative limit", []string{"--limit", "-1"}, "must be zero or positive"},
		{"strict without recompute", []string{"--strict"}, "--strict asserts a recomputation"},
		{"unparseable fiscal year", []string{"--fy", "nope"}, "is not a fiscal year"},
		// The FY2023-24 PER is linked from NEPRA's own index and 404s. The
		// honest answer is the availability reason, never an empty result set.
		{"unavailable per year", []string{"--fy", "2023-24", "--surface", "per"}, "returns HTTP 404"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := runConflicts(t, tc.args...)
			if err == nil {
				t.Fatalf("conflicts %v succeeded; want exit 2", tc.args)
			}
			var typed *cliError
			if !errors.As(err, &typed) || typed.code != 2 {
				t.Fatalf("conflicts %v: want a usage error (exit 2), got %v", tc.args, err)
			}
			if !strings.Contains(err.Error(), tc.phrase) {
				t.Errorf("conflicts %v: message %q does not carry %q", tc.args, err.Error(), tc.phrase)
			}
		})
	}
}

// TestConflictsDryRunEnvelope keeps --dry-run from leaving a --json caller with
// empty stdout, which looks like a broken command rather than a deliberate
// no-op.
func TestConflictsDryRunEnvelope(t *testing.T) {
	for _, args := range [][]string{{"--dry-run", "--json"}, {"--surface", "per", "--dry-run", "--json"}} {
		stdout, _, err := runConflicts(t, args...)
		if err != nil {
			t.Fatalf("conflicts %v: %v", args, err)
		}
		var got dryRunResult
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("conflicts %v did not print a dry_run envelope: %v\n%s", args, err, stdout)
		}
		if !got.DryRun || got.Action != "conflicts" {
			t.Errorf("conflicts %v dry-run envelope = %+v", args, got)
		}
	}
}

// TestConflictsBreakThresholdIsDeclaredAndApplied is the assertion that keeps
// a break count from being quotable without its threshold.
//
// MEASURED from the committed FY2024-25 capture: 3 steps at or above 100x, 4
// at 50x, 5 at the ledger's declared floor of 5x.
func TestConflictsBreakThresholdIsDeclaredAndApplied(t *testing.T) {
	for _, tc := range []struct {
		ratio string
		want  int
	}{{"100", 3}, {"50", 4}, {"5", 5}} {
		stdout, _, err := runConflicts(t, "--surface", "per", "--kind", "break",
			"--break-ratio", tc.ratio, "--json")
		if err != nil {
			t.Fatalf("--break-ratio %s: %v", tc.ratio, err)
		}
		env := decodeConflicts(t, stdout)
		if len(env.Results.Entries) != tc.want {
			var ids []string
			for _, e := range env.Results.Entries {
				ids = append(ids, e.ID)
			}
			t.Errorf("--break-ratio %s returned %d breaks, want %d: %v",
				tc.ratio, len(env.Results.Entries), tc.want, ids)
		}
		// The threshold must be in the document, and so must the floor.
		if env.Meta.BreakRatioLedgerFloor != conflictsLedgerBreakFloor {
			t.Errorf("meta.break_ratio_ledger_floor = %v, want %v",
				env.Meta.BreakRatioLedgerFloor, conflictsLedgerBreakFloor)
		}
		for _, e := range env.Results.Entries {
			if e.SameKey {
				t.Errorf("%s: a break claims same_key; it is a step between two adjacent PERIODS", e.ID)
			}
			if e.BreakRatioSeen == nil {
				t.Errorf("%s: a break carries no measured step, so its threshold cannot be re-applied", e.ID)
			} else if *e.BreakRatioSeen < env.Meta.BreakRatio {
				t.Errorf("%s: step %v is below the declared threshold %v",
					e.ID, *e.BreakRatioSeen, env.Meta.BreakRatio)
			}
		}
	}
	// The exact three at the default, by ratio.
	stdout, _, err := runConflicts(t, "--surface", "per", "--kind", "break", "--json")
	if err != nil {
		t.Fatal(err)
	}
	env := decodeConflicts(t, stdout)
	want := map[string]float64{
		"per-break-fy2021-22-iesco-saidi-from-fy2020-21": 755.1544117647,
		"per-break-fy2021-22-iesco-saifi-from-fy2020-21": 411.2,
		"per-break-fy2023-24-gepco-saidi-from-fy2022-23": 109.2656128531,
	}
	for _, e := range env.Results.Entries {
		wantRatio, ok := want[e.ID]
		if !ok {
			t.Errorf("unexpected break %s at the default threshold", e.ID)
			continue
		}
		if math.Abs(*e.BreakRatioSeen-wantRatio) > 1e-6 {
			t.Errorf("%s step = %.10f, want %.10f", e.ID, *e.BreakRatioSeen, wantRatio)
		}
		delete(want, e.ID)
	}
	for id := range want {
		t.Errorf("break %s was not reported at the default threshold", id)
	}
}

// TestConflictsExplainedByNamesTheReportThatCarriesTheWording keeps an
// explanation from being attributed to the report being read.
func TestConflictsExplainedByNamesTheReportThatCarriesTheWording(t *testing.T) {
	var checked int
	for _, e := range conflictLedger() {
		if e.ExplainedBy == nil {
			continue
		}
		checked++
		if e.ExplainedBy.ReportFY == e.B.ReportFY {
			t.Errorf("%s: explained_by names %s, the same report the step was READ from; NEPRA's wording "+
				"for the IESCO revision is in FY2021-22, not in the report that reprints the step",
				e.ID, e.ExplainedBy.ReportFY)
		}
		if e.ExplainedBy.ReadByThisBuild {
			t.Errorf("%s: explained_by claims this build read the quote, but no capture of %s exists",
				e.ID, e.ExplainedBy.ReportFY)
		}
		if e.ExplainedBy.Quote == "" {
			t.Errorf("%s: explained_by carries no wording", e.ID)
		}
	}
	if checked != 2 {
		t.Errorf("entries carrying an explanation = %d, want 2 (the IESCO SAIDI and SAIFI halves)", checked)
	}
}

// TestConflictsUnverifiedFiguresSurviveToTheLedger keeps nepra_scope.go's
// promise: it tells callers that `conflicts` is where the recorded unverified
// figures live.
func TestConflictsUnverifiedFiguresSurviveToTheLedger(t *testing.T) {
	stdout, _, err := runConflicts(t, "--json")
	if err != nil {
		t.Fatalf("conflicts --json: %v", err)
	}
	env := decodeConflicts(t, stdout)
	if len(env.Results.Unverified) != 3 {
		t.Fatalf("unverified figures = %d, want 3", len(env.Results.Unverified))
	}
	wantLabels := map[string]bool{"19,535.": false, "28,189.": false, "15,896.": false}
	for _, u := range env.Results.Unverified {
		label, _ := u["raw_label"].(string)
		if _, ok := wantLabels[label]; !ok {
			t.Errorf("unexpected unverified label %q", label)
			continue
		}
		wantLabels[label] = true
		if kind, _ := u["value_kind"].(string); kind != "unverified" {
			t.Errorf("%q: value_kind = %q, want unverified", label, kind)
		}
		// NO numeric field anywhere in the object. Parsing "19,535." as 19535
		// or 19.535 would be fabrication: the trailing digits are absent from
		// the text layer.
		for k, v := range u {
			switch v.(type) {
			case float64:
				t.Errorf("%q: field %q carries the number %v; a truncated chart label must never become one",
					label, k, v)
			}
		}
		if attributed, _ := u["entity_attributed"].(bool); attributed {
			t.Errorf("%q: entity_attributed is true, but these are system-wide chart labels", label)
		}
		if reason, _ := u["reason"].(string); !strings.Contains(reason, "truncated") {
			t.Errorf("%q: reason does not say the label is truncated: %q", label, reason)
		}
	}
	for label, seen := range wantLabels {
		if !seen {
			t.Errorf("unverified label %q was not surfaced", label)
		}
	}
	// The year believed truncated whose label was never captured is a KNOWN
	// GAP, never interpolated.
	if len(env.Results.UnverifiedGapFYs) != 1 || env.Results.UnverifiedGapFYs[0] != "FY2013-14" {
		t.Errorf("unverified_gap_fys = %v, want [FY2013-14]", env.Results.UnverifiedGapFYs)
	}
}

// TestConflictsProvenanceOnEverySide asserts every figure can be traced back
// to the bytes it came from.
func TestConflictsProvenanceOnEverySide(t *testing.T) {
	stdout, _, err := runConflicts(t, "--json")
	if err != nil {
		t.Fatalf("conflicts --json: %v", err)
	}
	if strings.Contains(stdout, "http_status") {
		t.Error("the output carries http_status; the client returns the body without the status code, so " +
			"recording one would assert a value never observed")
	}
	env := decodeConflicts(t, stdout)
	sawTrailingSpaceURL := false
	for _, e := range env.Results.Entries {
		if e.Direction == "" {
			t.Errorf("%s: no direction; a conflict with no direction claim must say so as \"unknowable\"", e.ID)
		}
		if e.Computable == "" || e.Derivation == "" {
			t.Errorf("%s: computable=%q derivation=%q; a reader must be able to tell a curated claim from "+
				"a computed one", e.ID, e.Computable, e.Derivation)
		}
		if e.Note == "" {
			t.Errorf("%s: no note", e.ID)
		}
		for i, s := range append([]map[string]any{e.A, e.B}, e.Also...) {
			kind, _ := s["value_kind"].(string)
			url, _ := s["url"].(string)
			// An `unavailable` side has no document to cite, which is the
			// point of the verdict.
			if url == "" && kind != "unavailable" {
				t.Errorf("%s side %d (%s): no url", e.ID, i, kind)
			}
			if strings.HasSuffix(url, "Companies%20.pdf") {
				sawTrailingSpaceURL = true
			}
		}
	}
	// The FY2020-21 PER path ends in a trailing ENCODED SPACE before .pdf and
	// the same URL without it returns 404, so a "cleaned" URL is a dead one.
	if !sawTrailingSpaceURL {
		t.Error("no entry cites the FY2020-21 PER; its trailing %20 is load-bearing and must be exercised")
	}
}

// TestConflictsLimitTruncationIsDeclared keeps a limited read from being
// mistaken for a complete ledger.
func TestConflictsLimitTruncationIsDeclared(t *testing.T) {
	full, _, err := runConflicts(t, "--json")
	if err != nil {
		t.Fatal(err)
	}
	total := decodeConflicts(t, full).Meta.EntriesTotal

	stdout, _, err := runConflicts(t, "--limit", "3", "--json")
	if err != nil {
		t.Fatal(err)
	}
	env := decodeConflicts(t, stdout)
	if len(env.Results.Entries) != 3 {
		t.Errorf("entries returned = %d, want 3", len(env.Results.Entries))
	}
	if !env.Meta.Truncated {
		t.Error("meta.truncated is false on a truncated read")
	}
	if env.Meta.EntriesTotal != total {
		t.Errorf("meta.entries_total = %d under --limit, want the pre-limit %d", env.Meta.EntriesTotal, total)
	}

	// A limit at or above the total is not a truncation.
	stdout, _, err = runConflicts(t, "--limit", "500", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if decodeConflicts(t, stdout).Meta.Truncated {
		t.Error("meta.truncated is true when --limit exceeded the entry count")
	}
}

// TestConflictsSortIsDeterministic keeps two runs from disagreeing on order,
// which would make a diff of two ledgers unreadable.
func TestConflictsSortIsDeterministic(t *testing.T) {
	first, _, err := runConflicts(t, "--json")
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := runConflicts(t, "--json")
	if err != nil {
		t.Fatal(err)
	}
	a := decodeConflicts(t, first).Results.Entries
	b := decodeConflicts(t, second).Results.Entries
	if len(a) != len(b) {
		t.Fatalf("two runs returned %d and %d entries", len(a), len(b))
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			t.Fatalf("entry %d differs between runs: %s vs %s", i, a[i].ID, b[i].ID)
		}
	}
}

// TestConflictsRefusalsAlwaysAccompanyResults keeps the declared scope in the
// document, so an agent reading only stdout still learns what it cannot ask.
func TestConflictsRefusalsAlwaysAccompanyResults(t *testing.T) {
	stdout, _, err := runConflicts(t, "--json")
	if err != nil {
		t.Fatal(err)
	}
	env := decodeConflicts(t, stdout)
	if len(env.Results.Refusals) != len(conflictRefusals)+len(conflictScopeRefusals) {
		t.Errorf("refusals = %d, want %d", len(env.Results.Refusals),
			len(conflictRefusals)+len(conflictScopeRefusals))
	}
	for _, r := range env.Results.Refusals {
		if r.Verdict == "" || r.Reason == "" {
			t.Errorf("refusal %+v carries no verdict or reason", r)
		}
	}
	if len(env.Results.DeclaredIncomplete) == 0 {
		t.Error("the ledger declares no incompleteness; it is a floor, not a complete set, and must say so")
	}
	// The chart-label conflicts are the richest in the corpus and are NOT
	// computable. They belong in refusals, never in entries.
	var sawChartLabels bool
	for _, r := range env.Results.Refusals {
		if strings.Contains(r.Subject, "chart-label") {
			sawChartLabels = true
			if r.Verdict != conflictNotComputable {
				t.Errorf("the chart-label refusal is verdict %q, want %q", r.Verdict, conflictNotComputable)
			}
		}
	}
	if !sawChartLabels {
		t.Error("the FY2024-25 chart-label disagreements are not declared; they would otherwise look absent")
	}
	for _, e := range env.Results.Entries {
		if strings.Contains(e.Note, "15.31") || strings.Contains(e.Note, "49.41") {
			t.Errorf("%s quotes a chart-label figure this build cannot read", e.ID)
		}
	}
}

// TestConflictsHumanSummaryNeverPicksASide checks the terminal view, which is
// the surface a human actually reads.
func TestConflictsHumanSummaryNeverPicksASide(t *testing.T) {
	// --human-friendly forces the terminal view even when stdout is piped,
	// which is what a test harness always is.
	_, stderr, err := runConflicts(t, "--human-friendly")
	if err != nil {
		t.Fatalf("conflicts: %v", err)
	}
	for _, want := range []string{"Nothing here is reconciled", "3547 [Table 6 p15] vs 1182.56 [Table 18 p29]",
		"pairs that work"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("the human summary does not carry %q:\n%s", want, stderr)
		}
	}
}
