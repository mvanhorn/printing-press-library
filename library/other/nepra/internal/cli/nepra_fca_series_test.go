// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Tests for the `fca` series flags.
//
// These run offline against a fixture of the published sheet's own shape. The
// headline figures are the ones the absorb manifest says nobody had ever
// computed, so they are pinned to the digit.

package cli

import (
	"math"
	"strings"
	"testing"
)

// fcaTestSheet mirrors the extracted sheet's real column names and cell
// spellings, including the accounting negatives and the odd header spacing.
func fcaTestSheet() []map[string]any {
	row := func(year, month, cReq, cAlw, kReq, kAlw string) map[string]any {
		return map[string]any{
			"Year": year, "Month": month,
			"CPPA - G FCA Requested":     cReq,
			"CPPA - G FCA Allowed (kWh)": cAlw,
			"K-Electric FCA Requested":   kReq,
			"K-Electric FCA Allowed":     kAlw,
		}
	}
	return []map[string]any{
		row("2018", "Jul", "1.0000", "0.5000", "2.0000", "(2.5935)"),
		row("2018", "Aug", "2.0000", "2.0000", "(1.798)", "1.0000"),
		row("2020", "Jan", "1.4883", "1.1108", "0.657", "0.9426"),
		// An empty cell is ABSENCE, not zero.
		row("2020", "Feb", "3.0000", "", "", "1.0000"),
	}
}

