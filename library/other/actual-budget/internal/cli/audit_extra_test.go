// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"
)

// processExitCode is the exit code the binary reports for err: Execute maps
// Cobra/pflag parse failures to usage errors (exit 2) before main calls
// ExitCode, which the in-process s3Run path skips.
func processExitCode(err error) int {
	if isCobraUsageError(err) {
		return 2
	}
	return s3ExitCode(err)
}

// Numeric flags are typed: non-numeric values are usage errors (exit 2) and
// defaults come from the named constants.
func TestAuditNumericFlagsTyped(t *testing.T) {
	s3Fixture(t)
	bad := [][]string{
		{"audit", "rules", "--months", "abc"},
		{"audit", "rules", "--months", "1.5"},
		{"audit", "schedules", "--tolerance", "x"},
		{"audit", "schedules", "--tolerance", "-1"},
		{"audit", "payees", "--min-similarity", "abc"},
		{"audit", "payees", "--min-similarity", "1.5"},
	}
	for _, args := range bad {
		if _, _, err := s3Run(t, args...); processExitCode(err) != 2 {
			t.Errorf("%v: exit %d (%v), want 2", args, processExitCode(err), err)
		}
	}
	for _, c := range []struct{ cmd, flag, want string }{
		{"rules", "months", "12"},
		{"schedules", "tolerance", "5"},
		{"payees", "min-similarity", "0.85"},
	} {
		sub, _, err := RootCmd().Find([]string{"audit", c.cmd})
		if err != nil {
			t.Fatal(err)
		}
		if f := sub.Flags().Lookup(c.flag); f == nil || f.DefValue != c.want {
			t.Errorf("audit %s --%s default = %v, want %s", c.cmd, c.flag, f, c.want)
		}
	}
}

// --as-of accepts YYYYMMDD as well as YYYY-MM-DD and gives the same result.
func TestAuditAsOfCompactDate(t *testing.T) {
	s3Fixture(t)
	var dashed, compact []s3SchedRow
	s3RunJSON(t, &dashed, "audit", "schedules", "--as-of", "2026-09-30")
	s3RunJSON(t, &compact, "audit", "schedules", "--as-of", "20260930")
	if len(dashed) == 0 || len(dashed) != len(compact) {
		t.Fatalf("dashed=%+v compact=%+v", dashed, compact)
	}
	var rows []s3RuleRow
	s3RunJSON(t, &rows, "audit", "rules", "--months", "1", "--as-of", "20260701")
	if !s3HasFinding(s3RuleFindings(rows)["rule-kroger"], "never-matches") {
		t.Errorf("compact --as-of window: rule-kroger = %+v", s3RuleFindings(rows)["rule-kroger"])
	}
	for _, args := range [][]string{
		{"audit", "rules", "--as-of", "2026-13-01"},
		{"audit", "schedules", "--as-of", "20261301"},
		{"changes", "since", "7d", "--as-of", "nope"},
	} {
		if _, _, err := s3Run(t, args...); s3ExitCode(err) != 2 {
			t.Errorf("%v: exit %d, want 2", args, s3ExitCode(err))
		}
	}
}

func TestParseSinceCompactDate(t *testing.T) {
	ref := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	want := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for _, in := range []string{"2026-09-01", "20260901"} {
		got, err := parseSince(in, ref)
		if err != nil || !got.Equal(want) {
			t.Errorf("parseSince(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if got, err := parseSince("7d", ref); err != nil || !got.Equal(ref.Add(-7*24*time.Hour)) {
		t.Errorf("parseSince(7d) = %v, %v", got, err)
	}
}

// --timeout bounds the command's context: an already-elapsed deadline fails
// instead of running unbounded.
func TestAuditTimeoutApplies(t *testing.T) {
	s3Fixture(t)
	for _, args := range [][]string{
		{"audit", "payees", "--timeout", "1ns"},
		{"audit", "rules", "--timeout", "1ns"},
		{"audit", "schedules", "--timeout", "1ns"},
		{"templates", "status", "--month", "2026-09", "--timeout", "1ns"},
		{"changes", "since", "30d", "--timeout", "1ns"},
	} {
		if _, _, err := s3Run(t, args...); err == nil {
			t.Errorf("%v: expected a deadline error", args)
		}
	}
}
