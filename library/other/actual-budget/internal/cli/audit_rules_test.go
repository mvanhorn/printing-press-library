// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/cliutil/testenv"
)

// TestNovelAuditRulesHelpWires smoke-tests that the audit rules command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelAuditRulesHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"audit", "rules", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("audit rules --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "rules"} {
		if !strings.Contains(help, want) {
			t.Fatalf("audit rules --help missing %q in output:\n%s", want, help)
		}
	}
}

type s3RuleRow struct {
	ID         string   `json:"id"`
	Summary    string   `json:"summary"`
	MatchCount int      `json:"match_count"`
	LastMatch  string   `json:"last_match"`
	Findings   []string `json:"findings"`
	Dangling   []string `json:"dangling"`
}

func s3RuleFindings(rows []s3RuleRow) map[string]s3RuleRow {
	by := map[string]s3RuleRow{}
	for _, r := range rows {
		by[r.ID] = r
	}
	return by
}

func s3HasFinding(r s3RuleRow, prefix string) bool {
	for _, f := range r.Findings {
		if strings.HasPrefix(f, prefix) {
			return true
		}
	}
	return false
}

func TestAuditRulesFindings(t *testing.T) {
	s3Fixture(t)
	var rows []s3RuleRow
	s3RunJSON(t, &rows, "audit", "rules", "--months", "0")
	by := s3RuleFindings(rows)
	if !s3HasFinding(by["rule-dead"], "never-matches") {
		t.Errorf("rule-dead: %+v", by["rule-dead"])
	}
	dangle := by["rule-dangling"]
	if !s3HasFinding(dangle, "dangling-reference") || len(dangle.Dangling) != 2 ||
		!strings.Contains(strings.Join(dangle.Dangling, " "), "pay-krogerold") || !strings.Contains(strings.Join(dangle.Dangling, " "), "cat-oldfood") {
		t.Errorf("rule-dangling: %+v", dangle)
	}
	if !s3HasFinding(by["rule-kroger"], "shadowed-by:rule-shadow") || !s3HasFinding(by["rule-shadow"], "shadowed-by:rule-kroger") {
		t.Errorf("conflict not reported on both: kroger=%+v shadow=%+v", by["rule-kroger"], by["rule-shadow"])
	}
	// Conditions compare against the RESOLVED payee id, so the merged
	// KROGER #123 transaction (txn-kr-3) counts: kr-1..kr-6 = 6.
	if by["rule-kroger"].MatchCount != 6 || by["rule-kroger"].LastMatch != "2026-09-06" {
		t.Errorf("rule-kroger match_count=%d last=%s, want 6 / 2026-09-06", by["rule-kroger"].MatchCount, by["rule-kroger"].LastMatch)
	}
	for _, id := range []string{"rule-rent-sched", "rule-netflix-sched"} {
		if _, shown := by[id]; shown {
			t.Errorf("schedule rule %s reported without findings: %+v", id, by[id])
		}
	}
}

func TestAuditRulesAllIncludesScheduleRulesWithoutNeverMatches(t *testing.T) {
	s3Fixture(t)
	var rows []s3RuleRow
	s3RunJSON(t, &rows, "audit", "rules", "--months", "0", "--all")
	by := s3RuleFindings(rows)
	if len(rows) != 6 {
		t.Fatalf("--all returned %d rules, want 6", len(rows))
	}
	for _, id := range []string{"rule-rent-sched", "rule-netflix-sched"} {
		if s3HasFinding(by[id], "never-matches") || s3HasFinding(by[id], "unevaluated") {
			t.Errorf("%s: %+v", id, by[id])
		}
	}
}

func TestAuditRulesWindowAndValidation(t *testing.T) {
	s3Fixture(t)
	var rows []s3RuleRow
	// A 1-month window ending 2026-07-01 contains no transactions: the Kroger rule never matches.
	s3RunJSON(t, &rows, "audit", "rules", "--months", "1", "--as-of", "2026-07-01")
	if !s3HasFinding(s3RuleFindings(rows)["rule-kroger"], "never-matches") {
		t.Errorf("windowed rule-kroger: %+v", s3RuleFindings(rows)["rule-kroger"])
	}
	if _, _, err := s3Run(t, "audit", "rules", "--months", "-1"); s3ExitCode(err) != 2 {
		t.Errorf("--months -1: exit %d", s3ExitCode(err))
	}
}

func TestAuditRulesNoMirrorEmpty(t *testing.T) {
	s3NoMirror(t)
	out, _, err := s3Run(t, "audit", "rules")
	if err != nil || strings.TrimSpace(out) != "[]" {
		t.Fatalf("no mirror: out=%q err=%v", out, err)
	}
}
