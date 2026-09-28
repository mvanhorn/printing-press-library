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

// TestExpensesList_BareArrayResponse covers amend-2026-09-28 finding F2:
// before this command existed, there was no live way to enumerate an
// existing report's expenses -- "reports get" returns only the header,
// there is no other list command, and "sync --resources expenses" refuses
// outright. Confirmed live 2026-09-28 that GET on the same collection URL
// "expenses create" POSTs to returns a bare JSON array (not a paginated
// Spring HAL envelope like "reports list"), so this test asserts that
// specific unwrapped-array shape passes straight through to results.
func TestExpensesList_BareArrayResponse(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			{
				"expenseId": "expense-1",
				"expenseType": {"id": "CELPH", "code": "OTHER", "name": "Mobile/Cellular Phone"},
				"vendor": {"id": null, "name": null, "description": "T-Mobile"},
				"businessPurpose": "on-call cell phone",
				"transactionAmount": {"value": 50, "currencyCode": "USD"},
				"transactionDate": "2026-09-22",
				"paymentType": {"id": "CASH", "code": "CASH", "name": "Cash"},
				"receiptImageId": "receipt-abc"
			}
		]`))
	}))
	defer server.Close()

	t.Setenv("CONCUR_BASE_URL", server.URL)
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")

	cmd := RootCmd()
	cmd.SetArgs([]string{
		"expenses", "list",
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

	wantPath := "/expensereports/v4/users/test-user-id/context/TRAVELER/reports/test-report-id/expenses"
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
	if expense["expenseId"] != "expense-1" {
		t.Errorf("expenseId = %v, want %q", expense["expenseId"], "expense-1")
	}
	if expense["receiptImageId"] != "receipt-abc" {
		t.Errorf("receiptImageId = %v, want %q (this is the field a caller checks to confirm a receipt actually attached server-side)", expense["receiptImageId"], "receipt-abc")
	}
}

// TestExpensesList_RequiresReportID covers the missing-required-flag path --
// --report-id is what scopes the collection being listed, so a bare
// invocation without it must fail with a usage error (exit 2) rather than
// silently listing nothing or panicking on an empty path parameter.
func TestExpensesList_RequiresReportID(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{
		"expenses", "list",
		"--user-id", "test-user-id",
		"--json",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error when --report-id is not set, got nil")
	}
}
