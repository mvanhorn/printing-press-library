// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/mufap"
	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/store"
)

// TestNovelRatesHelpWires smoke-tests that the rates command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelRatesHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"rates", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("rates --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "rates"} {
		if !strings.Contains(help, want) {
			t.Fatalf("rates --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestNovelRatesRejectsUnknownColumn guards the one input the command cannot
// recover from. A header label that the returns tab does not publish parses as
// "no value" on every row, so an unvalidated typo would report an empty series
// that reads exactly like an unpopulated mirror. --no-learn keeps the check off
// the local store, which the column validation is required to precede anyway.
func TestNovelRatesRejectsUnknownColumn(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"rates", "--from", "2026-09-01", "--column", "45 Days", "--no-learn"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("rates --column '45 Days' succeeded; want a usage error. output:\n%s", out.String())
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("rates --column '45 Days' exit code = %d, want 2 (usage); err = %v", got, err)
	}
}

// TestNovelRatesFundCountTravelsWithRate exercises the derivation the command
// wraps. Two facts are load-bearing for the emitted series and are asserted
// here rather than through the store: only annualized money-market rows feed
// the median, and fund_count reports how many of them actually reported, so a
// consumer can tell a three-fund median from a thirty-fund one.
func TestNovelRatesFundCountTravelsWithRate(t *testing.T) {
	const column = "30 Days"
	rows := []map[string]string{
		{"Fund Name": "A Cash", "Category": "Money Market (Annualized Return )", column: "10.00"},
		{"Fund Name": "B Cash", "Category": "Money Market (Annualized Return )", column: "12.00"},
		{"Fund Name": "C Cash", "Category": "Money Market (Annualized Return )", column: "14.00"},
		// Did not publish: excluded, never zero-filled.
		{"Fund Name": "D Cash", "Category": "Money Market (Annualized Return )", column: "-"},
		// Absolute-return equity: a price change, not a yield.
		{"Fund Name": "E Equity", "Category": "Equity", column: "40.00"},
	}

	pt := mufap.DeriveRate("2026-09-04", column, rows)
	if pt.FundCount != 3 {
		t.Fatalf("FundCount = %d, want 3 (non-reporting and non-money-market rows must not count)", pt.FundCount)
	}
	if pt.MedianYield != 12 {
		t.Fatalf("MedianYield = %v, want 12", pt.MedianYield)
	}
	if pt.Date != "2026-09-04" || pt.Column != column {
		t.Fatalf("point identity = (%q, %q), want (2026-09-04, %q)", pt.Date, pt.Column, column)
	}

	// A date the mirror holds but no money-market fund reported on must still
	// produce a point: the command emits it with fund_count 0 so the JSON shows
	// a thin date instead of silently dropping it from the series.
	empty := mufap.DeriveRate("2026-09-05", column, []map[string]string{
		{"Fund Name": "E Equity", "Category": "Equity", column: "40.00"},
	})
	if empty.FundCount != 0 {
		t.Fatalf("FundCount = %d for a date with no money-market fund, want 0", empty.FundCount)
	}
	if empty.MedianYield != 0 || empty.Date != "2026-09-05" {
		t.Fatalf("empty point = %+v, want a zero-valued rate still tagged with its date", empty)
	}
}

// TestNovelRatesSeriesKeepsFetchedAndEmptyDates pins the two facts a
// fund_count of 0 alone cannot carry.
//
// A date MUFAP published nothing on is mirrored with ZERO observation rows --
// backfill stores fetched-and-empty on purpose -- so it exists only in the
// coverage ledger. Building the series from observations alone dropped such a
// date silently, which is the exact hole the fund_count contract exists to
// show. And a date whose payloads failed to decode must not read as a date
// that was genuinely thin: malformed_rows keeps the failed read visible to a
// machine consumer reading stdout, not just to a human reading stderr.
func TestNovelRatesSeriesKeepsFetchedAndEmptyDates(t *testing.T) {
	const column = "30 Days"
	obs := []store.MUFAPObs{
		{Resource: "daily-returns", Date: "2026-09-01", Key: "a",
			Payload: `{"Fund Name":"A Cash","Category":"Money Market (Annualized Return )","30 Days":"10.00"}`},
		{Resource: "daily-returns", Date: "2026-09-01", Key: "b",
			Payload: `{"Fund Name":"B Cash","Category":"Money Market (Annualized Return )","30 Days":"12.00"}`},
		{Resource: "daily-returns", Date: "2026-09-01", Key: "c", Payload: `{not json`},
	}
	ledger := []store.MUFAPCoverage{
		{Resource: "daily-returns", Date: "2026-09-01", RowCount: 3},
		// Fetched, published nothing: a holiday, not a date nobody tried.
		{Resource: "daily-returns", Date: "2026-09-02", RowCount: 0},
	}

	points := ratesSeries(column, obs, ledger)
	if len(points) != 2 {
		t.Fatalf("ratesSeries returned %d point(s), want 2 (the fetched-and-empty date must not vanish): %+v", len(points), points)
	}
	if points[0].Date != "2026-09-01" || points[1].Date != "2026-09-02" {
		t.Fatalf("points out of date order: %+v", points)
	}
	if points[0].FundCount != 2 || points[0].StoredRows != 3 || points[0].MalformedRows != 1 {
		t.Fatalf("point 0 = fund_count %d, stored_rows %d, malformed_rows %d; want 2, 3, 1 (a payload that did not decode is a failed read, not a yield of zero)",
			points[0].FundCount, points[0].StoredRows, points[0].MalformedRows)
	}
	if points[1].FundCount != 0 || points[1].StoredRows != 0 || points[1].MalformedRows != 0 {
		t.Fatalf("point 1 = fund_count %d, stored_rows %d, malformed_rows %d; want 0, 0, 0 (stored_rows 0 is what marks a fetched-and-empty date)",
			points[1].FundCount, points[1].StoredRows, points[1].MalformedRows)
	}
	if points[1].Column != column {
		t.Fatalf("point 1 column = %q, want %q: an empty date still belongs to the requested column", points[1].Column, column)
	}
}
