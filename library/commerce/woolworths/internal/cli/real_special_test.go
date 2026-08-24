// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mvanhorn/printing-press-library/library/commerce/woolworths/internal/store"
)

// ppwTestDay is one day in seconds, used to lay out seeded histories.
const ppwTestDay = int64(86400)

// ppwBaseTime is a fixed epoch for seeded fixtures so no test depends on the
// wall clock. 2025-01-01T00:00:00Z.
const ppwBaseTime = int64(1735689600)

// ppwRunCLI executes one CLI invocation through the real root command with
// buffered stdout/stderr, exactly as a shell would. Learning journal capture is
// disabled so a test invocation never writes to the user's real data dir.
func ppwRunCLI(t *testing.T, args ...string) (stdout string, stderr string, err error) {
	t.Helper()
	t.Setenv("WOOLWORTHS_NO_LEARN", "1")
	root := RootCmd()
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs(args)
	err = root.Execute()
	return out.String(), errBuf.String(), err
}

// ppwSeedDB writes a fresh SQLite mirror containing the given observations and
// specials memberships, and returns its path.
func ppwSeedDB(t *testing.T, obs []store.PriceObservation, members []store.SpecialsMember) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mirror.db")
	ctx := context.Background()
	db, err := store.OpenWithContext(ctx, path)
	if err != nil {
		t.Fatalf("opening seed store: %v", err)
	}
	defer db.Close()
	if err := store.EnsureWoolworthsTables(ctx, db.DB()); err != nil {
		t.Fatalf("ensuring woolworths tables: %v", err)
	}
	if len(obs) > 0 {
		if err := store.RecordPriceObservations(ctx, db.DB(), obs); err != nil {
			t.Fatalf("seeding price observations: %v", err)
		}
	}
	if len(members) > 0 {
		if err := store.RecordSpecialsMembership(ctx, db.DB(), members); err != nil {
			t.Fatalf("seeding specials membership: %v", err)
		}
	}
	return path
}

// ppwObs builds one observation on a given day offset from ppwBaseTime.
func ppwObs(stockcode string, dayOffset int, price, wasPrice float64, onSpecial, halfPrice bool) store.PriceObservation {
	savings := wasPrice - price
	if savings < 0 {
		savings = 0
	}
	return store.PriceObservation{
		Stockcode:   stockcode,
		ObservedAt:  ppwBaseTime + int64(dayOffset)*ppwTestDay,
		Name:        "Arnott's Tim Tam Original Chocolate Biscuits",
		Brand:       "Arnott's",
		Price:       price,
		WasPrice:    wasPrice,
		Savings:     savings,
		IsOnSpecial: onSpecial,
		IsHalfPrice: halfPrice,
		PackageSize: "200g",
		IsAvailable: true,
	}
}

// ppwDecodeRows parses a --json payload that should be a JSON array. It fails
// the test loudly on `null`, on empty stdout, and on anything unparseable —
// the three failure modes the JSON-fidelity contract exists to prevent.
func ppwDecodeRows[T any](t *testing.T, stdout string) []T {
	t.Helper()
	trimmed := strings.TrimSpace(stdout)
	if trimmed == "" {
		t.Fatalf("--json produced empty stdout")
	}
	if trimmed == "null" {
		t.Fatalf("--json produced null instead of []")
	}
	var rows []T
	if err := json.Unmarshal([]byte(trimmed), &rows); err != nil {
		t.Fatalf("--json output is not a JSON array: %v\nraw output:\n%s", err, stdout)
	}
	if rows == nil {
		t.Fatalf("--json output decoded to a nil slice (was it `null`?):\n%s", stdout)
	}
	return rows
}

