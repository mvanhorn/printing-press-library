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
)

// TestNovelVerifyHelpWires smoke-tests that the verify command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelVerifyHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"verify", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("verify --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "verify"} {
		if !strings.Contains(help, want) {
			t.Fatalf("verify --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestVerifyAllocationEmptyMirrorKeepsReportSchema pins the
// one-top-level-type rule: the missing-mirror path must emit the same
// {"summary":{...},"funds":[]} object the success path emits, not a bare [],
// or `jq '.summary.failed'` hits a type error exactly when the mirror is
// absent. It also pins the partial-run denominators into the empty summary.
func TestVerifyAllocationEmptyMirrorKeepsReportSchema(t *testing.T) {
	cmd := RootCmd()
	missingDB := filepath.Join(t.TempDir(), "no-such-mirror.db")
	cmd.SetArgs([]string{
		"verify", "allocation", "--json", "--no-learn",
		"--home", t.TempDir(),
		"--month", "2026-07",
		"--db", missingDB,
	})
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("verify allocation against a missing mirror error = %v (stderr: %s)", err, errOut.String())
	}
	var report struct {
		Summary map[string]any `json:"summary"`
		Funds   []any          `json:"funds"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("missing-mirror output is not the verifyAllocationReport object: %v\noutput:\n%s", err, out.String())
	}
	if report.Summary == nil {
		t.Fatalf("missing-mirror output dropped .summary:\n%s", out.String())
	}
	if report.Funds == nil {
		t.Fatalf("missing-mirror output dropped .funds:\n%s", out.String())
	}
	if len(report.Funds) != 0 {
		t.Fatalf("missing-mirror output invented %d fund row(s):\n%s", len(report.Funds), out.String())
	}
	for _, key := range []string{"month", "source", "funds", "funds_discovered", "checked", "failed", "truncated", "failing_funds"} {
		if _, ok := report.Summary[key]; !ok {
			t.Fatalf("missing-mirror summary omits %q:\n%s", key, out.String())
		}
	}
}
