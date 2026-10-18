// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(amend-2026-10-17: add tests for expense update type browser fallback)

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestExpensesUpdate_Type_BrowserFallback(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "mock_state")
	oldPath := os.Getenv("PATH")

	t.Run("SuccessPath", func(t *testing.T) {
		_ = os.Remove(stateFile)
		_ = os.Remove(stateFile + ".combobox_clicked")
		_ = os.Remove(stateFile + ".option_clicked")
		_ = os.Remove(stateFile + ".save_clicked")

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
	if [ -f "$MOCK_STATE_FILE.save_clicked" ]; then
		echo "https://us2.concursolutions.com/nui/expense/reports/mock-report/expenses/parent-1/itemizations"
	else
		echo "https://us2.concursolutions.com/nui/expense/reports/mock-report/expenses/parent-1/itemizations/exp-1"
	fi
	exit 0
fi

if [ "$arg1" = "snapshot" ]; then
	if [ -f "$MOCK_STATE_FILE.combobox_clicked" ]; then
		# options appear in plain snapshot or interactive snapshot. Let's return them for both.
		echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{"e1":{"name":"Expense Type","role":"combobox"},"e2":{"name":"Hotel","role":"option"},"e3":{"name":"Airfare","role":"option"},"e4":{"name":"Save Itemization","role":"button"}}}}'
	else
		# combobox only
		echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{"e1":{"name":"Expense Type","role":"combobox"},"e4":{"name":"Save Itemization","role":"button"}}}}'
	fi
	exit 0
fi

if [ "$arg1" = "click" ]; then
	if [ "$arg2" = "@e1" ]; then
		touch "$MOCK_STATE_FILE.combobox_clicked"
	elif [ "$arg2" = "@e2" ]; then
		touch "$MOCK_STATE_FILE.option_clicked"
	elif [ "$arg2" = "@e4" ]; then
		touch "$MOCK_STATE_FILE.save_clicked"
	fi
	exit 0
fi

exit 0
`
		subTmpDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(subTmpDir, "agent-browser"), []byte(mockScript), 0o755); err != nil {
			t.Fatalf("writing mock binary: %v", err)
		}

		t.Setenv("PATH", subTmpDir+string(filepath.ListSeparator)+oldPath)
		t.Setenv("MOCK_STATE_FILE", stateFile)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/expenses/exp-1"):
				if _, err := os.Stat(stateFile + ".save_clicked"); err == nil {
					// After the save: returns the updated type
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"expenseId":"exp-1","parentExpenseId":"parent-1","expenseType":{"id":"HTL","name":"Hotel"},"transactionAmount":{"value":50.0}}`))
				} else {
					// Before: returns the old type
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"expenseId":"exp-1","parentExpenseId":"parent-1","expenseType":{"id":"TXI","name":"Taxi"},"transactionAmount":{"value":50.0}}`))
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
			"--type", "Hotel",
			"--json",
		})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(stateFile + ".save_clicked"); os.IsNotExist(err) {
			t.Error("expected Save Itemization to be clicked, but state file does not exist")
		}

		var envelope map[string]any
		if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
			t.Fatalf("failed to unmarshal output JSON: %v", err)
		}
		if success, ok := envelope["success"].(bool); !ok || !success {
			t.Errorf("expected envelope success: true, got %+v", envelope)
		}
	})

	t.Run("TypeNotOffered", func(t *testing.T) {
		_ = os.Remove(stateFile)
		_ = os.Remove(stateFile + ".combobox_clicked")
		_ = os.Remove(stateFile + ".option_clicked")
		_ = os.Remove(stateFile + ".save_clicked")

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

if [ "$arg1" = "snapshot" ]; then
	if [ -f "$MOCK_STATE_FILE.combobox_clicked" ]; then
		# Hotel and Airfare offered, Train requested is not offered
		echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{"e1":{"name":"Expense Type","role":"combobox"},"e2":{"name":"Hotel","role":"option"},"e3":{"name":"Airfare","role":"option"},"e4":{"name":"Save Itemization","role":"button"}}}}'
	else
		echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{"e1":{"name":"Expense Type","role":"combobox"},"e4":{"name":"Save Itemization","role":"button"}}}}'
	fi
	exit 0
fi

if [ "$arg1" = "click" ]; then
	if [ "$arg2" = "@e1" ]; then
		touch "$MOCK_STATE_FILE.combobox_clicked"
	fi
	exit 0
fi

exit 0
`
		subTmpDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(subTmpDir, "agent-browser"), []byte(mockScript), 0o755); err != nil {
			t.Fatalf("writing mock binary: %v", err)
		}

		t.Setenv("PATH", subTmpDir+string(filepath.ListSeparator)+oldPath)
		t.Setenv("MOCK_STATE_FILE", stateFile)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"expenseId":"exp-1","parentExpenseId":"parent-1","expenseType":{"id":"TXI","name":"Taxi"},"transactionAmount":{"value":50.0}}`))
		}))
		defer server.Close()

		t.Setenv("CONCUR_BASE_URL", server.URL)
		t.Setenv("CONCUR_UI_BASE_URL", "https://us2.concursolutions.com")

		cmd := RootCmd()
		cmd.SetArgs([]string{
			"expenses", "update", "exp-1",
			"--user-id", "test-user-id",
			"--report-id", "mock-report",
			"--type", "Train",
		})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected an error when the type is not offered, got nil")
		}
		if !strings.Contains(err.Error(), "expense type \"Train\" is not offered for this line under your Concur policy") {
			t.Errorf("expected error message to say Train is not offered, got: %v", err)
		}
		if !strings.Contains(err.Error(), "Airfare, Hotel") {
			t.Errorf("expected error message to contain list of offered types, got: %v", err)
		}
		if _, statErr := os.Stat(stateFile + ".save_clicked"); !os.IsNotExist(statErr) {
			t.Error("expected Save Itemization to NEVER be clicked when type is not offered, but it was")
		}
	})

	t.Run("TopLevelExpenseRejected", func(t *testing.T) {
		// Mock script that fails immediately to assert zero invocations
		mockScript := `#!/bin/bash
