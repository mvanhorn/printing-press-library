// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/store"
)

// TestNovelDispersionHelpWires smoke-tests that the dispersion command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelDispersionHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"dispersion", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dispersion --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "dispersion"} {
		if !strings.Contains(help, want) {
			t.Fatalf("dispersion --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestNovelDispersionRejectsEmptyCategory pins the guard that DeriveDispersion
// cannot supply itself: it filters only when the category is non-empty, so an
// empty --category matches every row and reduces annualized money-market yields
// and absolute equity returns to one median. The resulting p90-p10 gap compares
// a yield with a price change and is not a spread at all, so an empty value is
// a usage error rather than a silently meaningless answer. --no-learn keeps the
// check off the local store, which the validation must precede anyway.
func TestNovelDispersionRejectsEmptyCategory(t *testing.T) {
	for _, category := range []string{"", "   "} {
		cmd := RootCmd()
		cmd.SetArgs([]string{"dispersion", "--from", "2026-09-01", "--category", category, "--no-learn"})
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		err := cmd.Execute()
		if err == nil {
			t.Fatalf("dispersion --category %q succeeded; want a usage error. output:\n%s", category, out.String())
		}
		if got := ExitCode(err); got != 2 {
			t.Fatalf("dispersion --category %q exit code = %d, want 2 (usage); err = %v", category, got, err)
		}
	}
}

// TestNovelDispersionRejectsStrayPositional guards the arg shape. The command
// takes no positionals, but cobra's default ArbitraryArgs silently discarded
// them AND suppressed the bare-invocation help branch, so `dispersion
// 2026-09-01` answered over the whole mirror instead of that one date -- a
// wrong window rather than an error. The exit code is not asserted here because
// cobra's pre-RunE error is classified into code 2 by Execute(), which this
// test bypasses by driving RootCmd() directly.
func TestNovelDispersionRejectsStrayPositional(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"dispersion", "2026-09-01", "--no-learn"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("dispersion with a stray positional succeeded; want an error. output:\n%s", out.String())
	}
	if !strings.Contains(err.Error(), "2026-09-01") {
		t.Fatalf("error %q does not name the rejected positional", err.Error())
	}
}

// TestNovelDispersionSeriesSeparatesDecodeLossFromHolidays pins the fact a
// fund_count alone cannot carry. A mirrored payload that no longer decodes is
// a format mismatch, not a fund that returned zero: dropping it silently
// narrows the cross-section, and dropping every row of a date makes that date
// vanish from a series whose own help text tells the reader a missing date
// means MUFAP published nothing. Both losses are counted so neither can be
// read as a market fact.
func TestNovelDispersionSeriesSeparatesDecodeLossFromHolidays(t *testing.T) {
	const column = "YTD"
	obs := []store.MUFAPObs{
		{Resource: "daily-returns", Date: "2026-09-01", Key: "a",
			Payload: `{"Fund Name":"A Equity","Category":"Equity","YTD":"10.00"}`},
		{Resource: "daily-returns", Date: "2026-09-01", Key: "b",
			Payload: `{"Fund Name":"B Equity","Category":"Equity","YTD":"20.00"}`},
		{Resource: "daily-returns", Date: "2026-09-01", Key: "c",
			Payload: `{"Fund Name":"C Equity","Category":"Equity","YTD":"30.00"}`},
		{Resource: "daily-returns", Date: "2026-09-01", Key: "d", Payload: `{not json`},
		// Every row for this date fails to decode: a mirror defect, not a
		// holiday. It must not simply disappear.
		{Resource: "daily-returns", Date: "2026-09-02", Key: "e", Payload: `{also not json`},
		{Resource: "daily-returns", Date: "2026-09-03", Key: "f",
			Payload: `{"Fund Name":"A Equity","Category":"Equity","YTD":"5.00"}`},
		{Resource: "daily-returns", Date: "2026-09-03", Key: "g",
			Payload: `{"Fund Name":"B Equity","Category":"Equity","YTD":"7.00"}`},
	}

	res := dispersionSeries("Equity", column, obs)

	if res.StoredRows != len(obs) || res.MalformedRows != 2 {
		t.Fatalf("stored_rows %d, malformed %d; want %d, 2 (a payload that did not decode is a failed read, not a return of zero)",
			res.StoredRows, res.MalformedRows, len(obs))
	}
	if res.DecodeLostDates != 1 {
		t.Fatalf("decode-lost dates = %d, want 1 (2026-09-02 had rows and none decoded, so its absence is a mirror defect, not a holiday)", res.DecodeLostDates)
	}
	// All three OBSERVED dates stay in the series. 2026-09-02 is emitted with
	// fund_count 0 rather than dropped: it is a date the mirror holds, and
	// dropping it reported a real observation as missing data. fund_count 0 is
	// what a consumer gates on; DecodeLostDates says WHY it is zero.
	if len(res.Points) != 3 {
		t.Fatalf("points = %d, want 3 (every observed date stays in the series): %+v", len(res.Points), res.Points)
	}
	if res.Points[0].Date != "2026-09-01" || res.Points[0].FundCount != 3 {
		t.Fatalf("point 0 = %q fund_count %d, want 2026-09-01 fund_count 3 (the malformed row must not inflate or zero-fill the count)",
			res.Points[0].Date, res.Points[0].FundCount)
	}
	if res.Points[1].Date != "2026-09-02" || res.Points[1].FundCount != 0 {
		t.Fatalf("point 1 = %q fund_count %d, want 2026-09-02 fund_count 0 (a decode-lost date is observed, not absent)",
			res.Points[1].Date, res.Points[1].FundCount)
	}
	if res.Points[2].Date != "2026-09-03" || res.Points[2].FundCount != 2 {
		t.Fatalf("point 2 = %q fund_count %d, want 2026-09-03 fund_count 2", res.Points[2].Date, res.Points[2].FundCount)
	}
	if res.ThinDates != 1 {
		t.Fatalf("thin dates = %d, want 1 (2026-09-03 has two funds, below the %d-fund floor)", res.ThinDates, dispersionMinFunds)
	}
	if len(res.Pooled) != 1 || res.Pooled[0] != "Equity" {
		t.Fatalf("pooled labels = %v, want [Equity]: the point echoes the user's substring, so the labels actually pooled must be reported separately", res.Pooled)
	}
}

// TestNovelDispersionConventionsSplitsAnnualized covers the hazard that
// survives a non-empty --category: MUFAP suffixes the return convention onto
// the category label itself, so a substring match can straddle both and pool an
// annualized yield with an absolute price change.
func TestNovelDispersionConventionsSplitsAnnualized(t *testing.T) {
	annualized, absolute := dispersionConventions([]string{
		"Money Market (Annualized Return )",
		"VPS-Money Market",
	})
	if annualized != 1 || absolute != 1 {
		t.Fatalf("conventions = (%d annualized, %d absolute), want (1, 1)", annualized, absolute)
	}
	if a, b := dispersionConventions([]string{"Equity", "Asset Allocation"}); a != 0 || b != 2 {
		t.Fatalf("conventions = (%d annualized, %d absolute), want (0, 2)", a, b)
	}
}