// TestNovelRealSpecialHelpWires smoke-tests that the real-special command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelRealSpecialHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"real-special", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("real-special --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "real-special"} {
		if !strings.Contains(help, want) {
			t.Fatalf("real-special --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestPriceHistoryCommandsAreReadOnly pins the MCP annotation on all three
// price-history commands: they only ever read the API and append to the local
// mirror, so an MCP host must be free to call them without confirmation.
func TestPriceHistoryCommandsAreReadOnly(t *testing.T) {
	for _, name := range []string{"real-special", "cycle", "specials-diff"} {
		found := false
		for _, c := range RootCmd().Commands() {
			if c.Name() != name {
				continue
			}
			found = true
			if got := c.Annotations["mcp:read-only"]; got != "true" {
				t.Fatalf("%s mcp:read-only = %q, want \"true\"", name, got)
			}
		}
		if !found {
			t.Fatalf("command %q not registered on the root command", name)
		}
	}
}

// TestRealSpecialAbsenceOfCorrectness is the load-bearing honesty case: with a
// single observation on file the command must say NO-HISTORY, not guess.
func TestRealSpecialAbsenceOfCorrectness(t *testing.T) {
	db := ppwSeedDB(t, []store.PriceObservation{
		ppwObs("111111", 0, 3.00, 6.00, true, true),
	}, nil)

	stdout, stderr, err := ppwRunCLI(t, "real-special", "111111", "--data-source", "local", "--db", db, "--json")
	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)
	if err != nil {
		t.Fatalf("real-special returned error: %v", err)
	}
	rows := ppwDecodeRows[realSpecialRow](t, stdout)
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 verdict row, got %d", len(rows))
	}
	if rows[0].Verdict != "NO-HISTORY" {
		t.Fatalf("verdict = %q, want exactly \"NO-HISTORY\"", rows[0].Verdict)
	}
	for _, confident := range []string{"GENUINE", "RECYCLED", "WAS-PRICE-INFLATED"} {
		if rows[0].Verdict == confident {
			t.Fatalf("one observation produced the confident verdict %q", confident)
		}
	}
	if rows[0].Observations != 0 {
		t.Fatalf("prior observation count = %d, want 0 (the judged observation is not its own evidence)", rows[0].Observations)
	}
	if rows[0].Reason == "" {
		t.Fatalf("NO-HISTORY verdict carried no reason")
	}
}

// TestRealSpecialGenuine seeds a history whose current price is the lowest ever
// recorded and well under the median.
func TestRealSpecialGenuine(t *testing.T) {
	obs := []store.PriceObservation{
		ppwObs("222222", 0, 5.00, 5.00, false, false),
		ppwObs("222222", 1, 5.50, 5.50, false, false),
		ppwObs("222222", 2, 6.00, 6.00, false, false),
		ppwObs("222222", 3, 5.50, 5.50, false, false),
		ppwObs("222222", 4, 6.00, 6.00, false, false),
		ppwObs("222222", 5, 5.75, 5.75, false, false),
		// The observation under judgement: a new all-time low.
		ppwObs("222222", 6, 3.50, 5.50, true, false),
	}
	db := ppwSeedDB(t, obs, nil)

	stdout, stderr, err := ppwRunCLI(t, "real-special", "222222", "--data-source", "local", "--db", db, "--json")
	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)
	if err != nil {
		t.Fatalf("real-special returned error: %v", err)
	}
	rows := ppwDecodeRows[realSpecialRow](t, stdout)
	if len(rows) != 1 {
		t.Fatalf("expected 1 verdict row, got %d", len(rows))
	}
	r := rows[0]
	if r.Verdict != "GENUINE" {
		t.Fatalf("verdict = %q (reason %q), want \"GENUINE\"", r.Verdict, r.Reason)
	}
	if r.Observations != 6 {
		t.Fatalf("prior observations = %d, want 6", r.Observations)
	}
	// Reported figures are rounded to cents, so the true median 5.625 is
	// reported as 5.63.
	if r.LocalMin != 5.00 || r.LocalMedian != 5.63 {
		t.Fatalf("local min/median = %v/%v, want 5/5.63", r.LocalMin, r.LocalMedian)
	}
	if r.CurrentPrice != 3.50 {
		t.Fatalf("current price = %v, want 3.5", r.CurrentPrice)
	}
	if r.DaysOfHistory != 6 {
		t.Fatalf("days of history = %v, want 6", r.DaysOfHistory)
	}
}

