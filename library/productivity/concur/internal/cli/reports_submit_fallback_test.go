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

// TestReportsSubmit_BrowserFallback exercises the full command path with a
// mocked agent-browser binary and a mocked Concur API server, matching the
// pattern established in expenses_create_fallback_test.go and
// reports_create_fallback_test.go.
//
// Added per Greptile review, "New mutation fallbacks lack tests" -- this
// fallback previously shipped with zero command-level coverage of its own.
// The mock deliberately models the exact live-confirmed shape that
// motivated waitForRefExcluding in the first place: while the confirmation
// dialog is open, BOTH the underlying page's "Submit Report" button and the
// dialog's own "Submit Report" button are present simultaneously with an
// identical name and role -- a regression back to plain findRef/waitForRef
// (non-deterministic under Go's map iteration) would sometimes re-click
// the page button instead of the dialog's real confirm action, and this
// mock's state machine can only reach "submitted" by clicking the SPECIFIC
// dialog ref, not just any button matching the name.
func TestReportsSubmit_BrowserFallback(t *testing.T) {
	tmpDir := t.TempDir()
	mockBinPath := filepath.Join(tmpDir, "agent-browser")
	stateFile := filepath.Join(tmpDir, "mock_submitted")
	dialogOpenFile := filepath.Join(tmpDir, "mock_dialog_open")

	mockScript := `#!/bin/bash
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
	echo "https://us2.concursolutions.com/nui/expense/reports/mock-report"
	exit 0
fi

if [ "$arg1" = "snapshot" ]; then
	if [ -f "$MOCK_STATE_FILE" ]; then
		# Submitted: dialog closed, nothing left to click.
		echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{}}}'
	elif [ -f "$MOCK_DIALOG_OPEN_FILE" ]; then
		# Confirmation dialog open: the page's original "Submit Report"
		# button (e1) is STILL present alongside the dialog's own,
		# separate "Submit Report" button (e2) and "Cancel" (e3) -- the
		# exact live-confirmed ambiguity waitForRefExcluding exists for.
		echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{"e1":{"name":"Submit Report","role":"button"},"e2":{"name":"Submit Report","role":"button"},"e3":{"name":"Cancel","role":"button"}}}}'
	else
		echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{"e1":{"name":"Submit Report","role":"button"}}}}'
	fi
	exit 0
fi

if [ "$arg1" = "click" ] && [ "$arg2" = "@e1" ]; then
	touch "$MOCK_DIALOG_OPEN_FILE"
	exit 0
fi

if [ "$arg1" = "click" ] && [ "$arg2" = "@e2" ]; then
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
	t.Setenv("MOCK_DIALOG_OPEN_FILE", dialogOpenFile)

	t.Run("405Defect_TriggersFallback_ConfirmsDialogAndSubmits", func(t *testing.T) {
		_ = os.Remove(stateFile)
		_ = os.Remove(dialogOpenFile)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/submit"):
				w.WriteHeader(http.StatusMethodNotAllowed)
				_, _ = w.Write([]byte(`{"errorMessage":"Request method 'POST' is not supported","path":"/expensereports/v4/users/test-user-id/context/TRAVELER/reports/mock-report/submit"}`))
			case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/mock-report"):
				if _, err := os.Stat(stateFile); err == nil {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"reportId":"mock-report","isSubmitted":true,"approvalStatus":"Pending Concur Audit Service"}`))
				} else {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"reportId":"mock-report","isSubmitted":false,"approvalStatus":"Not Submitted"}`))
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
			"reports", "submit", "mock-report",
			"--user-id", "test-user-id",
			"--json",
		})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(dialogOpenFile); os.IsNotExist(err) {
			t.Error("expected the page-level Submit Report button to be clicked (opening the confirmation dialog), but it wasn't")
		}
		if _, err := os.Stat(stateFile); os.IsNotExist(err) {
			t.Error("expected the DIALOG's own Submit Report button to be clicked (not just the page button), but it wasn't")
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
		if isSubmitted, ok := data["isSubmitted"].(bool); !ok || !isSubmitted {
			t.Errorf("expected isSubmitted: true in the response, got %+v", data)
		}
	})

	t.Run("NonMatching405_DoesNotTriggerFallback", func(t *testing.T) {
		_ = os.Remove(stateFile)
		_ = os.Remove(dialogOpenFile)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			_, _ = w.Write([]byte(`{"errorMessage":"Method Not Allowed on an unrelated path"}`))
		}))
		defer server.Close()

		t.Setenv("CONCUR_BASE_URL", server.URL)
		t.Setenv("CONCUR_UI_BASE_URL", "https://us2.concursolutions.com")
		t.Setenv("PRINTING_PRESS_VERIFY", "1")
		t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")

		cmd := RootCmd()
		cmd.SetArgs([]string{
			"reports", "submit", "mock-report",
			"--user-id", "test-user-id",
			"--json",
		})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected an error for a 405 that doesn't match the submit-path signature, got nil")
		}
		if _, statErr := os.Stat(dialogOpenFile); !os.IsNotExist(statErr) {
			t.Error("expected the browser fallback to NEVER trigger for a non-matching 405, but it opened the confirmation dialog")
		}
	})

	t.Run("ConfirmClickNeverRegisters_ReportsFailureNotFalseSuccess", func(t *testing.T) {
		// PATCH(Greptile review, "Submission success is unverified") --
		// reproduces the exact false-success risk that review comment
		// flagged: the mocked dialog-confirm click below deliberately
		// never touches stateFile, so the API's GET keeps returning
		// isSubmitted:false / approvalStatus:"Not Submitted" even after
		// both clicks report no error. The command must fail, not report
		// success against unconfirmed data.
		_ = os.Remove(stateFile)
		_ = os.Remove(dialogOpenFile)

		noopDir := t.TempDir()
		noopScript := `#!/bin/bash
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
	echo "https://us2.concursolutions.com/nui/expense/reports/mock-report"
	exit 0
