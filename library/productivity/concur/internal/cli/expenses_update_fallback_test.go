// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExpensesUpdate_BrowserFallback exercises the full command path with a
// mocked agent-browser binary and a mocked Concur API server, matching the
// pattern established in expenses_create_fallback_test.go and
// reports_create_fallback_test.go.
//
// Added per Greptile review, "New mutation fallbacks lack tests" -- this
// fallback previously shipped (PR to fix the confirmed-live 404 defect)
// with zero command-level coverage of its own; only the sibling create
// fallback's mocks were updated to tolerate the changes this fallback's
// build introduced (the --session flag shift, the calendar picker).
func TestExpensesUpdate_BrowserFallback(t *testing.T) {
	tmpDir := t.TempDir()
	mockBinPath := filepath.Join(tmpDir, "agent-browser")
	stateFile := filepath.Join(tmpDir, "mock_state")

	mockScript := `#!/bin/bash
arg1="$1"
arg2="$2"

if [ "$arg1" = "--session" ]; then
	arg1="$3"
	arg2="$4"
	arg3="$5"
fi

if [ "$arg1" = "--cdp" ]; then
	echo '{"success": false}'
	exit 0
fi

if [ "$arg1" = "get" ] && [ "$arg2" = "url" ]; then
	if [ -f "$MOCK_STATE_FILE" ]; then
		echo "https://us2.concursolutions.com/nui/expense/reports/mock-report"
	else
		echo "https://us2.concursolutions.com/nui/expense/reports/mock-report/expenses/exp-1"
	fi
	exit 0
fi

if [ "$arg1" = "snapshot" ]; then
	echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{"e1":{"name":"Business Purpose","role":"textbox"},"e2":{"name":"Amount","role":"textbox"},"e3":{"name":"Save Expense","role":"button"}}}}'
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

	t.Run("404Defect_TriggersFallback_UpdatesPurposeAndAmount", func(t *testing.T) {
		_ = os.Remove(stateFile)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case r.Method == "PATCH" && strings.Contains(r.URL.Path, "/expenses/exp-1"):
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"errorMessage":"No static resource /expensereports/v4/users/test-user-id/context/TRAVELER/reports/mock-report/expenses/exp-1."}`))
			case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/expenses/exp-1"):
				if _, err := os.Stat(stateFile); err == nil {
					// After the mocked Save Expense click: new values.
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"expenseId":"exp-1","businessPurpose":"updated purpose","transactionAmount":{"value":99}}`))
				} else {
					// Before the browser fallback: original values.
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"expenseId":"exp-1","businessPurpose":"old purpose","transactionAmount":{"value":50}}`))
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
			"expenses", "update", "exp-1",
			"--user-id", "test-user-id",
			"--report-id", "mock-report",
			"--purpose", "updated purpose",
			"--amount", "99",
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
		data, ok := envelope["data"].(map[string]any)
		if !ok {
			t.Fatalf("expected data field in envelope, got %+v", envelope)
		}
		if data["businessPurpose"] != "updated purpose" {
			t.Errorf("expected updated businessPurpose in response, got %+v", data)
		}
	})

	t.Run("EmptyPurpose_RejectedBeforeOpeningBrowser", func(t *testing.T) {
		_ = os.Remove(stateFile)

		// Reuses the shared mock (agent-browser on PATH from the outer
		// scope, which touches stateFile only on "click @e3" -- Save
		// Expense). This test's whole point is that the browser is never
		// opened at all, so stateFile staying absent is a sufficient,
		// simpler proxy for "zero agent-browser invocations" than a
		// separate call-logging mock -- if this rejection ever regressed
		// into opening the browser, either that click would eventually
		// fire (stateFile appears) or the mocked snapshot/fields would
		// cause a different, equally test-failing error.
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == "PATCH" && strings.Contains(r.URL.Path, "/expenses/exp-1") {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"errorMessage":"No static resource /expensereports/v4/users/test-user-id/context/TRAVELER/reports/mock-report/expenses/exp-1."}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}))
		defer server.Close()

		t.Setenv("CONCUR_BASE_URL", server.URL)
		t.Setenv("CONCUR_UI_BASE_URL", "https://us2.concursolutions.com")
		t.Setenv("PRINTING_PRESS_VERIFY", "1")
		t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")

		cmd := RootCmd()
		cmd.SetArgs([]string{
			"expenses", "update", "exp-1",
			"--user-id", "test-user-id",
			"--report-id", "mock-report",
			"--purpose", "",
			"--json",
		})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected an error when --purpose \"\" requests clearing a required field, got nil")
		}
		if !strings.Contains(err.Error(), "cannot honor --purpose") {
			t.Errorf("expected error naming the rejected --purpose \"\" request, got: %v", err)
		}
		if _, statErr := os.Stat(stateFile); !os.IsNotExist(statErr) {
			t.Error("expected Save Expense to NEVER be clicked when --purpose \"\" is rejected before opening the browser, but it was")
		}
	})

	t.Run("SaveDidNotActuallyApply_ReportsFailureNotFalseSuccess", func(t *testing.T) {
		// PATCH(Greptile review, "Submission success is unverified"'s
		// sibling risk on the update path) -- reproduces the exact
		// live-caught false-success bug this fallback's own
		// verifyExpenseUpdateApplied exists to catch: the mocked click
		// handler below deliberately does NOT touch stateFile, so the
		// post-save GET keeps returning the ORIGINAL values even though
		// the click itself reported no error -- simulating the live
		// click-timing race where a validly-captured ref's click silently
		// failed to register. The command must fail, not report success
		// with stale data.
		_ = os.Remove(stateFile)

		noopClickScript := `#!/bin/bash
arg1="$1"
arg2="$2"

if [ "$arg1" = "--session" ]; then
	arg1="$3"
	arg2="$4"
fi

if [ "$arg1" = "--cdp" ]; then
	echo '{"success": false}'
	exit 0
fi

if [ "$arg1" = "get" ] && [ "$arg2" = "url" ]; then
	# Deliberately never reflects navigation: the save is mocked as never
	# actually registering, so clickWithNavigationRetry's own retries
	# exhaust and this function reports its own click failure. The
	# retries exhausting (rather than a false success slipping through)
	# is itself the property under test here.
	echo "https://us2.concursolutions.com/nui/expense/reports/mock-report/expenses/exp-1"
	exit 0
fi

if [ "$arg1" = "snapshot" ]; then
	echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{"e1":{"name":"Business Purpose","role":"textbox"},"e2":{"name":"Amount","role":"textbox"},"e3":{"name":"Save Expense","role":"button"}}}}'
	exit 0
fi

exit 0
`
		// Shadow the shared mock with this test's own agent-browser on
		// PATH, named identically so runAgentBrowser's exec.LookPath finds
		// this one instead.
		noopDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(noopDir, "agent-browser"), []byte(noopClickScript), 0o755); err != nil {
			t.Fatalf("writing shadowing mock binary: %v", err)
		}
		t.Setenv("PATH", noopDir+string(filepath.ListSeparator)+oldPath)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == "PATCH" && strings.Contains(r.URL.Path, "/expenses/exp-1") {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"errorMessage":"No static resource /expensereports/v4/users/test-user-id/context/TRAVELER/reports/mock-report/expenses/exp-1."}`))
				return
			}
			if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/expenses/exp-1") {
				// Always the ORIGINAL, unchanged value -- the save never
				// actually applied.
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"expenseId":"exp-1","businessPurpose":"old purpose","transactionAmount":{"value":50}}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}))
		defer server.Close()

		t.Setenv("CONCUR_BASE_URL", server.URL)
		t.Setenv("CONCUR_UI_BASE_URL", "https://us2.concursolutions.com")
		t.Setenv("PRINTING_PRESS_VERIFY", "1")
		t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")

		cmd := RootCmd()
		cmd.SetArgs([]string{
			"expenses", "update", "exp-1",
			"--user-id", "test-user-id",
			"--report-id", "mock-report",
			"--purpose", "updated purpose",
			"--amount", "99",
			"--json",
		})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected an error when the save never actually applied, got nil (false success)")
		}
	})
}