echo "Should not be invoked"
exit 1
`
		subTmpDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(subTmpDir, "agent-browser"), []byte(mockScript), 0o755); err != nil {
			t.Fatalf("writing mock binary: %v", err)
		}

		t.Setenv("PATH", subTmpDir+string(filepath.ListSeparator)+oldPath)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// parentExpenseId is null -> top level expense
			_, _ = w.Write([]byte(`{"expenseId":"exp-1","parentExpenseId":null,"expenseType":{"id":"TXI","name":"Taxi"},"transactionAmount":{"value":50.0}}`))
		}))
		defer server.Close()

		t.Setenv("CONCUR_BASE_URL", server.URL)
		t.Setenv("CONCUR_UI_BASE_URL", "https://us2.concursolutions.com")

		cmd := RootCmd()
		cmd.SetArgs([]string{
			"expenses", "update", "exp-1",
			"--user-id", "test-user-id",
			"--report-id", "mock-report",
			"--type", "Hotel",
		})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error on top-level expense update request, got nil")
		}
		if !strings.Contains(err.Error(), "only supported for itemization lines") {
			t.Errorf("expected error to mention itemization support only, got: %v", err)
		}
	})

	t.Run("SameTypeNoOp", func(t *testing.T) {
		// Mock script that fails immediately to assert zero invocations
		mockScript := `#!/bin/bash