fi

if [ "$arg1" = "snapshot" ]; then
	# Dialog always appears open (both buttons always present) but
	# clicking the dialog's own button below is a deliberate no-op, so
	# the report is never actually confirmed submitted.
	echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{"e1":{"name":"Submit Report","role":"button"},"e2":{"name":"Submit Report","role":"button"},"e3":{"name":"Cancel","role":"button"}}}}'
	exit 0
fi

exit 0
`
		if err := os.WriteFile(filepath.Join(noopDir, "agent-browser"), []byte(noopScript), 0o755); err != nil {
			t.Fatalf("writing noop mock binary: %v", err)
		}
		t.Setenv("PATH", noopDir+string(filepath.ListSeparator)+oldPath)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/submit"):
				w.WriteHeader(http.StatusMethodNotAllowed)
				_, _ = w.Write([]byte(`{"errorMessage":"Request method 'POST' is not supported","path":"/expensereports/v4/users/test-user-id/context/TRAVELER/reports/mock-report/submit"}`))
			case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/mock-report"):
				// Always unsubmitted -- the confirm click never registered.
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"reportId":"mock-report","isSubmitted":false,"approvalStatus":"Not Submitted"}`))
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
			"reports", "submit", "mock-report",
			"--user-id", "test-user-id",
			"--json",
		})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected an error when the report is never actually confirmed submitted, got nil (false success)")
		}
	})

	t.Run("UnparseableResponse_ReportsFailureNotFalseSuccess", func(t *testing.T) {
		// PATCH(Greptile review, "Submission success is unverified") --
		// the specific failure mode named in that review: a decode
		// failure on the post-click GET must not fall through to success.
		_ = os.Remove(stateFile)
		_ = os.Remove(dialogOpenFile)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/submit"):
				w.WriteHeader(http.StatusMethodNotAllowed)
				_, _ = w.Write([]byte(`{"errorMessage":"Request method 'POST' is not supported","path":"/expensereports/v4/users/test-user-id/context/TRAVELER/reports/mock-report/submit"}`))
			case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/mock-report"):
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`not valid json`))
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
			"reports", "submit", "mock-report",
			"--user-id", "test-user-id",
			"--json",
		})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected an error when the post-submit response cannot be parsed to verify submission, got nil (false success)")
		}
	})
}
