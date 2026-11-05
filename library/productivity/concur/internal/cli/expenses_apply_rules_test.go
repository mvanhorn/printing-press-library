// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// TestExpensesApplyRules_LiveShapeMismatches covers F1 and F2:
// verifies apply-rules GETs /reports/{id}/expenses and successfully
// parses a real nested live API response shape.
func TestExpensesApplyRules_LiveShapeMismatches(t *testing.T) {
	configPath := writeTestExpenseRulesConfig(t)

	// Set up mock HTTP server
	apiCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls++
		if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/reports/some-report-id/expenses") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{
					"expenseId": "exp-real-1",
					"expenseType": {
						"id": "CELPH",
						"name": "Mobile/Cellular Phone",
						"code": "CELPH",
						"isDeleted": false,
						"listItemId": "item-1"
					},
					"transactionDate": "2026-09-01",
					"transactionAmount": {
						"value": 65.00,
						"currencyCode": "USD"
					},
					"vendor": {
						"id": "vendor-1",
						"name": null,
						"description": "on-call cell phone"
					},
					"paymentType": {
						"id": "PAY-1",
						"name": "Corporate Card",
						"code": "CORP"
					},
					"businessPurpose": "",
					"hasExceptions": false,
					"hasBlockingExceptions": false
				}
			]`))
			return
		}
		if r.Method == "PATCH" && strings.Contains(r.URL.Path, "/expenses/exp-real-1") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("CONCUR_BASE_URL", server.URL)
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")

	cmd := RootCmd()
	cmd.SetArgs([]string{
		"expenses", "apply-rules", "some-report-id",
		"--user-id", "test-user-id",
		"--config", configPath,
		"--yes",
		"--json",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(os.Stderr)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if apiCalls != 2 {
		t.Errorf("expected 2 API calls (1 GET, 1 PATCH), got %d", apiCalls)
	}

	var envelope map[string]any
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to unmarshal output JSON: %v", err)
	}

	changes, ok := envelope["changes"].([]any)
	if !ok || len(changes) == 0 {
		t.Fatalf("expected at least one rule change, got envelope: %+v", envelope)
	}

	foundSetPurpose := false
	foundExceedsCap := false
	for _, c := range changes {
		m, ok := c.(map[string]any)
		if !ok {
			continue
		}
		changeType := m["change_type"].(string)
		if changeType == "set_business_purpose" {
			foundSetPurpose = true
		}
		if changeType == "exceeds_cap_needs_manual_split" {
			foundExceedsCap = true
		}
	}

	if !foundSetPurpose {
		t.Error("expected a business purpose change to be applied/logged")
	}
	if !foundExceedsCap {
		t.Error("expected a cap overage warning to be logged")
	}
}

// TestSyncExpenses_ReturnsSpecificError covers F3:
// asserts a helpful, specific error when syncing expenses.
func TestSyncExpenses_ReturnsSpecificError(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"sync", "--resources", "expenses"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected sync --resources expenses to fail, got nil")
	}
	expectedMsg := "must be specified for expenses sync"
	if !strings.Contains(out.String(), expectedMsg) {
		t.Fatalf("expected output to contain %q, but got:\n%s", expectedMsg, out.String())
	}
}
