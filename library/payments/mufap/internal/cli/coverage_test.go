// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestNovelCoverageHelpWires smoke-tests that the coverage command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelCoverageHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"coverage", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("coverage --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "coverage"} {
		if !strings.Contains(help, want) {
			t.Fatalf("coverage --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestNovelCoverageMonthlyCadenceWalk pins the cadence fix: "monthly" and
// "allocation" are mirrored under one month-end key per month (backfill builds
// them as time.Date(y, m+1, 0)), so a daily walk over an eight-month range
// fabricated ~240 never-fetched dates per monthly resource against ~8 real
// ones and reported the fabrications in summary.dates_never_fetched.
func TestNovelCoverageMonthlyCadenceWalk(t *testing.T) {
	from, _ := time.Parse("2006-01-02", "2026-01-01")
	to, _ := time.Parse("2006-01-02", "2026-09-04")
	ends := coverageMonthEndsIn(from, to)
	if len(ends) != 8 {
		t.Fatalf("coverageMonthEndsIn(2026-01-01..2026-09-04) = %d month-ends, want 8", len(ends))
	}
	if got := ends[0].Format("2006-01-02"); got != "2026-01-31" {
		t.Errorf("first month-end = %q, want 2026-01-31", got)
	}
	if got := ends[7].Format("2006-01-02"); got != "2026-08-31" {
		t.Errorf("last month-end = %q, want 2026-08-31 (September's end is past --to, so it is not a hole in this range)", got)
	}
	// A window that contains no month-end contains no monthly publication,
	// so it must claim no monthly hole at all.
	pFrom, _ := time.Parse("2006-01-02", "2026-07-01")
	pTo, _ := time.Parse("2006-01-02", "2026-07-15")
	if got := coverageMonthEndsIn(pFrom, pTo); len(got) != 0 {
		t.Errorf("coverageMonthEndsIn(2026-07-01..2026-07-15) = %v, want none", got)
	}
	for _, res := range []string{"monthly", "allocation"} {
		if coverageCadence(res) != "monthly" {
			t.Errorf("coverageCadence(%q) = %q, want monthly", res, coverageCadence(res))
		}
	}
	if coverageCadence("daily-returns") != "daily" {
		t.Errorf("coverageCadence(daily-returns) = %q, want daily", coverageCadence("daily-returns"))
	}
}

// TestNovelCoverageLedgerMonthAcceptsMUFAPKey pins the M-YYYY fold. The
// allocation endpoint is addressed as "7-2026" and the mirror keeps whichever
// key the fetch used (verify.go), while the store filters dates lexically --
// so without this fold an ISO --from/--to window drops the row and reports a
// fully mirrored month as a hole.
func TestNovelCoverageLedgerMonthAcceptsMUFAPKey(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"7-2026", "2026-07"},
		{"12-2026", "2026-12"},
		{"2026-07", "2026-07"},
		{"2026-07-31", "2026-07"},
	} {
		got, ok := coverageLedgerMonth(tc.in)
		if !ok || got != tc.want {
			t.Errorf("coverageLedgerMonth(%q) = %q,%v want %q,true", tc.in, got, ok, tc.want)
		}
	}
	for _, bad := range []string{"", "garbage", "2026-13-01"} {
		if got, ok := coverageLedgerMonth(bad); ok {
			t.Errorf("coverageLedgerMonth(%q) = %q,true want not-ok", bad, got)
		}
	}
	if coverageIsISODate("7-2026") {
		t.Error("coverageIsISODate(7-2026) = true; the MUFAP key must not be mistaken for an ISO date")
	}
	if !coverageIsISODate("2026-07-31") {
		t.Error("coverageIsISODate(2026-07-31) = false")
	}
}