echo "Should not be invoked"
exit 1
`
		subTmpDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(subTmpDir, "agent-browser"), []byte(mockScript), 0o755); err != nil {
			t.Fatalf("writing mock binary: %v", err)
		}

		t.Setenv("PATH", subTmpDir+string(filepath.ListSeparator)+oldPath)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"expenseId":"exp-1","parentExpenseId":"parent-1","expenseType":{"id":"HTL","name":"Hotel"},"transactionAmount":{"value":50.0}}`))
		}))
		defer server.Close()

		t.Setenv("CONCUR_BASE_URL", server.URL)
		t.Setenv("CONCUR_UI_BASE_URL", "https://us2.concursolutions.com")

		cmd := RootCmd()
		cmd.SetArgs([]string{
			"expenses", "update", "exp-1",
			"--user-id", "test-user-id",
			"--report-id", "mock-report",
			"--type", "Hotel",
		})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error on same type update: %v", err)
		}
	})

	t.Run("CombinedArgsRejected", func(t *testing.T) {
		// Mock script that fails immediately to assert zero invocations
		mockScript := `#!/bin/bash
echo "Should not be invoked"
exit 1
`
		subTmpDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(subTmpDir, "agent-browser"), []byte(mockScript), 0o755); err != nil {
			t.Fatalf("writing mock binary: %v", err)
		}
		t.Setenv("PATH", subTmpDir+string(filepath.ListSeparator)+oldPath)

		// Every other update field, including the --stdin body, must be refused
		// alongside --type rather than silently ignored.
		for _, other := range [][]string{
			{"--purpose", "Business"},
			{"--amount", "10"},
			{"--comment", "note"},
			{"--custom-fields", "[]"},
			{"--stdin"},
		} {
			cmd := RootCmd()
			cmd.SetArgs(append([]string{
				"expenses", "update", "exp-1",
				"--user-id", "test-user-id",
				"--report-id", "mock-report",
				"--type", "Hotel",
			}, other...))

			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(io.Discard)

			err := cmd.Execute()
			if err == nil {
				t.Fatalf("%v: expected combined flags check to fail, got nil", other)
			}
			if !strings.Contains(err.Error(), "cannot combine --type with other update fields") {
				t.Errorf("%v: expected combined error message, got: %v", other, err)
			}
		}
	})

	t.Run("NotLoggedIn", func(t *testing.T) {
		_ = os.Remove(stateFile)

		mockScript := `#!/bin/bash
arg1="$1"
arg2="$2"

if [ "$arg1" = "--session" ]; then
	arg1="$3"
	arg2="$4"
fi

if [ "$arg1" = "snapshot" ]; then
	# Returns a snapshot with Sign in button
	echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{"e1":{"name":"Sign in","role":"button"}}}}'
	exit 0
fi

exit 0
`
		subTmpDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(subTmpDir, "agent-browser"), []byte(mockScript), 0o755); err != nil {
			t.Fatalf("writing mock binary: %v", err)
		}

		t.Setenv("PATH", subTmpDir+string(filepath.ListSeparator)+oldPath)
		t.Setenv("MOCK_STATE_FILE", stateFile)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"expenseId":"exp-1","parentExpenseId":"parent-1","expenseType":{"id":"TXI","name":"Taxi"},"transactionAmount":{"value":50.0}}`))
		}))
		defer server.Close()

		t.Setenv("CONCUR_BASE_URL", server.URL)
		t.Setenv("CONCUR_UI_BASE_URL", "https://us2.concursolutions.com")

		cmd := RootCmd()
		cmd.SetArgs([]string{
			"expenses", "update", "exp-1",
			"--user-id", "test-user-id",
			"--report-id", "mock-report",
			"--type", "Hotel",
		})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error when not logged in, got nil")
		}
		if !strings.Contains(err.Error(), "not logged in to Concur") {
			t.Errorf("expected not logged in error, got: %v", err)
		}
	})

	t.Run("PostSaveVerificationMismatch", func(t *testing.T) {
		_ = os.Remove(stateFile)
		_ = os.Remove(stateFile + ".combobox_clicked")
		_ = os.Remove(stateFile + ".option_clicked")
		_ = os.Remove(stateFile + ".save_clicked")

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
	if [ -f "$MOCK_STATE_FILE.save_clicked" ]; then
		echo "https://us2.concursolutions.com/nui/expense/reports/mock-report/expenses/parent-1/itemizations"
	else
		echo "https://us2.concursolutions.com/nui/expense/reports/mock-report/expenses/parent-1/itemizations/exp-1"
	fi
	exit 0
fi

if [ "$arg1" = "snapshot" ]; then
	if [ -f "$MOCK_STATE_FILE.combobox_clicked" ]; then
		echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{"e1":{"name":"Expense Type","role":"combobox"},"e2":{"name":"Hotel","role":"option"},"e3":{"name":"Airfare","role":"option"},"e4":{"name":"Save Itemization","role":"button"}}}}'
	else
		echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{"e1":{"name":"Expense Type","role":"combobox"},"e4":{"name":"Save Itemization","role":"button"}}}}'
	fi
	exit 0
fi

if [ "$arg1" = "click" ]; then
	if [ "$arg2" = "@e1" ]; then
		touch "$MOCK_STATE_FILE.combobox_clicked"
	elif [ "$arg2" = "@e2" ]; then
		touch "$MOCK_STATE_FILE.option_clicked"
	elif [ "$arg2" = "@e4" ]; then
		touch "$MOCK_STATE_FILE.save_clicked"
	fi
	exit 0
fi

exit 0
`
		subTmpDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(subTmpDir, "agent-browser"), []byte(mockScript), 0o755); err != nil {
			t.Fatalf("writing mock binary: %v", err)
		}

		t.Setenv("PATH", subTmpDir+string(filepath.ListSeparator)+oldPath)
		t.Setenv("MOCK_STATE_FILE", stateFile)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// GET always returns original Taxi type (mismatch)
			_, _ = w.Write([]byte(`{"expenseId":"exp-1","parentExpenseId":"parent-1","expenseType":{"id":"TXI","name":"Taxi"},"transactionAmount":{"value":50.0}}`))
		}))
		defer server.Close()

		t.Setenv("CONCUR_BASE_URL", server.URL)
		t.Setenv("CONCUR_UI_BASE_URL", "https://us2.concursolutions.com")

		cmd := RootCmd()
		cmd.SetArgs([]string{
			"expenses", "update", "exp-1",
			"--user-id", "test-user-id",
			"--report-id", "mock-report",
			"--type", "Hotel",
		})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error on verification mismatch, got nil")
		}
		if !strings.Contains(err.Error(), "may already carry the new values") {
			t.Errorf("expected partial-success error, got: %v", err)
		}
	})

	t.Run("DryRunLaunchesNothing", func(t *testing.T) {
		// Mock script that fails immediately to assert zero invocations
		mockScript := `#!/bin/bash
echo "Should not be invoked"
exit 1
`
		subTmpDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(subTmpDir, "agent-browser"), []byte(mockScript), 0o755); err != nil {
			t.Fatalf("writing mock binary: %v", err)
		}

		t.Setenv("PATH", subTmpDir+string(filepath.ListSeparator)+oldPath)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"expenseId":"exp-1","parentExpenseId":"parent-1","expenseType":{"id":"TXI","name":"Taxi"},"transactionAmount":{"value":50.0}}`))
		}))
		defer server.Close()

		t.Setenv("CONCUR_BASE_URL", server.URL)
		t.Setenv("CONCUR_UI_BASE_URL", "https://us2.concursolutions.com")

		cmd := RootCmd()
		cmd.SetArgs([]string{
			"expenses", "update", "exp-1",
			"--user-id", "test-user-id",
			"--report-id", "mock-report",
			"--type", "Hotel",
			"--dry-run",
		})

		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error on dry-run: %v", err)
		}

		output := out.String()
		if !strings.Contains(output, "Dry-run: would open") {
			t.Errorf("expected dry-run output message, got: %q", output)
		}
		if !strings.Contains(output, "itemizations/exp-1") {
			t.Errorf("expected deep link in dry-run output, got: %q", output)
		}
	})
}

// PATCH(Greptile review, "Saved changes can look unsuccessful") -- the Save
// Itemization click helper can fail after the click already landed, so its
// error must be settled against a read-back of the line rather than reported
// as an ordinary failure.
func TestExpensesUpdate_Type_SaveClickErrorIsReconciled(t *testing.T) {
	const clickError = "Execution context was destroyed"
	taxi := `{"expenseId":"exp-1","parentExpenseId":"parent-1","expenseType":{"id":"TXI","name":"Taxi"},"transactionAmount":{"value":50.0}}`
	hotel := `{"expenseId":"exp-1","parentExpenseId":"parent-1","expenseType":{"id":"HTL","name":"Hotel"},"transactionAmount":{"value":50.0}}`

	t.Run("click errors after the save landed", func(t *testing.T) {
		stderr, err := runTypeUpdateWithFailingSave(t, true, func(stateFile string) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				body := taxi
				if _, statErr := os.Stat(stateFile + ".save_clicked"); statErr == nil {
					body = hotel
				}
				_, _ = w.Write([]byte(body))
			}
		})
		if err != nil {
			t.Fatalf("the line shows the new type, so the command must succeed: %v", err)
		}
		for _, want := range []string{clickError, "the save went through", "Expense type updated successfully"} {
			if !strings.Contains(stderr, want) {
				t.Errorf("stderr missing %q: %s", want, stderr)
			}
		}
	})

	t.Run("click errors and the line is unchanged", func(t *testing.T) {
		_, err := runTypeUpdateWithFailingSave(t, false, func(string) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(taxi))
			}
		})
		if err == nil {
			t.Fatal("expected the click failure to be reported")
		}
		if !strings.Contains(err.Error(), clickError) {
			t.Errorf("the original click error should be returned, got: %v", err)
		}
		if strings.Contains(err.Error(), "may already carry the new values") {
			t.Errorf("a confirmed no-op must not be reported as a possible partial success: %v", err)
		}
	})

	t.Run("click errors and the line cannot be read back", func(t *testing.T) {
		var reads atomic.Int32
		_, err := runTypeUpdateWithFailingSave(t, true, func(string) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if reads.Add(1) == 1 {
					_, _ = w.Write([]byte(taxi))
					return
				}
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"message":"unavailable"}`))
			}
		})
		if err == nil {
			t.Fatal("expected an error when neither the click nor the read-back can confirm the save")
		}
		for _, want := range []string{"may already carry the new values", "fetching expense after browser fallback save", clickError} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error missing %q: %v", want, err)
			}
		}
	})
}