// TestRealSpecialExcludesZeroPricedObservations is the NULL-price case. A
// sighting recorded while the product was unavailable carries price = 0. Left
// in, it makes local_min $0.00 and drags the median down, after which every
// genuine half-price week on that product is reported as ORDINARY.
func TestRealSpecialExcludesZeroPricedObservations(t *testing.T) {
	obs := []store.PriceObservation{
		ppwObs("555555", 0, 5.00, 5.00, false, false),
		ppwObs("555555", 1, 5.50, 5.50, false, false),
		// Recorded while the product was unavailable: a NULL/zero price.
		ppwObs("555555", 2, 0, 0, false, false),
		ppwObs("555555", 3, 6.00, 6.00, false, false),
		ppwObs("555555", 4, 5.50, 5.50, false, false),
		ppwObs("555555", 5, 6.00, 6.00, false, false),
		ppwObs("555555", 6, 5.75, 5.75, false, false),
		// The observation under judgement: a genuine all-time low.
		ppwObs("555555", 7, 3.50, 5.50, true, false),
	}
	db := ppwSeedDB(t, obs, nil)

	stdout, stderr, err := ppwRunCLI(t, "real-special", "555555", "--data-source", "local", "--db", db, "--json")
	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)
	if err != nil {
		t.Fatalf("real-special returned error: %v", err)
	}
	rows := ppwDecodeRows[realSpecialRow](t, stdout)
	if len(rows) != 1 {
		t.Fatalf("expected 1 verdict row, got %d", len(rows))
	}
	r := rows[0]
	if r.LocalMin == 0 {
		t.Fatalf("local_min = %v; the zero-priced observation poisoned the minimum", r.LocalMin)
	}
	if r.LocalMin != 5.00 {
		t.Fatalf("local_min = %v, want 5 (the lowest genuine price on record)", r.LocalMin)
	}
	if r.LocalMedian != 5.63 {
		t.Fatalf("local_median = %v, want 5.63 over the six priced observations", r.LocalMedian)
	}
	if r.Observations != 6 {
		t.Fatalf("prior observations = %d, want 6; the zero-priced sighting is not evidence", r.Observations)
	}
	if r.Verdict != "GENUINE" {
		t.Fatalf("verdict = %q (reason %q), want \"GENUINE\": $3.50 is a real all-time low", r.Verdict, r.Reason)
	}
}

// TestPpwTruncateIsRuneSafe pins the truncation used by the real-special, cycle
// and specials-diff tables: slicing bytes cuts a multi-byte rune in half and
// emits invalid UTF-8.
func TestPpwTruncateIsRuneSafe(t *testing.T) {
	tests := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{"shorter than the cap is untouched", "Tim Tam", 20, "Tim Tam"},
		{"ascii is truncated", "abcdefghij", 5, "abcd..."},
		{"multibyte runes are not split", "ééééééééé", 5, "éééé..."},
		{"multibyte under the cap is untouched", "café", 10, "café"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ppwTruncate(tt.in, tt.n)
			if got != tt.want {
				t.Fatalf("ppwTruncate(%q, %d) = %q, want %q", tt.in, tt.n, got, tt.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("ppwTruncate(%q, %d) produced invalid UTF-8: %q", tt.in, tt.n, got)
			}
		})
	}
}

// TestRealSpecialWasPriceInflated seeds a product parked at $3.00 for 20
// observations whose tile now advertises "was $6.00, now $3.00".
func TestRealSpecialWasPriceInflated(t *testing.T) {
	obs := make([]store.PriceObservation, 0, 21)
	for d := 0; d < 20; d++ {
		obs = append(obs, ppwObs("333333", d, 3.00, 3.00, false, false))
	}
	obs = append(obs, ppwObs("333333", 20, 3.00, 6.00, true, true))
	db := ppwSeedDB(t, obs, nil)

	stdout, stderr, err := ppwRunCLI(t, "real-special", "333333", "--data-source", "local", "--db", db, "--json")
	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)
	if err != nil {
		t.Fatalf("real-special returned error: %v", err)
	}
	rows := ppwDecodeRows[realSpecialRow](t, stdout)
	if len(rows) != 1 {
		t.Fatalf("expected 1 verdict row, got %d", len(rows))
	}
	r := rows[0]
	if r.Verdict != "WAS-PRICE-INFLATED" {
		t.Fatalf("verdict = %q (reason %q), want \"WAS-PRICE-INFLATED\"", r.Verdict, r.Reason)
	}
	if r.WasPrice != 6.00 || r.CurrentPrice != 3.00 {
		t.Fatalf("prices = now %v / was %v, want 3 / 6", r.CurrentPrice, r.WasPrice)
	}
	if r.LocalMedian != 3.00 {
		t.Fatalf("local median = %v, want 3", r.LocalMedian)
	}
	if r.Observations != 20 {
		t.Fatalf("prior observations = %d, want 20", r.Observations)
	}
}

