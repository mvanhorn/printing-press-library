// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/concur/internal/client"
)

// TestIsExpensesCreate404DefectError covers the trigger condition for the
// browser fallback: it must fire ONLY on the exact live-confirmed
// signature (HTTP 404 + "No static resource" + a path containing
// "/expenses"), never on an unrelated 404 or a different status code.
func TestIsExpensesCreate404DefectError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "exact live signature matches",
			err: &client.APIError{
				StatusCode: 404,
				Body:       `{"errorMessage":"No static resource /expensereports/v4/users/u1/context/TRAVELER/reports/r1/expenses."}`,
			},
			want: true,
		},
		{
			name: "matches on the PATCH (update) path too -- confirmed live the defect is not create-only",
			err: &client.APIError{
				StatusCode: 404,
				Body:       `{"errorMessage":"No static resource /expensereports/v4/users/u1/context/TRAVELER/reports/r1/expenses/exp1."}`,
			},
			want: true,
		},
		{
			name: "unrelated 404 (bad report id) does not match",
			err: &client.APIError{
				StatusCode: 404,
				Body:       `{"errorMessage":"report not found"}`,
			},
			want: false,
		},
		{
			name: "No static resource on an unrelated path does not match",
			err: &client.APIError{
				StatusCode: 404,
				Body:       `{"errorMessage":"No static resource /some/other/path."}`,
			},
			want: false,
		},
		{
			name: "400 with the same body text does not match (wrong status code)",
			err: &client.APIError{
				StatusCode: 400,
				Body:       `{"errorMessage":"No static resource /expenses."}`,
			},
			want: false,
		},
		{
			name: "nil error does not match",
			err:  nil,
			want: false,
		},
		{
			name: "non-APIError does not match",
			err:  errors.New("boom"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isExpensesCreate404DefectError(tt.err); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// TestExtractExpenseFallbackParams covers reading the fallback's needed
// fields from an already-constructed body map, for BOTH input paths this
// command supports: the flag-driven object shape this file builds, and an
// arbitrary --stdin caller's JSON (which the fallback must also work for
// now that the !stdinBody exclusion has been removed -- a --stdin caller's
// flag variables are always empty, so extraction has to read the body
// itself, not the flags).
func TestExtractExpenseFallbackParams(t *testing.T) {
	t.Run("flag-driven object shape", func(t *testing.T) {
		body := map[string]any{
			"expenseType":       map[string]any{"code": "01000"},
			"paymentType":       map[string]any{"id": "CASH"},
			"transactionDate":   "2026-09-15",
			"transactionAmount": 50.0,
			"vendor":            map[string]any{"name": "F45 Training"},
			"businessPurpose":   "gym",
		}
		typeCode, paymentType, txDate, amt, vendor, purpose := extractExpenseFallbackParams(body)
		if typeCode != "01000" || paymentType != "CASH" || txDate != "2026-09-15" || amt != 50.0 || vendor != "F45 Training" || purpose != "gym" {
			t.Errorf("got (%q, %q, %q, %v, %q, %q)", typeCode, paymentType, txDate, amt, vendor, purpose)
		}
	})

	t.Run("stdin caller using description instead of name for vendor", func(t *testing.T) {
		body := map[string]any{
			"expenseType":       map[string]any{"code": "CELPH"},
			"paymentType":       map[string]any{"id": "CASH"},
			"transactionDate":   "2026-09-15",
			"transactionAmount": 50.0,
			"vendor":            map[string]any{"description": "on-call cell phone"},
		}
		_, _, _, _, vendor, _ := extractExpenseFallbackParams(body)
		if vendor != "on-call cell phone" {
			t.Errorf("expected vendor extracted from the description sub-field, got %q", vendor)
		}
	})

	t.Run("non-map body returns zero values instead of panicking", func(t *testing.T) {
		typeCode, paymentType, txDate, amt, vendor, purpose := extractExpenseFallbackParams("not a map")
		if typeCode != "" || paymentType != "" || txDate != "" || amt != 0 || vendor != "" || purpose != "" {
			t.Errorf("expected all zero values for a non-map body, got (%q, %q, %q, %v, %q, %q)", typeCode, paymentType, txDate, amt, vendor, purpose)
		}
	})
}

// TestDiffNewExpense covers the before/after expense-list diff used to
// identify what the browser fallback's Save Expense click created, since
// the browser itself never surfaces a usable expense ID. Every candidate
// carries a nested transactionAmount/expenseType (the shape PR #1940's F2
// finding confirmed live for this same response), so these also exercise
// expenseAmountAndType's nested-shape branch, not just flat.
func TestDiffNewExpense(t *testing.T) {
	before := []json.RawMessage{
		json.RawMessage(`{"expenseId":"exp-1","transactionAmount":{"value":10},"expenseType":{"code":"OTHER"}}`),
	}
	beforeIDs := expenseIDSet(before)

	t.Run("exactly one new matching expense is identified", func(t *testing.T) {
		after := []json.RawMessage{
			json.RawMessage(`{"expenseId":"exp-1","transactionAmount":{"value":10},"expenseType":{"code":"OTHER"}}`),
			json.RawMessage(`{"expenseId":"exp-2","transactionAmount":{"value":50},"expenseType":{"code":"CELPH"}}`),
		}
		got, err := diffNewExpense(before, after, beforeIDs, 50, "CELPH")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var parsed struct {
			ExpenseID string `json:"expenseId"`
		}
		if err := json.Unmarshal(got, &parsed); err != nil {
			t.Fatalf("unexpected unmarshal error: %v", err)
		}
		if parsed.ExpenseID != "exp-2" {
			t.Errorf("got expense %q, want exp-2", parsed.ExpenseID)
		}
	})

	t.Run("zero new expenses errors instead of guessing", func(t *testing.T) {
		after := []json.RawMessage{
			json.RawMessage(`{"expenseId":"exp-1","transactionAmount":{"value":10},"expenseType":{"code":"OTHER"}}`),
		}
		if _, err := diffNewExpense(before, after, beforeIDs, 50, "CELPH"); err == nil {
			t.Error("expected an error when no new expense appeared, got nil")
		}
	})

	// Covers the Greptile review finding "Concurrent Expense
	// Misattribution": a second actor (shared manager/processor/proxy
	// context) adds an unrelated expense concurrently with this call's own
	// Save. Both are "new" by ID, but only one matches what THIS call
	// submitted -- the other actor's expense must never be misattributed
	// as this call's own.
	t.Run("concurrent unrelated new expense is not misattributed", func(t *testing.T) {
		after := []json.RawMessage{
			json.RawMessage(`{"expenseId":"exp-1","transactionAmount":{"value":10},"expenseType":{"code":"OTHER"}}`),
			json.RawMessage(`{"expenseId":"exp-other-actor","transactionAmount":{"value":9999},"expenseType":{"code":"AIRFR"}}`),
			json.RawMessage(`{"expenseId":"exp-2","transactionAmount":{"value":50},"expenseType":{"code":"CELPH"}}`),
		}
		got, err := diffNewExpense(before, after, beforeIDs, 50, "CELPH")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var parsed struct {
			ExpenseID string `json:"expenseId"`
		}
		if err := json.Unmarshal(got, &parsed); err != nil {
			t.Fatalf("unexpected unmarshal error: %v", err)
		}
		if parsed.ExpenseID != "exp-2" {
			t.Errorf("got expense %q, want exp-2 (not the concurrent unrelated expense)", parsed.ExpenseID)
		}
	})

	// When NEITHER new expense matches what was submitted, this must
	// error rather than pick one -- e.g. this call's own Save silently
	// failed while an unrelated concurrent expense happened to appear.
	t.Run("no matching candidate errors instead of guessing", func(t *testing.T) {
		after := []json.RawMessage{
			json.RawMessage(`{"expenseId":"exp-1","transactionAmount":{"value":10},"expenseType":{"code":"OTHER"}}`),
			json.RawMessage(`{"expenseId":"exp-other-actor","transactionAmount":{"value":9999},"expenseType":{"code":"AIRFR"}}`),
		}
		if _, err := diffNewExpense(before, after, beforeIDs, 50, "CELPH"); err == nil {
			t.Error("expected an error when no new expense matches the submitted amount/type, got nil")
		}
	})

	t.Run("multiple matching new expenses errors instead of guessing which one", func(t *testing.T) {
		after := []json.RawMessage{
			json.RawMessage(`{"expenseId":"exp-1","transactionAmount":{"value":10},"expenseType":{"code":"OTHER"}}`),
			json.RawMessage(`{"expenseId":"exp-2","transactionAmount":{"value":50},"expenseType":{"code":"CELPH"}}`),
			json.RawMessage(`{"expenseId":"exp-3","transactionAmount":{"value":50},"expenseType":{"code":"CELPH"}}`),
		}
		if _, err := diffNewExpense(before, after, beforeIDs, 50, "CELPH"); err == nil {
			t.Error("expected an error when multiple matching new expenses appeared, got nil")
		}
	})

	t.Run("candidate missing comparable fields is excluded, not auto-matched", func(t *testing.T) {
		after := []json.RawMessage{
			json.RawMessage(`{"expenseId":"exp-1","transactionAmount":{"value":10},"expenseType":{"code":"OTHER"}}`),
			json.RawMessage(`{"expenseId":"exp-2"}`), // no transactionAmount/expenseType at all
		}
		if _, err := diffNewExpense(before, after, beforeIDs, 50, "CELPH"); err == nil {
			t.Error("expected an error -- a candidate missing comparable fields must not be treated as an automatic match")
		}
	})
}

// TestExpenseAmountAndType covers both response shapes this session
// found evidence for: a flat number/string (used in earlier ad hoc mock
// bodies in this file) and the nested {"value":...}/{"code":...} shape PR
// #1940's F2 finding confirmed live for this same response. Defensive
// tolerance of either shape matters because this session did not
// independently re-verify these two specific fields' live shape.
func TestExpenseAmountAndType(t *testing.T) {
	t.Run("flat shape", func(t *testing.T) {
		amount, hasAmount, typeCode, hasType := expenseAmountAndType(json.RawMessage(`{"transactionAmount":50,"expenseType":"CELPH"}`))
		if !hasAmount || amount != 50 || !hasType || typeCode != "CELPH" {
			t.Errorf("got (%v, %v, %q, %v)", amount, hasAmount, typeCode, hasType)
		}
	})

	t.Run("nested shape", func(t *testing.T) {
		amount, hasAmount, typeCode, hasType := expenseAmountAndType(json.RawMessage(`{"transactionAmount":{"value":50,"currencyCode":"USD"},"expenseType":{"code":"CELPH","name":"Mobile/Cellular Phone"}}`))
		if !hasAmount || amount != 50 || !hasType || typeCode != "CELPH" {
			t.Errorf("got (%v, %v, %q, %v)", amount, hasAmount, typeCode, hasType)
		}
	})

	t.Run("missing fields report false, not zero-value false positives", func(t *testing.T) {
		_, hasAmount, _, hasType := expenseAmountAndType(json.RawMessage(`{}`))
		if hasAmount || hasType {
			t.Errorf("expected both hasAmount and hasType false for an empty object, got hasAmount=%v hasType=%v", hasAmount, hasType)
		}
	})
}

// TestExpensesCreatePartialSuccessError mirrors
// TestReportsCreatePartialSuccessError's coverage for the analogous
// duplicate-risk guard on the expenses-create browser fallback.
func TestExpensesCreatePartialSuccessError(t *testing.T) {
	cause := errors.New("boom")
	e := &expensesCreatePartialSuccessError{reportId: "RPT123", cause: cause}

	if !strings.Contains(e.Error(), "RPT123") {
		t.Errorf("expected error message to include the report ID, got: %s", e.Error())
	}
	if !strings.Contains(strings.ToLower(e.Error()), "duplicate") {
		t.Errorf("expected error message to warn about duplicate creation, got: %s", e.Error())
	}
	if !errors.Is(e, cause) {
		t.Error("expected Unwrap() to expose the underlying cause via errors.Is")
	}
}

// TestExpensesCreate_BrowserFallback exercises the full command path with a
// mocked agent-browser binary (a bash script on $PATH, matching the pattern
// established in reports_create_fallback_test.go) and a mocked Concur API
// server, so no real browser or live tenant is touched.
func TestExpensesCreate_BrowserFallback(t *testing.T) {
	tmpDir := t.TempDir()
	mockBinPath := filepath.Join(tmpDir, "agent-browser")
	stateFile := filepath.Join(tmpDir, "mock_state")

	mockScript := `#!/bin/bash
arg1="$1"
arg2="$2"

if [ "$arg1" = "--cdp" ]; then
	echo '{"success": false}'
	exit 0
fi

if [ "$arg1" = "get" ] && [ "$arg2" = "url" ]; then
	echo "https://us2.concursolutions.com/nui/expense/reports/mock-report/expenses/new?expenseTypeId=CELPH"
	exit 0
fi

if [ "$arg1" = "snapshot" ]; then
	echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{"e1":{"name":"Amount","role":"textbox"},"e2":{"name":"Vendor Description","role":"textbox"},"e3":{"name":"Save Expense","role":"button"},"e4":{"name":"Business Purpose","role":"textbox"}}}}'
	exit 0
fi

if [ "$arg1" = "click" ] && [ "$arg2" = "@e3" ]; then
	touch "$MOCK_STATE_FILE"
	exit 0
fi

exit 0
`
	if err := os.WriteFile(mockBinPath, []byte(mockScript), 0o755); err != nil {
		t.Fatalf("writing mock binary: %v", err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir+string(filepath.ListSeparator)+oldPath)
	t.Setenv("MOCK_STATE_FILE", stateFile)

	t.Run("404Defect_TriggersFallback", func(t *testing.T) {
		_ = os.Remove(stateFile)

		apiCalls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiCalls++
			w.Header().Set("Content-Type", "application/json")

			switch {
			case r.Method == "POST" && strings.Contains(r.URL.Path, "/expenses"):
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"errorMessage":"No static resource /expensereports/v4/users/test-user-id/context/TRAVELER/reports/mock-report/expenses."}`))
			case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/expenses"):
				if _, err := os.Stat(stateFile); err == nil {
					// After the mocked Save Expense click: one new expense.
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`[{"expenseId":"exp-new-1","transactionAmount":{"value":50},"expenseType":{"code":"CELPH"}}]`))
				} else {
					// Before the browser fallback: empty report.
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`[]`))
				}
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		t.Setenv("CONCUR_BASE_URL", server.URL)
		t.Setenv("CONCUR_UI_BASE_URL", "https://us2.concursolutions.com")
		t.Setenv("PRINTING_PRESS_VERIFY", "1")
		t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")

		cmd := RootCmd()
		cmd.SetArgs([]string{
			"expenses", "create",
			"--user-id", "test-user-id",
			"--report-id", "mock-report",
			"--type", "CELPH",
			"--date", "2026-09-15",
			"--amount", "50",
			"--payment-type", "CASH",
			"--vendor", "on-call cell phone",
			"--json",
		})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(stateFile); os.IsNotExist(err) {
			t.Error("expected browser fallback to be driven (Save Expense clicked), but state file does not exist")
		}

		var envelope map[string]any
		if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
			t.Fatalf("failed to unmarshal output JSON: %v", err)
		}
		if success, ok := envelope["success"].(bool); !ok || !success {
			t.Errorf("expected envelope success: true, got %+v", envelope)
		}
		// --json (not --agent) puts the payload directly under "data", not
		// nested under "results".
		data, ok := envelope["data"].(map[string]any)
		if !ok {
			t.Fatalf("expected data field in envelope, got %+v", envelope)
		}
		if data["expenseId"] != "exp-new-1" {
			t.Errorf("expected the diffed new expense in the response, got %+v", data)
		}
	})

	t.Run("NonMatching404_DoesNotTriggerFallback", func(t *testing.T) {
		_ = os.Remove(stateFile)

		apiCalls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiCalls++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorMessage":"report not found"}`))
		}))
		defer server.Close()

		t.Setenv("CONCUR_BASE_URL", server.URL)
		t.Setenv("CONCUR_UI_BASE_URL", "https://us2.concursolutions.com")
		t.Setenv("PRINTING_PRESS_VERIFY", "1")
		t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")

		cmd := RootCmd()
		cmd.SetArgs([]string{
			"expenses", "create",
			"--user-id", "test-user-id",
			"--report-id", "mock-report",
			"--type", "CELPH",
			"--date", "2026-09-15",
			"--amount", "50",
			"--json",
		})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)

		if err := cmd.Execute(); err == nil {
			t.Fatal("expected an error, got nil")
		}

		if apiCalls != 1 {
			t.Errorf("expected exactly 1 API call (no fallback GET calls), got %d", apiCalls)
		}
		if _, err := os.Stat(stateFile); !os.IsNotExist(err) {
			t.Error("expected browser fallback NOT to be driven, but state file exists")
		}
	})

	// StdinBody_ALSOTriggersFallback covers the fix removing this fallback's
	// former !stdinBody exclusion: a --stdin caller's flag variables are
	// always empty, so the OLD code could never drive the fallback for
	// that input path at all (the confirmed-live 404 defect would just
	// surface as a plain error, no matter which input path hit it).
	// Redirects the real os.Stdin via a pipe since expenses_create.go
	// reads directly from os.Stdin, not through cobra's InOrStdin().
	t.Run("StdinBody_ALSOTriggersFallback", func(t *testing.T) {
		_ = os.Remove(stateFile)

		apiCalls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiCalls++
			w.Header().Set("Content-Type", "application/json")
			switch {
			case r.Method == "POST" && strings.Contains(r.URL.Path, "/expenses"):
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"errorMessage":"No static resource /expensereports/v4/users/test-user-id/context/TRAVELER/reports/mock-report/expenses."}`))
			case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/expenses"):
				if _, err := os.Stat(stateFile); err == nil {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`[{"expenseId":"exp-new-stdin-1","transactionAmount":{"value":50},"expenseType":{"code":"01000"}}]`))
				} else {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`[]`))
				}
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		t.Setenv("CONCUR_BASE_URL", server.URL)
		t.Setenv("CONCUR_UI_BASE_URL", "https://us2.concursolutions.com")
		t.Setenv("PRINTING_PRESS_VERIFY", "1")
		t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")

		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("creating pipe: %v", err)
		}
		origStdin := os.Stdin
		os.Stdin = r
		t.Cleanup(func() { os.Stdin = origStdin })

		stdinBody := `{"expenseType":{"code":"01000"},"transactionAmount":50,"transactionDate":"2026-09-15","paymentType":{"id":"CASH"},"vendor":{"name":"F45 Training"},"businessPurpose":"gym"}`
		go func() {
			_, _ = w.Write([]byte(stdinBody))
			_ = w.Close()
		}()

		cmd := RootCmd()
		cmd.SetArgs([]string{
			"expenses", "create",
			"--user-id", "test-user-id",
			"--report-id", "mock-report",
			"--stdin",
			"--json",
		})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(stateFile); os.IsNotExist(err) {
			t.Error("expected browser fallback to be driven for a --stdin body too, but state file does not exist")
		}

		var envelope map[string]any
		if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
			t.Fatalf("failed to unmarshal output JSON: %v", err)
		}
		data, ok := envelope["data"].(map[string]any)
		if !ok || data["expenseId"] != "exp-new-stdin-1" {
			t.Errorf("expected the diffed new expense in the response for the --stdin path, got %+v", envelope)
		}
	})

	t.Run("SuccessfulHTTP_NeverTriggersFallback", func(t *testing.T) {
		_ = os.Remove(stateFile)

		apiCalls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiCalls++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"expenseId":"exp-direct-1"}`))
		}))
		defer server.Close()

		t.Setenv("CONCUR_BASE_URL", server.URL)
		t.Setenv("CONCUR_UI_BASE_URL", "https://us2.concursolutions.com")
		t.Setenv("PRINTING_PRESS_VERIFY", "1")
		t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")

		cmd := RootCmd()
		cmd.SetArgs([]string{
			"expenses", "create",
			"--user-id", "test-user-id",
			"--report-id", "mock-report",
			"--type", "CELPH",
			"--date", "2026-09-15",
			"--amount", "50",
			"--json",
		})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if apiCalls != 1 {
			t.Errorf("expected exactly 1 API call, got %d", apiCalls)
		}
		if _, err := os.Stat(stateFile); !os.IsNotExist(err) {
			t.Error("expected browser fallback NOT to be driven, but state file exists")
		}
	})
}