// runTypeUpdateWithFailingSave runs `expenses update --type Hotel` against a
// fake browser whose Save Itemization click exits non-zero, as when the click
// navigates the page and the helper loses its execution context. landed
// records the click as having registered, which the fake Concur can key its
// read-back on. getExpense serves the GET of the line; it receives the state
// file prefix the fake browser writes to.
func runTypeUpdateWithFailingSave(t *testing.T, landed bool, getExpense func(stateFile string) http.HandlerFunc) (stderr string, err error) {
	t.Helper()
	stateFile := filepath.Join(t.TempDir(), "mock_state")

	record := ""
	if landed {
		record = `touch "$MOCK_STATE_FILE.save_clicked"`
	}
	mockScript := fmt.Sprintf(`#!/bin/bash
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
	echo "https://us2.concursolutions.com/nui/expense/reports/mock-report/expenses/parent-1/itemizations/exp-1"
	exit 0
fi

if [ "$arg1" = "snapshot" ]; then
	if [ -f "$MOCK_STATE_FILE.combobox_clicked" ]; then
		echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{"e1":{"name":"Expense Type","role":"combobox"},"e2":{"name":"Hotel","role":"option"},"e3":{"name":"Airfare","role":"option"},"e4":{"name":"Save Itemization","role":"button"}}}}'
	else
		echo '{"success":true,"data":{"origin":"https://us2.concursolutions.com","refs":{"e1":{"name":"Expense Type","role":"combobox"},"e4":{"name":"Save Itemization","role":"button"}}}}'
	fi
	exit 0
fi

if [ "$arg1" = "click" ]; then
	if [ "$arg2" = "@e1" ]; then
		touch "$MOCK_STATE_FILE.combobox_clicked"
	elif [ "$arg2" = "@e4" ]; then
		%s
		echo "Execution context was destroyed" >&2
		exit 1
	fi
	exit 0
fi

exit 0
`, record)
	binDir := t.TempDir()
	if werr := os.WriteFile(filepath.Join(binDir, "agent-browser"), []byte(mockScript), 0o755); werr != nil {
		t.Fatalf("writing mock binary: %v", werr)
	}
	t.Setenv("PATH", binDir+string(filepath.ListSeparator)+os.Getenv("PATH"))
	t.Setenv("MOCK_STATE_FILE", stateFile)

	server := httptest.NewServer(getExpense(stateFile))
	t.Cleanup(server.Close)
	t.Setenv("CONCUR_BASE_URL", server.URL)
	t.Setenv("CONCUR_UI_BASE_URL", "https://us2.concursolutions.com")

	cmd := RootCmd()
	cmd.SetArgs([]string{
		"expenses", "update", "exp-1",
		"--user-id", "test-user-id",
		"--report-id", "mock-report",
		"--type", "Hotel",
		"--json",
	})
	var stderrBuf bytes.Buffer
	cmd.SetOut(io.Discard)
	cmd.SetErr(&stderrBuf)
	err = cmd.Execute()
	return stderrBuf.String(), err
}