// TestRealSpecialNonsenseTermReturnsNothing checks the negative path offline:
// a term that matches nothing must not borrow an unrelated product's history.
func TestRealSpecialNonsenseTermReturnsNothing(t *testing.T) {
	db := ppwSeedDB(t, []store.PriceObservation{
		ppwObs("222222", 0, 5.00, 5.00, false, false),
		ppwObs("222222", 1, 3.00, 6.00, true, true),
	}, nil)

	stdout, stderr, err := ppwRunCLI(t, "real-special", "zzqqxxnonsense", "--data-source", "local", "--db", db, "--json")
	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)
	if err != nil {
		t.Fatalf("real-special returned error: %v", err)
	}
	rows := ppwDecodeRows[realSpecialRow](t, stdout)
	if len(rows) != 0 {
		t.Fatalf("nonsense term produced %d verdict row(s): %+v", len(rows), rows)
	}
	if !strings.Contains(stderr, "nothing recorded locally") {
		t.Fatalf("expected an honest note on stderr, got:\n%s", stderr)
	}
}

// TestRealSpecialJSONFidelityOnMissingMirror covers the missing-mirror guard:
// [] on stdout, guidance on stderr, exit 0.
func TestRealSpecialJSONFidelityOnMissingMirror(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.db")
	stdout, stderr, err := ppwRunCLI(t, "real-special", "tim tam", "--data-source", "local", "--db", missing, "--json")
	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)
	if err != nil {
		t.Fatalf("missing mirror returned error: %v", err)
	}
	if _, statErr := os.Stat(missing); !os.IsNotExist(statErr) {
		t.Fatalf("the guard created the mirror it was supposed to refuse to open")
	}
	rows := ppwDecodeRows[realSpecialRow](t, stdout)
	if len(rows) != 0 {
		t.Fatalf("expected [], got %d rows", len(rows))
	}
	if !strings.Contains(stderr, "no local mirror at") {
		t.Fatalf("missing-mirror guidance absent from stderr:\n%s", stderr)
	}
}

func TestRealSpecialDryRun(t *testing.T) {
	stdout, stderr, err := ppwRunCLI(t, "real-special", "tim tam", "--dry-run", "--json")
	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)
	if err != nil {
		t.Fatalf("dry run returned error: %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &envelope); err != nil {
		t.Fatalf("dry-run envelope is not JSON: %v\nraw:\n%s", err, stdout)
	}
	if envelope["dry_run"] != true {
		t.Fatalf("dry-run envelope missing dry_run=true: %v", envelope)
	}
	if envelope["action"] != "real-special" {
		t.Fatalf("dry-run action = %v, want real-special", envelope["action"])
	}
}

func TestRealSpecialBareInvocationPrintsHelp(t *testing.T) {
	stdout, _, err := ppwRunCLI(t, "real-special")
	if err != nil {
		t.Fatalf("bare invocation returned error: %v", err)
	}
	if !strings.Contains(stdout, "Usage:") {
		t.Fatalf("bare invocation did not print help:\n%s", stdout)
	}
}

func TestRealSpecialMissingArgIsUsageError(t *testing.T) {
	db := ppwSeedDB(t, []store.PriceObservation{ppwObs("222222", 0, 5, 5, false, false)}, nil)
	_, _, err := ppwRunCLI(t, "real-special", "--db", db, "--data-source", "local", "--json")
	if err == nil {
		t.Fatalf("expected a usage error when the positional argument is omitted")
	}
	if code := ExitCode(err); code != 2 {
		t.Fatalf("exit code = %d, want 2 for a usage error (err: %v)", code, err)
	}
}
