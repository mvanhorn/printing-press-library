// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestExpenseRulesConfig writes a minimal non-empty rules config to a
// temp file so tests can reach the confirmation gate (an empty/missing
// config is a legitimate no-op that short-circuits before it).
func writeTestExpenseRulesConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "expense_types.json")
	content := `{"Mobile/Cellular Phone": {"business_purpose": "on-call cell phone", "reimbursement_cap": 50.00}}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing test rules config: %v", err)
	}
	return path
}

// TestExpensesApplyRulesHelpWires smoke-tests that the expenses apply-rules
// command resolves at runtime and renders useful --help output.
func TestExpensesApplyRulesHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"expenses", "apply-rules", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expenses apply-rules --help error = %v (command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "apply-rules", "--config"} {
		if !strings.Contains(help, want) {
			t.Fatalf("expenses apply-rules --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestExpensesApplyRulesRequiresConfirmation verifies the --yes confirmation
// gate actually blocks a real (non-dry-run) mutating invocation, matching the
// pattern used by the other mutating novel commands.
func TestExpensesApplyRulesRequiresConfirmation(t *testing.T) {
	configPath := writeTestExpenseRulesConfig(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"expenses", "apply-rules", "some-report-id", "--user-id", "test-user", "--config", configPath})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error when --yes is not passed for a real (non-dry-run) mutation, got nil")
	}
	if !strings.Contains(err.Error(), "confirmation required") {
		t.Fatalf("expected a confirmation-required error, got: %v", err)
	}
}

// TestExpensesApplyRulesDryRunSkipsConfirmation verifies --dry-run does not
// require --yes (dry-run never mutates, so confirmation would be noise).
func TestExpensesApplyRulesDryRunSkipsConfirmation(t *testing.T) {
	configPath := writeTestExpenseRulesConfig(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"expenses", "apply-rules", "some-report-id", "--user-id", "test-user", "--dry-run", "--config", configPath})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err != nil && strings.Contains(err.Error(), "confirmation required") {
		t.Fatalf("--dry-run should not require --yes confirmation, got: %v", err)
	}
}

// TestExpensesApplyRulesEmptyConfigIsNoOp verifies that a missing/empty rules
// config is treated as "nothing to apply" rather than an error, matching the
// prior-art semantics this command was ported from.
func TestExpensesApplyRulesEmptyConfigIsNoOp(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{
		"expenses", "apply-rules", "some-report-id",
		"--user-id", "test-user", "--yes", "--json",
		"--config", "/nonexistent/path/expense_types.json",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected a missing config file to be a no-op, got error: %v", err)
	}
	if !strings.Contains(out.String(), `"changes":null`) && !strings.Contains(out.String(), `"changes": null`) {
		t.Fatalf("expected empty changes for a missing config, got:\n%s", out.String())
	}
}