// TestFCAParsesAccountingNegatives is the central trap. The sheet writes
// negatives as "(2.5935)" — 49 such cells across the 48 published rows — and
// strconv.ParseFloat REJECTS that string. Code that ignores the error reads a
// real negative adjustment as zero; code that strips the parens without
// restoring the sign reads it as positive. Both silently reverse the
// direction of a tariff adjustment.
func TestFCAParsesAccountingNegatives(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want float64
		ok   bool
	}{
		{"(2.5935)", -2.5935, true},
		{"(0.1930)", -0.1930, true},
		{"2.5935", 2.5935, true},
		{"1,234.5", 1234.5, true},
		{" 9.8972 ", 9.8972, true},
		{"", 0, false}, // absence, not zero
	} {
		got, ok, err := fcaParseFigure(tc.in)
		if err != nil {
			t.Fatalf("fcaParseFigure(%q): %v", tc.in, err)
		}
		if ok != tc.ok {
			t.Fatalf("fcaParseFigure(%q) ok = %v, want %v", tc.in, ok, tc.ok)
		}
		if ok && math.Abs(got-tc.want) > 1e-9 {
			t.Fatalf("fcaParseFigure(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
	// A cell that is neither must be an ERROR, never a silent zero.
	if _, _, err := fcaParseFigure("not a number"); err == nil {
		t.Fatal("an unparseable cell was accepted; it would have been read as 0.00")
	}
	if _, _, err := fcaParseFigure("(nope)"); err == nil {
		t.Fatal("an unparseable parenthesised cell was accepted")
	}
}

// TestFCAEmptyCellIsAbsentNotZero pins the distinction across the row
// builder, not just the parser.
func TestFCAEmptyCellIsAbsentNotZero(t *testing.T) {
	rows, _, err := fcaBuildRows(fcaTestSheet(), fcaEntities)
	if err != nil {
		t.Fatalf("fcaBuildRows: %v", err)
	}
	var feb *fcaRow
	for i := range rows {
		if rows[i].BillingMonth == "2020-02" && rows[i].Entity == "cppag" {
			feb = &rows[i]
		}
	}
	if feb == nil {
		t.Fatal("2020-02 cppag missing")
	}
	if feb.Allowed != nil {
		t.Fatalf("an EMPTY allowed cell became %v; absence is not a zero adjustment", *feb.Allowed)
	}
	if feb.Requested == nil || *feb.Requested != 3.0 {
		t.Fatalf("requested = %v, want 3.0", feb.Requested)
	}
	// With one side absent, neither the disallowance nor the comparison may
	// be computed: an arithmetic against a missing operand is an invention.
	if feb.Disallowance != nil {
		t.Fatalf("disallowance computed against a missing operand: %v", *feb.Disallowance)
	}
	if feb.AllowedBelowRequested != nil {
		t.Fatalf("allowed_below_requested decided with one side absent: %v", *feb.AllowedBelowRequested)
	}
}

// TestFCACumulativeSumsNegativesWithTheirSign pins the aggregation against
// hand-computed arithmetic over the fixture.
func TestFCACumulativeSumsNegativesWithTheirSign(t *testing.T) {
	rows, _, err := fcaBuildRows(fcaTestSheet(), fcaEntities)
	if err != nil {
		t.Fatalf("fcaBuildRows: %v", err)
	}
	cum := fcaBuildCumulative(rows, fcaEntities)
	byID := map[string]fcaCumulative{}
	for _, c := range cum {
		byID[c.Entity] = c
	}

	ke := byID["ke"]
	// requested: 2.0000 + (1.798)-> -1.798 + 0.657 + "" absent = 0.859
	if ke.Requested == nil || math.Abs(*ke.Requested-0.859) > 1e-9 {
		t.Fatalf("ke cumulative requested = %v, want 0.859 (a parenthesised cell summed as negative)", ke.Requested)
	}
	// allowed: (2.5935)-> -2.5935 + 1.0 + 0.9426 + 1.0 = 0.3491
	if ke.Allowed == nil || math.Abs(*ke.Allowed-0.3491) > 1e-9 {
		t.Fatalf("ke cumulative allowed = %v, want 0.3491", ke.Allowed)
	}
	if ke.MonthsRequested != 3 {
		t.Fatalf("ke months_requested = %d, want 3 (the empty cell must not count)", ke.MonthsRequested)
	}
	if ke.MonthsAllowed != 4 {
		t.Fatalf("ke months_allowed = %d, want 4", ke.MonthsAllowed)
	}
	if ke.MonthsBoth != 3 {
		t.Fatalf("ke months_both = %d, want 3", ke.MonthsBoth)
	}
}

// TestFCACumulativeCarriesItsDenominator pins that a total can never be read
// without the month count it was summed over. The same disallowance over 12
// months and over 48 months means two different things.
func TestFCACumulativeCarriesItsDenominator(t *testing.T) {
	rows, _, err := fcaBuildRows(fcaTestSheet(), fcaEntities)
	if err != nil {
		t.Fatalf("fcaBuildRows: %v", err)
	}
	for _, c := range fcaBuildCumulative(rows, fcaEntities) {
		if c.Basis == "" {
			t.Fatalf("%s cumulative carries no basis prose", c.Entity)
		}
		if !strings.Contains(c.Basis, "months") {
			t.Fatalf("%s basis does not state the month counts: %q", c.Entity, c.Basis)
		}
		if c.Disallowance != nil && c.MonthsBoth == 0 {
			t.Fatalf("%s has a disallowance over zero contributing months", c.Entity)
		}
	}
}

// TestFCAUnmeasuredTotalIsAbsentNotZero pins that an entity with nothing
// published returns no number at all, rather than 0.00 — the same defect the
// sibling MWSum type exists to prevent.
func TestFCAUnmeasuredTotalIsAbsentNotZero(t *testing.T) {
	blank := []map[string]any{{
		"Year": "2020", "Month": "Jan",
		"CPPA - G FCA Requested": "", "CPPA - G FCA Allowed (kWh)": "",
		"K-Electric FCA Requested": "", "K-Electric FCA Allowed": "",
	}}
	rows, _, err := fcaBuildRows(blank, fcaEntities)
	if err != nil {
		t.Fatalf("fcaBuildRows: %v", err)
	}
	for _, c := range fcaBuildCumulative(rows, fcaEntities) {
		if c.Requested != nil {
			t.Fatalf("%s reported a cumulative requested of %v over zero published months", c.Entity, *c.Requested)
		}
		if c.Allowed != nil {
			t.Fatalf("%s reported a cumulative allowed of %v over zero published months", c.Entity, *c.Allowed)
		}
		if c.Disallowance != nil {
			t.Fatalf("%s reported a cumulative disallowance over zero published months", c.Entity)
		}
	}
}

// TestFCAEntityColumnsAreVerbatim pins the sheet's own header spellings. The
// CPPA-G columns carry spaces around the hyphen and only the allowed column
// has a unit suffix; a tidied copy of either string silently matches nothing
// and every figure would come back absent.
func TestFCAEntityColumnsAreVerbatim(t *testing.T) {
	want := map[string][2]string{
		"cppag": {"CPPA - G FCA Requested", "CPPA - G FCA Allowed (kWh)"},
		"ke":    {"K-Electric FCA Requested", "K-Electric FCA Allowed"},
	}
	if len(fcaEntities) != 2 {
		t.Fatalf("fcaEntities has %d entries, want exactly 2", len(fcaEntities))
	}
	for _, e := range fcaEntities {
		w, ok := want[e.ID]
		if !ok {
			t.Fatalf("unexpected entity %q", e.ID)
		}
		if e.RequestedColumn != w[0] || e.AllowedColumn != w[1] {
			t.Fatalf("%s columns = %q / %q, want %q / %q", e.ID, e.RequestedColumn, e.AllowedColumn, w[0], w[1])
		}
	}
	// And the sheet must actually be keyed by those exact strings.
	rows, _, err := fcaBuildRows(fcaTestSheet(), fcaEntities)
	if err != nil {
		t.Fatalf("fcaBuildRows: %v", err)
	}
	found := 0
	for _, r := range rows {
		if r.Requested != nil || r.Allowed != nil {
			found++
		}
	}
	if found == 0 {
		t.Fatal("no figure joined against the verbatim column names")
	}
}

// TestFCAEntitiesAreNeverSummedTogether pins the refusal. CPPA-G and
// K-Electric are separate tariffs for separate consumer populations,
// published side by side; one combined total would be meaningless.
func TestFCAEntitiesAreNeverSummedTogether(t *testing.T) {
	rows, _, err := fcaBuildRows(fcaTestSheet(), fcaEntities)
	if err != nil {
		t.Fatalf("fcaBuildRows: %v", err)
	}
	cum := fcaBuildCumulative(rows, fcaEntities)
	if len(cum) != 2 {
		t.Fatalf("cumulative returned %d entries, want one per entity; a combined row would be a sum across two different tariffs", len(cum))
	}
	seen := map[string]bool{}
	for _, c := range cum {
		if seen[c.Entity] {
			t.Fatalf("entity %q appears twice", c.Entity)
		}
		seen[c.Entity] = true
	}
}

// TestFCABillingMonthForm pins the YYYY-MM shape check, which is separate
// from whether the sheet publishes that month (a usage error versus a
// not-found).
func TestFCABillingMonthForm(t *testing.T) {
	for _, ok := range []string{"2020-01", "2018-07", "2022-12"} {
		if !fcaValidMonthForm(ok) {
			t.Fatalf("%q was rejected", ok)
		}
	}
	for _, bad := range []string{"", "2020", "2020-1", "2020-13", "2020-00", "notamonth", "20-01", "2020/01"} {
		if fcaValidMonthForm(bad) {
			t.Fatalf("%q was accepted as a billing month", bad)
		}
	}
}

// TestFCAWindowComesFromTheRows pins that the published window is derived
// from the data rather than hardcoded, so it cannot drift from the sheet.
func TestFCAWindowComesFromTheRows(t *testing.T) {
	rows, _, err := fcaBuildRows(fcaTestSheet(), fcaEntities)
	if err != nil {
		t.Fatalf("fcaBuildRows: %v", err)
	}
	if got, want := fcaWindow(rows), "2018-07 .. 2020-02"; got != want {
		t.Fatalf("window = %q, want %q", got, want)
	}
	if got := fcaWindow(nil); !strings.Contains(got, "no published months") {
		t.Fatalf("empty window = %q", got)
	}
}

// TestFCAUnknownMonthSpellingIsSkippedAndReported pins that a row this build
// cannot place in time is dropped WITH a warning, never given a neighbour's
// date.
func TestFCAUnknownMonthSpellingIsSkippedAndReported(t *testing.T) {
	sheet := append(fcaTestSheet(), map[string]any{
		"Year": "2021", "Month": "Smarch",
		"CPPA - G FCA Requested": "1.0", "CPPA - G FCA Allowed (kWh)": "1.0",
	})
	rows, warnings, err := fcaBuildRows(sheet, fcaEntities)
	if err != nil {
		t.Fatalf("fcaBuildRows: %v", err)
	}
	for _, r := range rows {
		if strings.HasPrefix(r.BillingMonth, "2021") {
			t.Fatalf("a row with an unreadable month was placed at %s", r.BillingMonth)
		}
	}
	if len(warnings) == 0 {
		t.Fatal("a skipped row produced no warning; a silent drop is indistinguishable from an empty source")
	}
}

// TestFCAFlagsAreDeclaredOnTheCommand pins the wiring: the three approved
// flags must exist on the shipped `fca`, which is a GENERATED command the
// hook augments in place.
func TestFCAFlagsAreDeclaredOnTheCommand(t *testing.T) {
	cmd, rest, err := RootCmd().Find([]string{"fca"})
	if err != nil || len(rest) != 0 || cmd.Name() != "fca" {
		t.Fatalf("fca did not resolve: %v rest=%v", err, rest)
	}
	for _, f := range []string{"entity", "cumulative", "billing-month"} {
		if cmd.Flags().Lookup(f) == nil {
			t.Fatalf("--%s is approved in transcendence row 9 but not declared on fca", f)
		}
	}
	// The generated command's own identity must survive the augmentation.
	if cmd.Annotations["mcp:read-only"] != "true" {
		t.Fatal("augmenting fca dropped its mcp:read-only annotation")
	}
	if cmd.RunE == nil {
		t.Fatal("fca has no RunE after augmentation")
	}
}

// TestFCADerivedFiguresCarryNoFloatDust pins the exact published decimals.
//
// This is a REGRESSION TEST for a real defect. The first build of this
// command emitted cumulative_disallowance 8.842499999999998 and a monthly
// disallowance of 0.37749999999999995, because float64 cannot represent the
// published operands and 48 accumulated differences drift. A test that
// printed the value with %.4f PASSED against that build — the dust was
// invisible to formatted output and only shows in the raw JSON number, which
// is what a consumer actually parses.
//
// Rounding is applied to DERIVED values only; a published operand is parsed
// exactly as printed and is never touched.
func TestFCADerivedFiguresCarryNoFloatDust(t *testing.T) {
	// The real corpus arithmetic that produced the dust.
	//
	// THE OPERANDS MUST BE float64 VARIABLES, NOT LITERALS. Go evaluates
	// untyped constant arithmetic at arbitrary precision, so `1.4883 - 1.1108`
	// written inline folds to EXACTLY 0.3775 at compile time and the runtime
	// dust never occurs. An earlier version of this test did exactly that and
	// passed against a build with no rounding at all.
	for _, tc := range []struct {
		a, b, want float64
	}{
		{9.9095, 9.8972, 0.0123},
		{1.4883, 1.1108, 0.3775},
		{11.389, 11.1023, 0.2867},
	} {
		dusty := tc.a - tc.b
		if dusty == tc.want {
			t.Fatalf("%v - %v produced no dust at runtime (%v); this case asserts nothing", tc.a, tc.b, dusty)
		}
		if got := fcaRound(dusty); got != tc.want {
			t.Fatalf("fcaRound(%v - %v) = %v, want exactly %v", tc.a, tc.b, got, tc.want)
		}
	}
	// Rounding must not move a value that is already at the published
	// precision, and must preserve sign.
	for _, v := range []float64{8.8425, 81.9748, 73.1323, -2.5935, 0, -0.0379} {
		if got := fcaRound(v); got != v {
			t.Fatalf("fcaRound(%v) = %v; a value already at the published precision must not move", v, got)
		}
	}

	// And end to end over the fixture, against EXPECTED DECIMALS rather than
	// against the value's own rounding. Comparing x to fcaRound(x) is
	// tautological whenever fcaRound is the identity, which is precisely the
	// regression this test exists to catch.
	rows, _, err := fcaBuildRows(fcaTestSheet(), fcaEntities)
	if err != nil {
		t.Fatalf("fcaBuildRows: %v", err)
	}
	wantMonth := map[string]float64{
		// 1.4883 - 1.1108, the dusty one.
		"2020-01/cppag": 0.3775,
		// 0.657 - 0.9426: allowed ABOVE requested, and also dusty.
		"2020-01/ke": -0.2856,
	}
	seen := 0
	for _, r := range rows {
		want, ok := wantMonth[r.BillingMonth+"/"+r.Entity]
		if !ok || r.Disallowance == nil {
			continue
		}
		seen++
		if *r.Disallowance != want {
			t.Fatalf("%s %s disallowance = %v, want exactly %v", r.Entity, r.BillingMonth, *r.Disallowance, want)
		}
	}
	if seen != len(wantMonth) {
		t.Fatalf("checked %d of %d expected months", seen, len(wantMonth))
	}
	// The cumulative over the fixture, hand-computed at the published
	// precision: cppag requested 1+2+1.4883+3 = 7.4883, allowed
	// 0.5+2+1.1108 = 3.6108, disallowance over the three both-published
	// months 0.5 + 0 + 0.3775 = 0.8775.
	for _, c := range fcaBuildCumulative(rows, fcaEntities) {
		if c.Entity != "cppag" {
			continue
		}
		if c.Requested == nil || *c.Requested != 7.4883 {
			t.Fatalf("cppag cumulative requested = %v, want exactly 7.4883", c.Requested)
		}
		if c.Allowed == nil || *c.Allowed != 3.6108 {
			t.Fatalf("cppag cumulative allowed = %v, want exactly 3.6108", c.Allowed)
		}
		if c.Disallowance == nil || *c.Disallowance != 0.8775 {
			t.Fatalf("cppag cumulative disallowance = %v, want exactly 0.8775", c.Disallowance)
		}
	}
}

// TestFCAPublishedDecimalsMatchesTheSource pins the rounding precision to a
// MEASURED property of the sheet rather than to a hand-picked constant.
//
// The contract is equality, in both directions. Rounding derived values
// COARSER than the source would discard published detail; rounding FINER
// leaves representation dust in place for any figure that needs more
// accumulation than this corpus happens to require. Measured on the live
// sheet: every one of the 48 rows' figures carries at most four decimal
// places, so four is the source's own precision.
func TestFCAPublishedDecimalsMatchesTheSource(t *testing.T) {
	maxDecimals := 0
	cells := 0
	for _, rec := range fcaTestSheet() {
		for key, v := range rec {
			if key == "Year" || key == "Month" {
				continue
			}
			s, _ := v.(string)
			s = strings.TrimSpace(strings.Trim(strings.TrimSpace(s), "()"))
			if s == "" {
				continue
			}
			cells++
			if i := strings.IndexByte(s, '.'); i >= 0 {
				if d := len(s) - i - 1; d > maxDecimals {
					maxDecimals = d
				}
			}
		}
	}
	if cells == 0 {
		t.Fatal("no figure cells in the fixture; this test would assert nothing")
	}
	if maxDecimals != fcaPublishedDecimals {
		t.Fatalf("the sheet publishes at most %d decimal places but fcaPublishedDecimals is %d. "+
			"Rounding coarser than the source discards published detail; rounding finer leaves "+
			"representation dust in place. The two must be equal, and changing the constant is a "+
			"deliberate decision about the SOURCE, not a tuning knob.",
			maxDecimals, fcaPublishedDecimals)
	}
}
