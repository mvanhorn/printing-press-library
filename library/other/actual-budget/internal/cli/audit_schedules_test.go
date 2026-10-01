// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/cliutil/testenv"
)

// TestNovelAuditSchedulesHelpWires smoke-tests that the audit schedules command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelAuditSchedulesHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"audit", "schedules", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("audit schedules --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "schedules"} {
		if !strings.Contains(help, want) {
			t.Fatalf("audit schedules --help missing %q in output:\n%s", want, help)
		}
	}
}

type s3SchedRow struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Payee          string   `json:"payee"`
	ExpectedAmount int64    `json:"expected_amount"`
	NextDate       string   `json:"next_date"`
	LastPosted     string   `json:"last_posted"`
	LastAmount     *int64   `json:"last_amount"`
	Findings       []string `json:"findings"`
}

func TestAuditSchedulesOverdueAndDrift(t *testing.T) {
	s3Fixture(t)
	var rows []s3SchedRow
	s3RunJSON(t, &rows, "audit", "schedules", "--as-of", "2026-09-30")
	by := map[string]s3SchedRow{}
	for _, r := range rows {
		by[r.Name] = r
	}
	nf := by["Netflix"]
	if !slices.Contains(nf.Findings, "overdue") || nf.NextDate != "2026-09-10" || nf.Payee != "Netflix" {
		t.Errorf("Netflix: %+v", nf)
	}
	rent := by["Rent"]
	if !slices.Contains(rent.Findings, "amount-drift") || slices.Contains(rent.Findings, "overdue") {
		t.Errorf("Rent findings: %+v", rent)
	}
	if rent.ExpectedAmount != -150000 || rent.LastAmount == nil || *rent.LastAmount != -165000 || rent.NextDate != "2026-10-03" || rent.LastPosted != "2026-09-03" {
		t.Errorf("Rent row: %+v", rent)
	}
}

func TestAuditSchedulesToleranceHidesDrift(t *testing.T) {
	s3Fixture(t)
	var rows []s3SchedRow
	s3RunJSON(t, &rows, "audit", "schedules", "--as-of", "2026-09-30", "--tolerance", "15")
	for _, r := range rows {
		if r.Name == "Rent" {
			t.Fatalf("Rent should have no findings at 15%% tolerance: %+v", r)
		}
	}
	if _, _, err := s3Run(t, "audit", "schedules", "--tolerance", "x"); processExitCode(err) != 2 {
		t.Errorf("--tolerance x: exit %d", processExitCode(err))
	}
}

func TestAuditSchedulesNoMirrorEmpty(t *testing.T) {
	s3NoMirror(t)
	out, _, err := s3Run(t, "audit", "schedules")
	if err != nil || strings.TrimSpace(out) != "[]" {
		t.Fatalf("no mirror: out=%q err=%v", out, err)
	}
}
