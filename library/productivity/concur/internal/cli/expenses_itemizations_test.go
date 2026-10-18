// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestExpensesItemizations_BareArrayResponse(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			{
				"expenseId": "550e8400-e29b-41d4-a716-446655440001",
				"parentExpenseId": "550e8400-e29b-41d4-a716-446655440000",
				"expenseType": {"id": "CELPH", "code": "OTHER", "name": "Mobile/Cellular Phone"},
				"vendor": {"id": null, "name": null, "description": "Vendor Description"},
				"businessPurpose": "business-purpose-desc",
				"transactionAmount": {"value": 25.50, "currencyCode": "USD"},
				"transactionDate": "2026-10-17"
			}
		]`))
	}))
	defer server.Close()

	t.Setenv("CONCUR_BASE_URL", server.URL)
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")

	cmd := RootCmd()
	cmd.SetArgs([]string{
		"expenses", "itemizations", "550e8400-e29b-41d4-a716-446655440000",
		"--user-id", "test-user-id",
		"--report-id", "test-report-id",
		"--json",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(os.Stderr)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantPath := "/expensereports/v4/users/test-user-id/context/TRAVELER/reports/test-report-id/expenses/550e8400-e29b-41d4-a716-446655440000/itemizations"
	if gotPath != wantPath {
		t.Errorf("request path = %q, want %q", gotPath, wantPath)
	}

	var envelope map[string]any
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to unmarshal output JSON: %v", err)
	}

	results, ok := envelope["results"].([]any)
	if !ok {
		t.Fatalf("expected a results array in envelope, got %+v", envelope)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 expense, got %d", len(results))
	}

	expense, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected result shape: %+v", results[0])
	}
	if expense["expenseId"] != "550e8400-e29b-41d4-a716-446655440001" {
		t.Errorf("expenseId = %v, want %q", expense["expenseId"], "550e8400-e29b-41d4-a716-446655440001")
	}
	if expense["parentExpenseId"] != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("parentExpenseId = %v, want %q", expense["parentExpenseId"], "550e8400-e29b-41d4-a716-446655440000")
	}
}

func TestExpensesItemizations_MissingArgsAndFlags(t *testing.T) {
	// Missing positional arg
	cmd := RootCmd()
	cmd.SetArgs([]string{
		"expenses", "itemizations",
		"--user-id", "test-user-id",
		"--report-id", "test-report-id",
		"--json",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error with missing positional arg, got nil")
	}

	// Missing report-id
	cmd = RootCmd()
	cmd.SetArgs([]string{
		"expenses", "itemizations", "550e8400-e29b-41d4-a716-446655440000",
		"--user-id", "test-user-id",
		"--json",
	})
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error with missing --report-id, got nil")
	}

	// Invalid context-type
	cmd = RootCmd()
	cmd.SetArgs([]string{
		"expenses", "itemizations", "550e8400-e29b-41d4-a716-446655440000",
		"--user-id", "test-user-id",
		"--report-id", "test-report-id",
		"--context-type", "INVALID_CONTEXT_TYPE",
		"--json",
	})
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error with invalid --context-type, got nil")
	}
}