// TestExpensesCreate_BrowserFallback_RejectsUnhonorableFields covers the
// Greptile review finding "Fallback Discards Requested Fields": a
// historical --date or non-Cash --payment-type used to only warn, then
// save with Concur's form defaults (today, Cash) anyway and report
// success -- creating an expense whose required date or requested payment
// type silently didn't match what was asked for. Both are now rejected
// BEFORE the browser is even opened (both checks are static), verified
// here by asserting zero agent-browser invocations occurred.
func TestExpensesCreate_BrowserFallback_RejectsUnhonorableFields(t *testing.T) {
	tmpDir := t.TempDir()
	mockBinPath := filepath.Join(tmpDir, "agent-browser")
	callLog := filepath.Join(tmpDir, "calls.log")

	// Any invocation at all is a failure for this test -- these checks
	// must reject before ever shelling out to agent-browser.
	mockScript := "#!/bin/bash\necho \"$@\" >> " + callLog + "\necho '{\"success\":false}'\nexit 0\n"
	if err := os.WriteFile(mockBinPath, []byte(mockScript), 0o755); err != nil {
		t.Fatalf("writing mock binary: %v", err)
	}
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir+string(filepath.ListSeparator)+oldPath)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" && strings.Contains(r.URL.Path, "/expenses") {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorMessage":"No static resource /expensereports/v4/users/test-user-id/context/TRAVELER/reports/mock-report/expenses."}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	t.Setenv("CONCUR_BASE_URL", server.URL)
	t.Setenv("CONCUR_UI_BASE_URL", "https://us2.concursolutions.com")
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")

	tests := []struct {
		name       string
		extraArgs  []string
		wantErrSub string
	}{
		{
			name:       "historical date is rejected",
			extraArgs:  []string{"--date", "2020-01-01"},
			wantErrSub: "cannot honor --date",
		},
		{
			name:       "non-Cash payment type is rejected",
			extraArgs:  []string{"--date", time.Now().Format("2006-01-02"), "--payment-type", "COMP"},
			wantErrSub: "cannot honor --payment-type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Remove(callLog)

			args := []string{
				"expenses", "create",
				"--user-id", "test-user-id",
				"--report-id", "mock-report",
				"--type", "CELPH",
				"--amount", "50",
				"--json",
			}
			args = append(args, tt.extraArgs...)

			cmd := RootCmd()
			cmd.SetArgs(args)

			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(io.Discard)

			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected an error rejecting the unhonorable field, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErrSub) {
				t.Errorf("expected error containing %q, got: %v", tt.wantErrSub, err)
			}
			if _, statErr := os.Stat(callLog); !os.IsNotExist(statErr) {
				t.Error("expected zero agent-browser invocations (rejected before opening the browser), but the mock was called")
			}
		})
	}
}

