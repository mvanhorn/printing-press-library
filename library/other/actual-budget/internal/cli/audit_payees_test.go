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

// TestNovelAuditPayeesHelpWires smoke-tests that the audit payees command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelAuditPayeesHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"audit", "payees", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("audit payees --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "payees"} {
		if !strings.Contains(help, want) {
			t.Fatalf("audit payees --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestAuditPayeesClustersAmazonVariants(t *testing.T) {
	s3Fixture(t)
	var rows []payeeClusterRow
	s3RunJSON(t, &rows, "audit", "payees")
	if len(rows) != 1 {
		t.Fatalf("clusters = %d, want 1: %+v", len(rows), rows)
	}
	r := rows[0]
	if r.TargetID != "pay-amazon" || r.TargetName != "Amazon" {
		t.Fatalf("target = %s %s, want pay-amazon Amazon", r.TargetID, r.TargetName)
	}
	names := map[string]bool{}
	for _, m := range r.Merge {
		names[m.Name] = true
	}
	if len(r.Merge) != 2 || !names["AMAZON MKTPLACE"] || !names["Amazon.com"] {
		t.Fatalf("merge = %+v", r.Merge)
	}
	if !strings.Contains(r.MergeCommand, "payees merge --target-id pay-amazon --merge-ids ") || !strings.Contains(r.MergeCommand, "pay-amazon2") {
		t.Fatalf("merge_command = %q", r.MergeCommand)
	}
	// Negative: unrelated payees are never clustered.
	for _, other := range []string{"Kroger", "Chipotle", "Netflix", "Oak Street Apartments"} {
		if names[other] || r.TargetName == other {
			t.Fatalf("%s should not be clustered", other)
		}
	}
}

func TestAuditPayeesRejectsBadSimilarity(t *testing.T) {
	s3Fixture(t)
	for _, v := range []string{"2", "-0.1", "abc"} {
		_, _, err := s3Run(t, "audit", "payees", "--min-similarity", v)
		if processExitCode(err) != 2 {
			t.Fatalf("--min-similarity %s: exit %d (%v), want 2", v, processExitCode(err), err)
		}
	}
}

func TestAuditPayeesNoMirrorEmpty(t *testing.T) {
	s3NoMirror(t)
	out, _, err := s3Run(t, "audit", "payees")
	if err != nil || strings.TrimSpace(out) != "[]" {
		t.Fatalf("no mirror: out=%q err=%v, want []", out, err)
	}
}