// TestNovelCoverageMissingMirrorReportsHoles pins two fixes at once: a missing
// data.db must emit the same coverageView object as the success path (it used
// to emit a bare [], breaking any consumer decoding the documented object),
// and it must still run the never-fetched walk -- "no mirror at all" is
// precisely the case where every date in the range is a hole.
func TestNovelCoverageMissingMirrorReportsHoles(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.db")
	cmd := RootCmd()
	cmd.SetArgs([]string{"coverage", "--json", "--no-learn",
		"--home", t.TempDir(), "--db", missing,
		"--resource", "daily-returns", "--from", "2026-09-01", "--to", "2026-09-04"})
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("coverage on a missing mirror error = %v", err)
	}
	var view struct {
		From                string `json:"from"`
		To                  string `json:"to"`
		NeverFetchedChecked bool   `json:"never_fetched_checked"`
		Rows                []struct {
			Resource string `json:"resource"`
			Date     string `json:"date"`
			Status   string `json:"status"`
		} `json:"rows"`
		Summary []struct {
			Resource          string `json:"resource"`
			Cadence           string `json:"cadence"`
			DatesNeverFetched int    `json:"dates_never_fetched"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(out.Bytes(), &view); err != nil {
		t.Fatalf("machine output is not the documented coverageView object: %v\n%s", err, out.String())
	}
	if !view.NeverFetchedChecked || view.From != "2026-09-01" || view.To != "2026-09-04" {
		t.Errorf("view echoed range %q..%q checked=%v; want the requested range, checked", view.From, view.To, view.NeverFetchedChecked)
	}
	if len(view.Rows) != 4 {
		t.Fatalf("rows = %d, want 4 never-fetched dates (an absent mirror is a hole on every date)", len(view.Rows))
	}
	for _, r := range view.Rows {
		if r.Status != coverageStatusNever {
			t.Errorf("row %s/%s status = %q, want %q", r.Resource, r.Date, r.Status, coverageStatusNever)
		}
	}
	if len(view.Summary) != 1 || view.Summary[0].DatesNeverFetched != 4 || view.Summary[0].Cadence != "daily" {
		t.Errorf("summary = %+v, want one daily resource with 4 never-fetched dates", view.Summary)
	}
	if !strings.Contains(errBuf.String(), "no local mirror") {
		t.Errorf("stderr missing the 'no local mirror' hint: %q", errBuf.String())
	}
}

// TestNovelCoverageGapsOnlyWithoutRangeDoesNotClaimAllClear pins the false
// all-clear: --gaps-only with no --from/--to never runs the never-fetched
// walk, so an empty result proves only that no ledger row is empty. The old
// wording ("no gaps: every date in range was fetched") asserted the opposite
// of what ran, and returned before the caveat could print.
func TestNovelCoverageGapsOnlyWithoutRangeDoesNotClaimAllClear(t *testing.T) {
	// --human-friendly writes a package global; restore it so the rest of the
	// package's tests keep the routing they were written against.
	prevHuman := humanFriendly
	defer func() { humanFriendly = prevHuman }()

	missing := filepath.Join(t.TempDir(), "absent.db")
	cmd := RootCmd()
	cmd.SetArgs([]string{"coverage", "--human-friendly", "--gaps-only", "--no-learn",
		"--home", t.TempDir(), "--db", missing})
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("coverage --gaps-only error = %v", err)
	}
	got := out.String()
	if strings.Contains(got, "no gaps") {
		t.Errorf("--gaps-only without a range still claims %q:\n%s", "no gaps", got)
	}
	if !strings.Contains(got, "NOT checked") || !strings.Contains(got, coverageNeverCaveat) {
		t.Errorf("--gaps-only without a range must say the walk did not run and print the caveat:\n%s", got)
	}
}

// TestNovelCoverageHelpDoesNotOverstateNeverFetched pins the wording: the
// ledger records successful saves only and has no outcome column, so a date
// that failed on every attempt is indistinguishable from one never attempted.
func TestNovelCoverageHelpDoesNotOverstateNeverFetched(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"coverage", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("coverage --help error = %v", err)
	}
	help := out.String()
	for _, want := range []string{"every attempt failed", "month-end"} {
		if !strings.Contains(help, want) {
			t.Errorf("coverage --help missing %q:\n%s", want, help)
		}
	}
}