// TestExpensesCreate_BrowserFallback_AbortsWhenFieldNotFillable covers the
// Greptile review finding "Business Purpose Gets Dropped" (and the
// identical latent bug in Vendor Description, fixed for consistency): if
// the browser fallback can't find a field the caller explicitly requested,
// it must abort BEFORE the irreversible Save Expense click, not warn and
// save without it. Uses a mock snapshot missing the Business Purpose ref
// entirely (unlike the shared mock in TestExpensesCreate_BrowserFallback,
// which includes it) to force the not-found path deterministically.
func TestExpensesCreate_BrowserFallback_AbortsWhenFieldNotFillable(t *testing.T) {
	tmpDir := t.TempDir()
	mockBinPath := filepath.Join(tmpDir, "agent-browser")
	saveClickedFile := filepath.Join(tmpDir, "save_clicked")

	mockScript := `#!/bin/bash
arg1="$1"
arg2="$2"

if [ "$arg1" = "--cdp" ]; then
	echo '{"success": false}'
	exit 0
fi

if [ "$arg1" = "get" ] && [ "$arg2" = "url" ]; then
	echo "https://us2.concursolutions.com/nui/expense/reports/mock-report/expenses/new?expenseTypeId=CELPH"
	exit 0
fi

if [ "$arg1" = "snapshot" ]; then
	echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{"e1":{"name":"Amount","role":"textbox"},"e2":{"name":"Save Expense","role":"button"}}}}'
	exit 0
fi

if [ "$arg1" = "click" ] && [ "$arg2" = "@e2" ]; then
	touch "` + saveClickedFile + `"
	exit 0
fi

exit 0
`
	if err := os.WriteFile(mockBinPath, []byte(mockScript), 0o755); err != nil {
		t.Fatalf("writing mock binary: %v", err)
	}
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir+string(filepath.ListSeparator)+oldPath)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" && strings.Contains(r.URL.Path, "/expenses") {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorMessage":"No static resource /expensereports/v4/users/test-user-id/context/TRAVELER/reports/mock-report/expenses."}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	t.Setenv("CONCUR_BASE_URL", server.URL)
	t.Setenv("CONCUR_UI_BASE_URL", "https://us2.concursolutions.com")
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")

	cmd := RootCmd()
	cmd.SetArgs([]string{
		"expenses", "create",
		"--user-id", "test-user-id",
		"--report-id", "mock-report",
		"--type", "CELPH",
		"--date", time.Now().Format("2006-01-02"),
		"--amount", "50",
		"--business-purpose", "gym",
		"--json",
	})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error when the Business Purpose field cannot be found, got nil")
	}
	if !strings.Contains(err.Error(), "Business Purpose") {
		t.Errorf("expected error naming the unfillable field, got: %v", err)
	}
	if _, statErr := os.Stat(saveClickedFile); !os.IsNotExist(statErr) {
		t.Error("expected Save Expense to NEVER be clicked when a requested field can't be filled, but it was")
	}
}
